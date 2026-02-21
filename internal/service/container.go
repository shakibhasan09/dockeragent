package service

import (
	"context"
	"fmt"
	"io"
	"net/netip"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

type ContainerService struct {
	docker *client.Client
}

func NewContainerService(docker *client.Client) *ContainerService {
	return &ContainerService{docker: docker}
}

func (s *ContainerService) Create(ctx context.Context, req model.CreateContainerRequest) (model.CreateContainerResponse, error) {
	cfg := &container.Config{
		Image:      req.Image,
		Cmd:        req.Cmd,
		Entrypoint: req.Entrypoint,
		Env:        req.Env,
		Labels:     req.Labels,
		WorkingDir: req.WorkingDir,
		User:       req.User,
		Hostname:   req.Hostname,
	}

	exposedPorts, portBindings, err := buildPortMappings(req.Ports)
	if err != nil {
		return model.CreateContainerResponse{}, fmt.Errorf("invalid port mapping: %w", err)
	}
	cfg.ExposedPorts = exposedPorts

	hostCfg := &container.HostConfig{
		PortBindings: portBindings,
	}

	if req.NetworkMode != "" {
		hostCfg.NetworkMode = container.NetworkMode(req.NetworkMode)
	}

	if len(req.Volumes) > 0 {
		hostCfg.Mounts = buildMounts(req.Volumes)
	}

	if req.RestartPolicy != nil {
		hostCfg.RestartPolicy = container.RestartPolicy{
			Name:              container.RestartPolicyMode(req.RestartPolicy.Name),
			MaximumRetryCount: req.RestartPolicy.MaxRetryCount,
		}
	}

	if req.Resources != nil {
		if req.Resources.CPUs > 0 {
			hostCfg.Resources.NanoCPUs = int64(req.Resources.CPUs * 1e9)
		}
		if req.Resources.MemoryMB > 0 {
			hostCfg.Resources.Memory = req.Resources.MemoryMB * 1024 * 1024
		}
		if req.Resources.CPUShares > 0 {
			hostCfg.Resources.CPUShares = req.Resources.CPUShares
		}
	}

	var networkingCfg *network.NetworkingConfig
	if len(req.Networks) > 0 {
		networkingCfg = &network.NetworkingConfig{
			EndpointsConfig: make(map[string]*network.EndpointSettings),
		}
		for _, netName := range req.Networks {
			networkingCfg.EndpointsConfig[netName] = &network.EndpointSettings{}
		}
	}

	result, err := s.docker.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           cfg,
		HostConfig:       hostCfg,
		NetworkingConfig: networkingCfg,
		Name:             req.Name,
	})
	if err != nil {
		return model.CreateContainerResponse{}, fmt.Errorf("create container: %w", err)
	}

	_, err = s.docker.ContainerStart(ctx, result.ID, client.ContainerStartOptions{})
	if err != nil {
		return model.CreateContainerResponse{}, fmt.Errorf("start container: %w", err)
	}

	return model.CreateContainerResponse{
		ID:       result.ID,
		Warnings: result.Warnings,
	}, nil
}

func (s *ContainerService) List(ctx context.Context, all bool) (model.ContainerListResponse, error) {
	result, err := s.docker.ContainerList(ctx, client.ContainerListOptions{All: all})
	if err != nil {
		return model.ContainerListResponse{}, fmt.Errorf("list containers: %w", err)
	}

	containers := make([]model.ContainerSummary, 0, len(result.Items))
	for _, c := range result.Items {
		ports := make([]model.PortSummary, 0, len(c.Ports))
		for _, p := range c.Ports {
			ps := model.PortSummary{
				PrivatePort: p.PrivatePort,
				PublicPort:  p.PublicPort,
				Type:        p.Type,
			}
			if p.IP.IsValid() {
				ps.IP = p.IP.String()
			}
			ports = append(ports, ps)
		}
		containers = append(containers, model.ContainerSummary{
			ID:      c.ID,
			Names:   c.Names,
			Image:   c.Image,
			ImageID: c.ImageID,
			Command: c.Command,
			Created: c.Created,
			State:   string(c.State),
			Status:  c.Status,
			Ports:   ports,
			Labels:  c.Labels,
		})
	}
	return model.ContainerListResponse{Containers: containers}, nil
}

func (s *ContainerService) Inspect(ctx context.Context, id string) (container.InspectResponse, error) {
	result, err := s.docker.ContainerInspect(ctx, id, client.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, fmt.Errorf("inspect container: %w", err)
	}
	return result.Container, nil
}

func (s *ContainerService) Stop(ctx context.Context, id string, req model.StopContainerRequest) error {
	_, err := s.docker.ContainerStop(ctx, id, client.ContainerStopOptions{
		Timeout: req.Timeout,
		Signal:  req.Signal,
	})
	if err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	return nil
}

func (s *ContainerService) Remove(ctx context.Context, id string, q model.RemoveContainerQuery) error {
	_, err := s.docker.ContainerRemove(ctx, id, client.ContainerRemoveOptions{
		Force:         q.Force,
		RemoveVolumes: q.RemoveVolumes,
	})
	if err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}

func (s *ContainerService) Logs(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error) {
	tail := q.Tail
	if tail == "" {
		tail = "100"
	}
	result, err := s.docker.ContainerLogs(ctx, id, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     q.Follow,
		Tail:       tail,
		Since:      q.Since,
		Until:      q.Until,
		Timestamps: q.Timestamps,
	})
	if err != nil {
		return nil, fmt.Errorf("container logs: %w", err)
	}
	return result, nil
}

func (s *ContainerService) Ping(ctx context.Context) error {
	_, err := s.docker.Ping(ctx, client.PingOptions{})
	return err
}

func buildPortMappings(ports []model.PortMapping) (network.PortSet, network.PortMap, error) {
	if len(ports) == 0 {
		return nil, nil, nil
	}
	exposed := make(network.PortSet)
	bindings := make(network.PortMap)

	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		portStr := p.ContainerPort + "/" + proto
		containerPort, err := network.ParsePort(portStr)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid container port %q: %w", portStr, err)
		}

		exposed[containerPort] = struct{}{}

		binding := network.PortBinding{
			HostPort: p.HostPort,
		}
		if p.HostIP != "" {
			addr, err := netip.ParseAddr(p.HostIP)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid host_ip %q: %w", p.HostIP, err)
			}
			binding.HostIP = addr
		}
		bindings[containerPort] = append(bindings[containerPort], binding)
	}
	return exposed, bindings, nil
}

func buildMounts(volumes []model.VolumeMount) []mount.Mount {
	mounts := make([]mount.Mount, 0, len(volumes))
	for _, v := range volumes {
		mountType := mount.TypeBind
		switch v.Type {
		case "volume":
			mountType = mount.TypeVolume
		case "tmpfs":
			mountType = mount.TypeTmpfs
		}
		mounts = append(mounts, mount.Mount{
			Type:     mountType,
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}
	return mounts
}
