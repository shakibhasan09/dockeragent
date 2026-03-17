package handler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// MaxFileSize is the maximum allowed file content size (10 MB).
const MaxFileSize = 10 * 1024 * 1024

// FileServicer is the service interface used by FileHandler.
type FileServicer interface {
	WriteFile(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error)
}

// SymlinkEvaluator abstracts symlink resolution for testability.
type SymlinkEvaluator interface {
	EvalSymlinks(path string) (string, error)
}

// OSSymlinkEvaluator uses the real os/filepath package.
type OSSymlinkEvaluator struct{}

func (OSSymlinkEvaluator) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

type FileHandler struct {
	svc      FileServicer
	symlinks SymlinkEvaluator
}

func NewFileHandler(svc FileServicer) *FileHandler {
	return &FileHandler{svc: svc, symlinks: OSSymlinkEvaluator{}}
}

// NewFileHandlerWithSymlinks creates a FileHandler with a custom SymlinkEvaluator (for testing).
func NewFileHandlerWithSymlinks(svc FileServicer, se SymlinkEvaluator) *FileHandler {
	return &FileHandler{svc: svc, symlinks: se}
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

	if len(req.Content) > MaxFileSize {
		return fiber.NewError(fiber.StatusBadRequest, "content exceeds maximum file size of 10 MB")
	}

	hostPath := filepath.Join("/host", cleaned)

	// Resolve symlinks on the parent directory to prevent symlink traversal.
	parentDir := filepath.Dir(hostPath)
	resolvedParent, err := h.symlinks.EvalSymlinks(parentDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fiber.NewError(fiber.StatusBadRequest, "cannot resolve path: "+err.Error())
	}
	// If the parent exists, verify it's still under /host.
	if err == nil && !strings.HasPrefix(resolvedParent, "/host") {
		return fiber.NewError(fiber.StatusBadRequest, "path must not escape the host mount via symlinks")
	}

	req.Path = hostPath

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
	resp.Path = cleaned
	return c.Status(fiber.StatusCreated).JSON(resp)
}

func classifyFileError(err error) error {
	msg := err.Error()
	if errors.Is(err, os.ErrPermission) || strings.Contains(msg, "permission denied") {
		return fiber.NewError(fiber.StatusForbidden, msg)
	}
	if strings.Contains(msg, "invalid permission") {
		return fiber.NewError(fiber.StatusBadRequest, msg)
	}
	return fiber.NewError(fiber.StatusInternalServerError, msg)
}
