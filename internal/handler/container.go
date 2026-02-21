package handler

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shakibhasan09/dockeragent/internal/model"
	"github.com/shakibhasan09/dockeragent/internal/service"
)

type ContainerHandler struct {
	svc *service.ContainerService
}

func NewContainerHandler(svc *service.ContainerService) *ContainerHandler {
	return &ContainerHandler{svc: svc}
}

func (h *ContainerHandler) CreateContainer(c fiber.Ctx) error {
	var req model.CreateContainerRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body: "+err.Error())
	}

	if req.Image == "" {
		return fiber.NewError(fiber.StatusBadRequest, "image is required")
	}
	if req.RestartPolicy != nil {
		switch req.RestartPolicy.Name {
		case "no", "always", "on-failure", "unless-stopped":
		default:
			return fiber.NewError(fiber.StatusBadRequest,
				"restart_policy.name must be one of: no, always, on-failure, unless-stopped")
		}
	}
	if req.Resources != nil {
		if req.Resources.CPUs < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "resources.cpus cannot be negative")
		}
		if req.Resources.MemoryMB < 0 {
			return fiber.NewError(fiber.StatusBadRequest, "resources.memory_mb cannot be negative")
		}
	}
	for _, p := range req.Ports {
		if p.ContainerPort == "" {
			return fiber.NewError(fiber.StatusBadRequest, "ports[].container_port is required")
		}
	}
	for _, v := range req.Volumes {
		if v.Source == "" || v.Target == "" {
			return fiber.NewError(fiber.StatusBadRequest, "volumes[].source and volumes[].target are required")
		}
	}

	resp, err := h.svc.Create(c.Context(), req)
	if err != nil {
		return classifyDockerError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func (h *ContainerHandler) ListContainers(c fiber.Ctx) error {
	all := c.Query("all") == "true"
	resp, err := h.svc.List(c.Context(), all)
	if err != nil {
		return classifyDockerError(err)
	}
	return c.JSON(resp)
}

func (h *ContainerHandler) InspectContainer(c fiber.Ctx) error {
	id := c.Params("id")
	resp, err := h.svc.Inspect(c.Context(), id)
	if err != nil {
		return classifyDockerError(err)
	}
	return c.JSON(resp)
}

func (h *ContainerHandler) StopContainer(c fiber.Ctx) error {
	id := c.Params("id")
	var req model.StopContainerRequest
	_ = c.Bind().JSON(&req)

	if err := h.svc.Stop(c.Context(), id, req); err != nil {
		return classifyDockerError(err)
	}
	return c.JSON(model.MessageResponse{Message: "container stopped"})
}

func (h *ContainerHandler) RemoveContainer(c fiber.Ctx) error {
	id := c.Params("id")
	var q model.RemoveContainerQuery
	if err := c.Bind().Query(&q); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid query parameters")
	}

	if err := h.svc.Remove(c.Context(), id, q); err != nil {
		return classifyDockerError(err)
	}
	return c.JSON(model.MessageResponse{Message: "container removed"})
}

func (h *ContainerHandler) GetContainerLogs(c fiber.Ctx) error {
	id := c.Params("id")
	var q model.LogsQuery
	if err := c.Bind().Query(&q); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid query parameters")
	}

	reader, err := h.svc.Logs(c.Context(), id, q)
	if err != nil {
		return classifyDockerError(err)
	}

	if !q.Follow {
		defer reader.Close()
		var lines []string
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		return c.JSON(fiber.Map{"lines": lines})
	}

	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")

	return c.SendStreamWriter(func(w *bufio.Writer) {
		defer reader.Close()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			fmt.Fprintf(w, "data: %s\n\n", scanner.Text())
			if err := w.Flush(); err != nil {
				return
			}
		}
	})
}

func (h *ContainerHandler) HealthCheck(c fiber.Ctx) error {
	resp := model.HealthResponse{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	if err := h.svc.Ping(c.Context()); err != nil {
		resp.Status = "degraded"
		resp.Docker = err.Error()
		return c.Status(fiber.StatusServiceUnavailable).JSON(resp)
	}
	resp.Status = "ok"
	resp.Docker = "connected"
	return c.JSON(resp)
}

func classifyDockerError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "not found") || strings.Contains(msg, "No such container") {
		return fiber.NewError(fiber.StatusNotFound, msg)
	}
	if strings.Contains(msg, "conflict") || strings.Contains(msg, "already in use") {
		return fiber.NewError(fiber.StatusConflict, msg)
	}
	if strings.Contains(msg, "not modified") {
		return fiber.NewError(fiber.StatusNotModified, msg)
	}
	return fiber.NewError(fiber.StatusInternalServerError, msg)
}
