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
	// filepath.Clean resolves ".." for absolute paths, so this is a defence
	// against future regressions rather than a real-world payload check.
	if strings.Contains(cleaned, "..") {
		return fiber.NewError(fiber.StatusBadRequest, "path must not contain '..'")
	}

	if len(req.Content) > MaxFileSize {
		return fiber.NewError(fiber.StatusBadRequest, "content exceeds maximum file size of 10 MB")
	}

	hostPath := filepath.Join("/host", cleaned)

	// Walk up to the nearest existing ancestor and verify it resolves under
	// /host. Previously only the immediate parent was checked; if the parent
	// did not exist the symlink check was skipped entirely, letting an
	// attacker plant a symlink deeper in the path (e.g. /host/a/b where /a
	// is a symlink escaping /host). Walking up plugs that gap.
	if err := h.verifyUnderHost(hostPath); err != nil {
		return err
	}

	req.Path = hostPath

	if req.Permission != "" {
		v, err := strconv.ParseUint(req.Permission, 8, 32)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest,
				"permission must be a valid octal string (e.g., \"0644\")")
		}
		// Cap to the standard 12 permission bits (sticky/setuid/setgid + rwx);
		// anything above is almost certainly a typo (e.g. "77777") and silently
		// passing huge values to os.FileMode would set unrelated mode bits.
		if v > 0o7777 {
			return fiber.NewError(fiber.StatusBadRequest,
				"permission must be within 0o7777")
		}
	}

	resp, err := h.svc.WriteFile(c.Context(), req)
	if err != nil {
		return classifyFileError(err)
	}
	resp.Path = cleaned
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// verifyUnderHost walks up from hostPath until EvalSymlinks resolves an
// existing component, then asserts the resolved path is contained in
// /host. Returns a 400 Fiber error on rejection.
func (h *FileHandler) verifyUnderHost(hostPath string) error {
	candidate := hostPath
	for {
		resolved, err := h.symlinks.EvalSymlinks(candidate)
		if err == nil {
			if !isUnderHost(resolved) {
				return fiber.NewError(fiber.StatusBadRequest,
					"path must not escape the host mount via symlinks")
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fiber.NewError(fiber.StatusBadRequest, "cannot resolve path: "+err.Error())
		}
		parent := filepath.Dir(candidate)
		if parent == candidate {
			return fiber.NewError(fiber.StatusBadRequest,
				"no existing ancestor under /host")
		}
		candidate = parent
	}
}

// isUnderHost returns true iff p equals /host or sits beneath it. A simple
// strings.HasPrefix(p, "/host") would also match /hostile.
func isUnderHost(p string) bool {
	return p == "/host" || strings.HasPrefix(p, "/host/")
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
