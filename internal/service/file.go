package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// FileSystem abstracts filesystem operations for testability.
type FileSystem interface {
	MkdirAll(path string, perm os.FileMode) error
	WriteFile(name string, data []byte, perm os.FileMode) error
}

// OSFileSystem implements FileSystem using the real os package.
type OSFileSystem struct{}

func (OSFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (OSFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
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
