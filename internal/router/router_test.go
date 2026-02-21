package router

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/moby/moby/api/types/container"

	"github.com/shakibhasan09/dockeragent/internal/config"
	"github.com/shakibhasan09/dockeragent/internal/handler"
	"github.com/shakibhasan09/dockeragent/internal/model"
)

// --- mock service for router tests ---

type mockService struct {
	listFn func(ctx context.Context, all bool) (model.ContainerListResponse, error)
	pingFn func(ctx context.Context) error
}

func (m *mockService) Create(ctx context.Context, req model.CreateContainerRequest) (model.CreateContainerResponse, error) {
	return model.CreateContainerResponse{}, nil
}
func (m *mockService) List(ctx context.Context, all bool) (model.ContainerListResponse, error) {
	if m.listFn != nil {
		return m.listFn(ctx, all)
	}
	return model.ContainerListResponse{}, nil
}
func (m *mockService) Inspect(ctx context.Context, id string) (container.InspectResponse, error) {
	return container.InspectResponse{}, nil
}
func (m *mockService) Stop(ctx context.Context, id string, req model.StopContainerRequest) error {
	return nil
}
func (m *mockService) Remove(ctx context.Context, id string, q model.RemoveContainerQuery) error {
	return nil
}
func (m *mockService) Logs(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error) {
	return nil, nil
}
func (m *mockService) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

// --- ErrorHandler tests ---

func TestErrorHandler_FiberError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})
	app.Get("/test", func(c fiber.Ctx) error {
		return fiber.NewError(fiber.StatusBadRequest, "bad input")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	var body model.ErrorResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Status != 400 {
		t.Errorf("body.Status = %d", body.Status)
	}
	if body.Message != "bad input" {
		t.Errorf("body.Message = %s", body.Message)
	}
	if body.Error != "Bad Request" {
		t.Errorf("body.Error = %s", body.Error)
	}
}

func TestErrorHandler_GenericError(t *testing.T) {
	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})
	app.Get("/test", func(c fiber.Ctx) error {
		return errors.New("boom")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
	var body model.ErrorResponse
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Status != 500 {
		t.Errorf("body.Status = %d", body.Status)
	}
	if body.Message != "internal server error" {
		t.Errorf("body.Message = %s", body.Message)
	}
}

// --- Route auth tests ---

type mockFileService struct{}

func (m *mockFileService) WriteFile(_ context.Context, _ model.WriteFileRequest) (model.WriteFileResponse, error) {
	return model.WriteFileResponse{Path: "/tmp/test", Size: 2, Message: "file written successfully"}, nil
}

func newTestRouterApp(svc *mockService, apiKey string) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})
	ch := handler.NewContainerHandler(svc)
	fh := handler.NewFileHandler(&mockFileService{})
	cfg := config.Config{APIKey: apiKey}
	Setup(app, ch, fh, cfg)
	return app
}

func TestRoutes_HealthBypassesAuth(t *testing.T) {
	app := newTestRouterApp(&mockService{}, "secret")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRoutes_APIRequiresAuth(t *testing.T) {
	app := newTestRouterApp(&mockService{}, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRoutes_APIWithValidKey(t *testing.T) {
	app := newTestRouterApp(&mockService{}, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	req.Header.Set("X-API-Key", "secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRoutes_APIWithInvalidKey(t *testing.T) {
	app := newTestRouterApp(&mockService{}, "secret")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/containers", nil)
	req.Header.Set("X-API-Key", "wrong")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRoutes_FilesRequiresAuth(t *testing.T) {
	app := newTestRouterApp(&mockService{}, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", strings.NewReader(`{"path":"/tmp/test","content":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRoutes_FilesWithValidKey(t *testing.T) {
	app := newTestRouterApp(&mockService{}, "secret")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/files", strings.NewReader(`{"path":"/tmp/test","content":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "secret")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}
