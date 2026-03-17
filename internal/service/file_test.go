package service

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// --- mock file system ---

type mockFileSystem struct {
	mkdirAllFn  func(path string, perm os.FileMode) error
	writeFileFn func(name string, data []byte, perm os.FileMode) error
}

func (m *mockFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return m.mkdirAllFn(path, perm)
}

func (m *mockFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return m.writeFileFn(name, data, perm)
}

// --- WriteFile tests ---

func TestFileService_WriteFile_Success_DefaultPermission(t *testing.T) {
	var capturedPath string
	var capturedPerm os.FileMode
	var capturedData []byte
	mock := &mockFileSystem{
		mkdirAllFn: func(path string, perm os.FileMode) error {
			return nil
		},
		writeFileFn: func(name string, data []byte, perm os.FileMode) error {
			capturedPath = name
			capturedData = data
			capturedPerm = perm
			return nil
		},
	}
	svc := NewFileService(mock)
	resp, err := svc.WriteFile(context.Background(), writeFileReq("/host/tmp/test.txt", "hello world", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPath != "/host/tmp/test.txt" {
		t.Errorf("expected path /host/tmp/test.txt, got %s", capturedPath)
	}
	if string(capturedData) != "hello world" {
		t.Errorf("expected data 'hello world', got %s", string(capturedData))
	}
	if capturedPerm != 0644 {
		t.Errorf("expected perm 0644, got %o", capturedPerm)
	}
	if resp.Path != "/host/tmp/test.txt" {
		t.Errorf("resp.Path = %s", resp.Path)
	}
	if resp.Size != 11 {
		t.Errorf("expected size 11, got %d", resp.Size)
	}
	if resp.Message != "file written successfully" {
		t.Errorf("Message = %s", resp.Message)
	}
}

func TestFileService_WriteFile_Success_CustomPermission(t *testing.T) {
	var capturedPerm os.FileMode
	mock := &mockFileSystem{
		mkdirAllFn: func(path string, perm os.FileMode) error {
			return nil
		},
		writeFileFn: func(name string, data []byte, perm os.FileMode) error {
			capturedPerm = perm
			return nil
		},
	}
	svc := NewFileService(mock)
	_, err := svc.WriteFile(context.Background(), writeFileReq("/host/tmp/script.sh", "#!/bin/bash", "0755"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedPerm != 0755 {
		t.Errorf("expected perm 0755, got %o", capturedPerm)
	}
}

func TestFileService_WriteFile_InvalidPermission(t *testing.T) {
	mock := &mockFileSystem{
		mkdirAllFn:  func(path string, perm os.FileMode) error { return nil },
		writeFileFn: func(name string, data []byte, perm os.FileMode) error { return nil },
	}
	svc := NewFileService(mock)
	_, err := svc.WriteFile(context.Background(), writeFileReq("/host/tmp/test.txt", "hello", "zzz"))
	if err == nil {
		t.Fatal("expected error for invalid permission")
	}
	if !strings.Contains(err.Error(), "invalid permission") {
		t.Errorf("expected 'invalid permission' in error, got: %v", err)
	}
}

func TestFileService_WriteFile_MkdirError(t *testing.T) {
	mock := &mockFileSystem{
		mkdirAllFn: func(path string, perm os.FileMode) error {
			return errors.New("permission denied")
		},
		writeFileFn: func(name string, data []byte, perm os.FileMode) error {
			return nil
		},
	}
	svc := NewFileService(mock)
	_, err := svc.WriteFile(context.Background(), writeFileReq("/host/tmp/test.txt", "hello", ""))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create directories") {
		t.Errorf("expected 'create directories' in error, got: %v", err)
	}
}

func TestFileService_WriteFile_WriteError(t *testing.T) {
	mock := &mockFileSystem{
		mkdirAllFn: func(path string, perm os.FileMode) error {
			return nil
		},
		writeFileFn: func(name string, data []byte, perm os.FileMode) error {
			return errors.New("disk full")
		},
	}
	svc := NewFileService(mock)
	_, err := svc.WriteFile(context.Background(), writeFileReq("/host/tmp/test.txt", "hello", ""))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "write file") {
		t.Errorf("expected 'write file' in error, got: %v", err)
	}
}

func TestFileService_WriteFile_MkdirCreatesParentWith0755(t *testing.T) {
	var capturedDirPerm os.FileMode
	mock := &mockFileSystem{
		mkdirAllFn: func(path string, perm os.FileMode) error {
			capturedDirPerm = perm
			return nil
		},
		writeFileFn: func(name string, data []byte, perm os.FileMode) error {
			return nil
		},
	}
	svc := NewFileService(mock)
	_, err := svc.WriteFile(context.Background(), writeFileReq("/host/tmp/sub/dir/test.txt", "hello", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedDirPerm != 0755 {
		t.Errorf("expected dir perm 0755, got %o", capturedDirPerm)
	}
}

func TestFileService_WriteFile_EmptyContent(t *testing.T) {
	mock := &mockFileSystem{
		mkdirAllFn: func(path string, perm os.FileMode) error { return nil },
		writeFileFn: func(name string, data []byte, perm os.FileMode) error {
			return nil
		},
	}
	svc := NewFileService(mock)
	resp, err := svc.WriteFile(context.Background(), writeFileReq("/host/tmp/empty.txt", "", ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Size != 0 {
		t.Errorf("expected size 0, got %d", resp.Size)
	}
}

// helper to build a WriteFileRequest
func writeFileReq(path, content, perm string) model.WriteFileRequest {
	return model.WriteFileRequest{
		Path:       path,
		Content:    content,
		Permission: perm,
	}
}
