package service

import (
	"context"
	"errors"
	"io"
	"net/netip"
	"strings"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// --- mock Docker client ---

type mockDockerClient struct {
	containerCreateFn  func(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error)
	containerStartFn   func(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error)
	containerListFn    func(ctx context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error)
	containerInspectFn func(ctx context.Context, id string, opts client.ContainerInspectOptions) (client.ContainerInspectResult, error)
	containerStopFn    func(ctx context.Context, id string, opts client.ContainerStopOptions) (client.ContainerStopResult, error)
	containerRemoveFn  func(ctx context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error)
	containerLogsFn    func(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error)
	pingFn             func(ctx context.Context, opts client.PingOptions) (client.PingResult, error)
}

func (m *mockDockerClient) ContainerCreate(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
	return m.containerCreateFn(ctx, opts)
}
func (m *mockDockerClient) ContainerStart(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error) {
	return m.containerStartFn(ctx, id, opts)
}
func (m *mockDockerClient) ContainerList(ctx context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error) {
	return m.containerListFn(ctx, opts)
}
func (m *mockDockerClient) ContainerInspect(ctx context.Context, id string, opts client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
	return m.containerInspectFn(ctx, id, opts)
}
func (m *mockDockerClient) ContainerStop(ctx context.Context, id string, opts client.ContainerStopOptions) (client.ContainerStopResult, error) {
	return m.containerStopFn(ctx, id, opts)
}
func (m *mockDockerClient) ContainerRemove(ctx context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
	return m.containerRemoveFn(ctx, id, opts)
}
func (m *mockDockerClient) ContainerLogs(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
	return m.containerLogsFn(ctx, id, opts)
}
func (m *mockDockerClient) Ping(ctx context.Context, opts client.PingOptions) (client.PingResult, error) {
	return m.pingFn(ctx, opts)
}

// --- buildPortMappings tests ---

func TestBuildPortMappings_Nil(t *testing.T) {
	exposed, bindings, err := buildPortMappings(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exposed != nil || bindings != nil {
		t.Fatalf("expected nil, got exposed=%v bindings=%v", exposed, bindings)
	}
}

func TestBuildPortMappings_Empty(t *testing.T) {
	exposed, bindings, err := buildPortMappings([]model.PortMapping{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exposed != nil || bindings != nil {
		t.Fatalf("expected nil, got exposed=%v bindings=%v", exposed, bindings)
	}
}

func TestBuildPortMappings_TCPDefault(t *testing.T) {
	ports := []model.PortMapping{
		{ContainerPort: "80", HostPort: "8080"},
	}
	exposed, bindings, err := buildPortMappings(ports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPort, _ := network.ParsePort("80/tcp")
	if _, ok := exposed[expectedPort]; !ok {
		t.Fatalf("expected exposed port 80/tcp")
	}
	b, ok := bindings[expectedPort]
	if !ok || len(b) != 1 {
		t.Fatalf("expected one binding for 80/tcp, got %v", b)
	}
	if b[0].HostPort != "8080" {
		t.Errorf("expected HostPort=8080, got %s", b[0].HostPort)
	}
}

func TestBuildPortMappings_UDP(t *testing.T) {
	ports := []model.PortMapping{
		{ContainerPort: "53", Protocol: "udp"},
	}
	exposed, _, err := buildPortMappings(ports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPort, _ := network.ParsePort("53/udp")
	if _, ok := exposed[expectedPort]; !ok {
		t.Fatal("expected exposed port 53/udp")
	}
}

func TestBuildPortMappings_WithHostIP(t *testing.T) {
	ports := []model.PortMapping{
		{ContainerPort: "80", HostPort: "8080", HostIP: "127.0.0.1"},
	}
	_, bindings, err := buildPortMappings(ports)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPort, _ := network.ParsePort("80/tcp")
	b := bindings[expectedPort]
	if len(b) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(b))
	}
	expectedIP := netip.MustParseAddr("127.0.0.1")
	if b[0].HostIP != expectedIP {
		t.Errorf("expected HostIP=%v, got %v", expectedIP, b[0].HostIP)
	}
}

func TestBuildPortMappings_InvalidHostIP(t *testing.T) {
	ports := []model.PortMapping{
		{ContainerPort: "80", HostIP: "not-an-ip"},
	}
	_, _, err := buildPortMappings(ports)
	if err == nil {
		t.Fatal("expected error for invalid host IP")
	}
	if !strings.Contains(err.Error(), "invalid host_ip") {
		t.Errorf("expected 'invalid host_ip' in error, got: %v", err)
	}
}

func TestBuildPortMappings_InvalidPort(t *testing.T) {
	ports := []model.PortMapping{
		{ContainerPort: "abc"},
	}
	_, _, err := buildPortMappings(ports)
	if err == nil {
		t.Fatal("expected error for invalid container port")
	}
	if !strings.Contains(err.Error(), "invalid container port") {
		t.Errorf("expected 'invalid container port' in error, got: %v", err)
	}
}

// --- buildMounts tests ---

func TestBuildMounts_Bind(t *testing.T) {
	vols := []model.VolumeMount{
		{Source: "/host/path", Target: "/container/path", Type: ""},
	}
	mounts := buildMounts(vols)
	if len(mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(mounts))
	}
	if mounts[0].Type != mount.TypeBind {
		t.Errorf("expected TypeBind, got %v", mounts[0].Type)
	}
	if mounts[0].Source != "/host/path" || mounts[0].Target != "/container/path" {
		t.Errorf("unexpected source/target: %s -> %s", mounts[0].Source, mounts[0].Target)
	}
}

func TestBuildMounts_Volume(t *testing.T) {
	vols := []model.VolumeMount{
		{Source: "myvolume", Target: "/data", Type: "volume"},
	}
	mounts := buildMounts(vols)
	if mounts[0].Type != mount.TypeVolume {
		t.Errorf("expected TypeVolume, got %v", mounts[0].Type)
	}
}

func TestBuildMounts_Tmpfs(t *testing.T) {
	vols := []model.VolumeMount{
		{Source: "", Target: "/tmp", Type: "tmpfs"},
	}
	mounts := buildMounts(vols)
	if mounts[0].Type != mount.TypeTmpfs {
		t.Errorf("expected TypeTmpfs, got %v", mounts[0].Type)
	}
}

func TestBuildMounts_ReadOnly(t *testing.T) {
	vols := []model.VolumeMount{
		{Source: "/host", Target: "/container", ReadOnly: true},
	}
	mounts := buildMounts(vols)
	if !mounts[0].ReadOnly {
		t.Error("expected ReadOnly=true")
	}
}

func TestBuildMounts_Multiple(t *testing.T) {
	vols := []model.VolumeMount{
		{Source: "/a", Target: "/b"},
		{Source: "vol", Target: "/c", Type: "volume"},
	}
	mounts := buildMounts(vols)
	if len(mounts) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(mounts))
	}
}

// --- Create tests ---

func TestCreate_Success_Minimal(t *testing.T) {
	mock := &mockDockerClient{
		containerCreateFn: func(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
			if opts.Config.Image != "nginx:latest" {
				t.Errorf("expected image nginx:latest, got %s", opts.Config.Image)
			}
			return client.ContainerCreateResult{ID: "abc123", Warnings: nil}, nil
		},
		containerStartFn: func(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error) {
			if id != "abc123" {
				t.Errorf("expected id abc123, got %s", id)
			}
			return client.ContainerStartResult{}, nil
		},
	}
	svc := NewContainerService(mock)
	resp, err := svc.Create(context.Background(), model.CreateContainerRequest{Image: "nginx:latest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "abc123" {
		t.Errorf("expected ID abc123, got %s", resp.ID)
	}
}

func TestCreate_Success_Full(t *testing.T) {
	cpus := 1.5
	memMB := int64(512)
	mock := &mockDockerClient{
		containerCreateFn: func(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
			cfg := opts.Config
			if cfg.Image != "myapp:v1" {
				t.Errorf("Image = %s", cfg.Image)
			}
			if len(cfg.Cmd) != 1 || cfg.Cmd[0] != "serve" {
				t.Errorf("Cmd = %v", cfg.Cmd)
			}
			if cfg.WorkingDir != "/app" {
				t.Errorf("WorkingDir = %s", cfg.WorkingDir)
			}
			if cfg.Hostname != "myhost" {
				t.Errorf("Hostname = %s", cfg.Hostname)
			}

			hc := opts.HostConfig
			if hc.NetworkMode != "bridge" {
				t.Errorf("NetworkMode = %s", hc.NetworkMode)
			}
			if hc.RestartPolicy.Name != "always" {
				t.Errorf("RestartPolicy.Name = %s", hc.RestartPolicy.Name)
			}
			expectedNano := int64(cpus * 1e9)
			if hc.Resources.NanoCPUs != expectedNano {
				t.Errorf("NanoCPUs = %d, want %d", hc.Resources.NanoCPUs, expectedNano)
			}
			expectedMem := memMB * 1024 * 1024
			if hc.Resources.Memory != expectedMem {
				t.Errorf("Memory = %d, want %d", hc.Resources.Memory, expectedMem)
			}

			if opts.NetworkingConfig == nil {
				t.Error("expected NetworkingConfig")
			} else if _, ok := opts.NetworkingConfig.EndpointsConfig["mynet"]; !ok {
				t.Error("expected mynet in EndpointsConfig")
			}
			if opts.Name != "mycontainer" {
				t.Errorf("Name = %s", opts.Name)
			}

			return client.ContainerCreateResult{ID: "full123", Warnings: []string{"warn1"}}, nil
		},
		containerStartFn: func(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error) {
			return client.ContainerStartResult{}, nil
		},
	}

	svc := NewContainerService(mock)
	resp, err := svc.Create(context.Background(), model.CreateContainerRequest{
		Image:       "myapp:v1",
		Name:        "mycontainer",
		Cmd:         []string{"serve"},
		WorkingDir:  "/app",
		Hostname:    "myhost",
		NetworkMode: "bridge",
		Networks:    []string{"mynet"},
		RestartPolicy: &model.RestartPolicy{
			Name: "always",
		},
		Resources: &model.ResourceLimits{
			CPUs:     cpus,
			MemoryMB: memMB,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "full123" {
		t.Errorf("ID = %s", resp.ID)
	}
	if len(resp.Warnings) != 1 || resp.Warnings[0] != "warn1" {
		t.Errorf("Warnings = %v", resp.Warnings)
	}
}

func TestCreate_DockerCreateError(t *testing.T) {
	mock := &mockDockerClient{
		containerCreateFn: func(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
			return client.ContainerCreateResult{}, errors.New("image not found")
		},
	}
	svc := NewContainerService(mock)
	_, err := svc.Create(context.Background(), model.CreateContainerRequest{Image: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create container") {
		t.Errorf("expected 'create container' in error, got: %v", err)
	}
}

func TestCreate_DockerStartError(t *testing.T) {
	mock := &mockDockerClient{
		containerCreateFn: func(ctx context.Context, opts client.ContainerCreateOptions) (client.ContainerCreateResult, error) {
			return client.ContainerCreateResult{ID: "abc"}, nil
		},
		containerStartFn: func(ctx context.Context, id string, opts client.ContainerStartOptions) (client.ContainerStartResult, error) {
			return client.ContainerStartResult{}, errors.New("permission denied")
		},
	}
	svc := NewContainerService(mock)
	_, err := svc.Create(context.Background(), model.CreateContainerRequest{Image: "nginx"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "start container") {
		t.Errorf("expected 'start container' in error, got: %v", err)
	}
}

func TestCreate_InvalidPortMapping(t *testing.T) {
	mock := &mockDockerClient{}
	svc := NewContainerService(mock)
	_, err := svc.Create(context.Background(), model.CreateContainerRequest{
		Image: "nginx",
		Ports: []model.PortMapping{{ContainerPort: "abc"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid port mapping") {
		t.Errorf("expected 'invalid port mapping' in error, got: %v", err)
	}
}

// --- List tests ---

func TestList_Success_Empty(t *testing.T) {
	mock := &mockDockerClient{
		containerListFn: func(ctx context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error) {
			return client.ContainerListResult{Items: nil}, nil
		},
	}
	svc := NewContainerService(mock)
	resp, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(resp.Containers))
	}
}

func TestList_Success_WithContainers(t *testing.T) {
	mock := &mockDockerClient{
		containerListFn: func(ctx context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error) {
			return client.ContainerListResult{
				Items: []container.Summary{
					{
						ID:      "c1",
						Names:   []string{"/test"},
						Image:   "nginx",
						ImageID: "sha256:abc",
						Command: "nginx -g",
						Created: 1000,
						State:   container.StateRunning,
						Status:  "Up 5 minutes",
						Ports: []container.PortSummary{
							{
								IP:          netip.MustParseAddr("0.0.0.0"),
								PrivatePort: 80,
								PublicPort:  8080,
								Type:        "tcp",
							},
						},
						Labels: map[string]string{"app": "web"},
					},
				},
			}, nil
		},
	}
	svc := NewContainerService(mock)
	resp, err := svc.List(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(resp.Containers))
	}
	c := resp.Containers[0]
	if c.ID != "c1" {
		t.Errorf("ID = %s", c.ID)
	}
	if c.State != "running" {
		t.Errorf("State = %s", c.State)
	}
	if len(c.Ports) != 1 || c.Ports[0].IP != "0.0.0.0" {
		t.Errorf("Ports = %+v", c.Ports)
	}
}

func TestList_AllFlag(t *testing.T) {
	var capturedAll bool
	mock := &mockDockerClient{
		containerListFn: func(ctx context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error) {
			capturedAll = opts.All
			return client.ContainerListResult{}, nil
		},
	}
	svc := NewContainerService(mock)
	_, _ = svc.List(context.Background(), true)
	if !capturedAll {
		t.Error("expected All=true to be passed through")
	}
}

func TestList_DockerError(t *testing.T) {
	mock := &mockDockerClient{
		containerListFn: func(ctx context.Context, opts client.ContainerListOptions) (client.ContainerListResult, error) {
			return client.ContainerListResult{}, errors.New("daemon error")
		},
	}
	svc := NewContainerService(mock)
	_, err := svc.List(context.Background(), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "list containers") {
		t.Errorf("expected 'list containers' in error, got: %v", err)
	}
}

// --- Inspect tests ---

func TestInspect_Success(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(ctx context.Context, id string, opts client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
			if id != "abc123" {
				t.Errorf("expected id abc123, got %s", id)
			}
			return client.ContainerInspectResult{
				Container: container.InspectResponse{
					ID: "abc123",
				},
			}, nil
		},
	}
	svc := NewContainerService(mock)
	resp, err := svc.Inspect(context.Background(), "abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "abc123" {
		t.Errorf("expected ID abc123, got %s", resp.ID)
	}
}

func TestInspect_DockerError(t *testing.T) {
	mock := &mockDockerClient{
		containerInspectFn: func(ctx context.Context, id string, opts client.ContainerInspectOptions) (client.ContainerInspectResult, error) {
			return client.ContainerInspectResult{}, errors.New("not found")
		},
	}
	svc := NewContainerService(mock)
	_, err := svc.Inspect(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "inspect container") {
		t.Errorf("expected 'inspect container' in error, got: %v", err)
	}
}

// --- Stop tests ---

func TestStop_Success(t *testing.T) {
	timeout := 10
	var capturedOpts client.ContainerStopOptions
	mock := &mockDockerClient{
		containerStopFn: func(ctx context.Context, id string, opts client.ContainerStopOptions) (client.ContainerStopResult, error) {
			if id != "c1" {
				t.Errorf("expected id c1, got %s", id)
			}
			capturedOpts = opts
			return client.ContainerStopResult{}, nil
		},
	}
	svc := NewContainerService(mock)
	err := svc.Stop(context.Background(), "c1", model.StopContainerRequest{
		Timeout: &timeout,
		Signal:  "SIGTERM",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOpts.Signal != "SIGTERM" {
		t.Errorf("Signal = %s", capturedOpts.Signal)
	}
	if capturedOpts.Timeout == nil || *capturedOpts.Timeout != 10 {
		t.Errorf("Timeout = %v", capturedOpts.Timeout)
	}
}

func TestStop_DockerError(t *testing.T) {
	mock := &mockDockerClient{
		containerStopFn: func(ctx context.Context, id string, opts client.ContainerStopOptions) (client.ContainerStopResult, error) {
			return client.ContainerStopResult{}, errors.New("No such container")
		},
	}
	svc := NewContainerService(mock)
	err := svc.Stop(context.Background(), "bad", model.StopContainerRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "stop container") {
		t.Errorf("expected 'stop container' in error, got: %v", err)
	}
}

// --- Remove tests ---

func TestRemove_Success(t *testing.T) {
	var capturedOpts client.ContainerRemoveOptions
	mock := &mockDockerClient{
		containerRemoveFn: func(ctx context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
			capturedOpts = opts
			return client.ContainerRemoveResult{}, nil
		},
	}
	svc := NewContainerService(mock)
	err := svc.Remove(context.Background(), "c1", model.RemoveContainerQuery{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !capturedOpts.Force {
		t.Error("expected Force=true")
	}
	if !capturedOpts.RemoveVolumes {
		t.Error("expected RemoveVolumes=true")
	}
}

func TestRemove_DockerError(t *testing.T) {
	mock := &mockDockerClient{
		containerRemoveFn: func(ctx context.Context, id string, opts client.ContainerRemoveOptions) (client.ContainerRemoveResult, error) {
			return client.ContainerRemoveResult{}, errors.New("conflict")
		},
	}
	svc := NewContainerService(mock)
	err := svc.Remove(context.Background(), "c1", model.RemoveContainerQuery{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "remove container") {
		t.Errorf("expected 'remove container' in error, got: %v", err)
	}
}

// --- Logs tests ---

type nopCloser struct {
	io.Reader
}

func (nopCloser) Close() error { return nil }

func TestLogs_Success_DefaultTail(t *testing.T) {
	var capturedOpts client.ContainerLogsOptions
	mock := &mockDockerClient{
		containerLogsFn: func(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
			capturedOpts = opts
			return nopCloser{strings.NewReader("line1\nline2\n")}, nil
		},
	}
	svc := NewContainerService(mock)
	rc, err := svc.Logs(context.Background(), "c1", model.LogsQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()
	if capturedOpts.Tail != "100" {
		t.Errorf("expected Tail=100, got %s", capturedOpts.Tail)
	}
	if !capturedOpts.ShowStdout || !capturedOpts.ShowStderr {
		t.Error("expected ShowStdout and ShowStderr to be true")
	}
}

func TestLogs_Success_CustomOptions(t *testing.T) {
	var capturedOpts client.ContainerLogsOptions
	mock := &mockDockerClient{
		containerLogsFn: func(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
			capturedOpts = opts
			return nopCloser{strings.NewReader("")}, nil
		},
	}
	svc := NewContainerService(mock)
	_, err := svc.Logs(context.Background(), "c1", model.LogsQuery{
		Follow:     true,
		Tail:       "50",
		Since:      "2024-01-01T00:00:00Z",
		Until:      "2024-12-31T23:59:59Z",
		Timestamps: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOpts.Tail != "50" {
		t.Errorf("Tail = %s", capturedOpts.Tail)
	}
	if !capturedOpts.Follow {
		t.Error("expected Follow=true")
	}
	if !capturedOpts.Timestamps {
		t.Error("expected Timestamps=true")
	}
	if capturedOpts.Since != "2024-01-01T00:00:00Z" {
		t.Errorf("Since = %s", capturedOpts.Since)
	}
	if capturedOpts.Until != "2024-12-31T23:59:59Z" {
		t.Errorf("Until = %s", capturedOpts.Until)
	}
}

func TestLogs_DockerError(t *testing.T) {
	mock := &mockDockerClient{
		containerLogsFn: func(ctx context.Context, id string, opts client.ContainerLogsOptions) (client.ContainerLogsResult, error) {
			return nil, errors.New("not found")
		},
	}
	svc := NewContainerService(mock)
	_, err := svc.Logs(context.Background(), "bad", model.LogsQuery{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "container logs") {
		t.Errorf("expected 'container logs' in error, got: %v", err)
	}
}

// --- Ping tests ---

func TestPing_Success(t *testing.T) {
	mock := &mockDockerClient{
		pingFn: func(ctx context.Context, opts client.PingOptions) (client.PingResult, error) {
			return client.PingResult{}, nil
		},
	}
	svc := NewContainerService(mock)
	if err := svc.Ping(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPing_Error(t *testing.T) {
	mock := &mockDockerClient{
		pingFn: func(ctx context.Context, opts client.PingOptions) (client.PingResult, error) {
			return client.PingResult{}, errors.New("connection refused")
		},
	}
	svc := NewContainerService(mock)
	err := svc.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "connection refused" {
		t.Errorf("expected 'connection refused', got: %v", err)
	}
}
