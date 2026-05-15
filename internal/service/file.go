package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

// OSFileSystem implements FileSystem using the real os package. The
// WriteFile implementation opens with O_NOFOLLOW so a pre-existing
// symlink at the target path is rejected (EMLINK / ELOOP) rather than
// followed — the handler's symlink check only verifies ancestors, so
// without O_NOFOLLOW a TOCTOU swap or a leaf symlink would still
// silently write through.
type OSFileSystem struct{}

func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, perm)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

type FileService struct {
	fs FileSystem
}

func NewFileService(fs FileSystem) *FileService {
	return &FileService{fs: fs}
}

func (s *FileService) WriteFile(_ context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error) {
	permStr := req.Permission
	if permStr == "" {
		permStr = "0644"
	}
	permUint, err := strconv.ParseUint(permStr, 8, 32)
	if err != nil {
		return model.WriteFileResponse{}, fmt.Errorf("invalid permission %q: %w", permStr, err)
	}
	perm := os.FileMode(permUint)

	dir := filepath.Dir(req.Path)
	if err := s.fs.MkdirAll(dir, 0755); err != nil {
		return model.WriteFileResponse{}, fmt.Errorf("create directories: %w", err)
	}

	data := []byte(req.Content)
	if err := s.fs.WriteFile(req.Path, data, perm); err != nil {
		return model.WriteFileResponse{}, fmt.Errorf("write file: %w", err)
	}

	return model.WriteFileResponse{
		Path:    req.Path,
		Size:    int64(len(data)),
		Message: "file written successfully",
	}, nil
}
