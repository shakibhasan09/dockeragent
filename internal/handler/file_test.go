package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// --- mock file service ---

type mockFileService struct {
	writeFileFn func(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error)
}

func (m *mockFileService) WriteFile(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error) {
	return m.writeFileFn(ctx, req)
}

// --- mock symlink evaluator ---

type mockSymlinkEvaluator struct {
	evalSymlinksFn func(path string) (string, error)
}

func (m *mockSymlinkEvaluator) EvalSymlinks(path string) (string, error) {
	return m.evalSymlinksFn(path)
}

// --- WriteFile handler tests ---

func TestWriteFile_Success(t *testing.T) {
	mock := &mockFileService{
		writeFileFn: func(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error) {
			if req.Path != "/host/tmp/test.txt" {
				t.Errorf("expected path /host/tmp/test.txt, got %s", req.Path)
			}
			return model.WriteFileResponse{
				Path:    req.Path,
				Size:    int64(len(req.Content)),
				Message: "file written successfully",
			}, nil
		},
	}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) {
			return path, nil
		},
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/tmp/test.txt","content":"hello world"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	var body model.WriteFileResponse
	decodeJSON(t, resp, &body)
	if body.Path != "/tmp/test.txt" {
		t.Errorf("expected path /tmp/test.txt, got %s", body.Path)
	}
	if body.Size != 11 {
		t.Errorf("expected size 11, got %d", body.Size)
	}
}

func TestWriteFile_WithPermission(t *testing.T) {
	var capturedPerm string
	mock := &mockFileService{
		writeFileFn: func(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error) {
			capturedPerm = req.Permission
			return model.WriteFileResponse{Path: req.Path, Size: 5, Message: "ok"}, nil
		},
	}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) { return path, nil },
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/tmp/f.txt","content":"hello","permission":"0755"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	resp.Body.Close()
	if capturedPerm != "0755" {
		t.Errorf("expected permission 0755, got %s", capturedPerm)
	}
}

func TestWriteFile_InvalidJSON(t *testing.T) {
	mock := &mockFileService{}
	h := NewFileHandler(mock)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files", `{invalid`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWriteFile_MissingPath(t *testing.T) {
	mock := &mockFileService{}
	h := NewFileHandler(mock)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files", `{"content":"hello"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	var body model.ErrorResponse
	decodeJSON(t, resp, &body)
	if !strings.Contains(body.Message, "path is required") {
		t.Errorf("expected 'path is required', got: %s", body.Message)
	}
}

func TestWriteFile_RelativePath(t *testing.T) {
	mock := &mockFileService{}
	h := NewFileHandler(mock)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"relative/path.txt","content":"hello"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	var body model.ErrorResponse
	decodeJSON(t, resp, &body)
	if !strings.Contains(body.Message, "path must be absolute") {
		t.Errorf("expected 'path must be absolute', got: %s", body.Message)
	}
}

func TestWriteFile_PathTraversal_DoubleDot(t *testing.T) {
	// Paths where ".." survives filepath.Clean are rejected.
	// In practice filepath.Clean resolves ".." so this test uses
	// the symlink evaluator to verify the /host prefix check.
	mock := &mockFileService{}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) {
			// Simulate parent resolving outside /host
			return "/outside/host", nil
		},
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/etc/passwd","content":"bad"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWriteFile_InvalidPermission(t *testing.T) {
	mock := &mockFileService{}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) { return path, nil },
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/tmp/test.txt","content":"hello","permission":"999"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	var body model.ErrorResponse
	decodeJSON(t, resp, &body)
	if !strings.Contains(body.Message, "permission must be a valid octal") {
		t.Errorf("expected octal error, got: %s", body.Message)
	}
}

func TestWriteFile_ContentTooLarge(t *testing.T) {
	mock := &mockFileService{}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) { return path, nil },
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	// Use a custom app with a large enough body limit so the request reaches our handler validation.
	app := fiber.New(fiber.Config{
		ErrorHandler: testErrorHandler,
		BodyLimit:    MaxFileSize + 1024*1024, // allow body through to handler
	})
	app.Post("/files", h.WriteFile)
	largeContent := strings.Repeat("x", MaxFileSize+1)
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/tmp/big.txt","content":"`+largeContent+`"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWriteFile_SymlinkTraversal(t *testing.T) {
	mock := &mockFileService{}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) {
			// Simulate a symlink that resolves outside /host
			return "/etc/shadow", nil
		},
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/tmp/link/test.txt","content":"bad"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	var body model.ErrorResponse
	decodeJSON(t, resp, &body)
	if !strings.Contains(body.Message, "symlinks") {
		t.Errorf("expected symlink error, got: %s", body.Message)
	}
}

func TestWriteFile_ServiceError_PermissionDenied(t *testing.T) {
	mock := &mockFileService{
		writeFileFn: func(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error) {
			return model.WriteFileResponse{}, errors.New("permission denied")
		},
	}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) { return path, nil },
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/tmp/test.txt","content":"hello"}`)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestWriteFile_ServiceError_Generic(t *testing.T) {
	mock := &mockFileService{
		writeFileFn: func(ctx context.Context, req model.WriteFileRequest) (model.WriteFileResponse, error) {
			return model.WriteFileResponse{}, errors.New("disk full")
		},
	}
	se := &mockSymlinkEvaluator{
		evalSymlinksFn: func(path string) (string, error) { return path, nil },
	}
	h := NewFileHandlerWithSymlinks(mock, se)
	app := newTestApp(h.WriteFile, http.MethodPost, "/files")
	resp := doRequest(t, app, http.MethodPost, "/files",
		`{"path":"/tmp/test.txt","content":"hello"}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- classifyFileError tests ---

func TestClassifyFileError_PermissionDenied(t *testing.T) {
	err := classifyFileError(errors.New("permission denied"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusForbidden {
		t.Errorf("expected 403, got %d", fe.Code)
	}
}

func TestClassifyFileError_InvalidPermission(t *testing.T) {
	err := classifyFileError(errors.New("invalid permission"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusBadRequest {
		t.Errorf("expected 400, got %d", fe.Code)
	}
}

func TestClassifyFileError_Generic(t *testing.T) {
	err := classifyFileError(errors.New("disk full"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusInternalServerError {
		t.Errorf("expected 500, got %d", fe.Code)
	}
}
