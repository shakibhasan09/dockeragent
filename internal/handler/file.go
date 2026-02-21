package handler

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// FileServicer is the service interface used by FileHandler.
type FileServicer interface {
	WriteFile(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error)
}

type FileHandler struct {
	svc FileServicer
}

func NewFileHandler(svc FileServicer) *FileHandler {
	return &FileHandler{svc: svc}
}

func (h *FileHandler) WriteFile(c fiber.Ctx) error {
	var req model.WriteFileRequest
	if err := c.Bind().JSON(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body: "+err.Error())
	}

	if req.Path == "" {
		return fiber.NewError(fiber.StatusBadRequest, "path is required")
	}
	if !filepath.IsAbs(req.Path) {
		return fiber.NewError(fiber.StatusBadRequest, "path must be absolute")
	}
	cleaned := filepath.Clean(req.Path)
	if strings.Contains(cleaned, "..") {
		return fiber.NewError(fiber.StatusBadRequest, "path must not contain '..'")
	}
	req.Path = cleaned

	if req.Permission != "" {
		if _, err := strconv.ParseUint(req.Permission, 8, 32); err != nil {
			return fiber.NewError(fiber.StatusBadRequest,
				"permission must be a valid octal string (e.g., \"0644\")")
		}
	}

	resp, err := h.svc.WriteFile(c.Context(), req)
	if err != nil {
		return classifyFileError(err)
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func classifyFileError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "permission denied") {
		return fiber.NewError(fiber.StatusForbidden, msg)
	}
	if strings.Contains(msg, "invalid permission") {
		return fiber.NewError(fiber.StatusBadRequest, msg)
	}
	return fiber.NewError(fiber.StatusInternalServerError, msg)
}
