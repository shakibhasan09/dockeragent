package handler

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

	"github.com/shakibhasan09/dockeragent/internal/model"
)

// --- mock service ---

type mockContainerService struct {
	createFn  func(ctx context.Context, req model.CreateContainerRequest) (model.CreateContainerResponse, error)
	listFn    func(ctx context.Context, all bool) (model.ContainerListResponse, error)
	inspectFn func(ctx context.Context, id string) (container.InspectResponse, error)
	stopFn    func(ctx context.Context, id string, req model.StopContainerRequest) error
	removeFn  func(ctx context.Context, id string, q model.RemoveContainerQuery) error
	logsFn    func(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error)
	pingFn    func(ctx context.Context) error
}

func (m *mockContainerService) Create(ctx context.Context, req model.CreateContainerRequest) (model.CreateContainerResponse, error) {
	return m.createFn(ctx, req)
}
func (m *mockContainerService) List(ctx context.Context, all bool) (model.ContainerListResponse, error) {
	return m.listFn(ctx, all)
}
func (m *mockContainerService) Inspect(ctx context.Context, id string) (container.InspectResponse, error) {
	return m.inspectFn(ctx, id)
}
func (m *mockContainerService) Stop(ctx context.Context, id string, req model.StopContainerRequest) error {
	return m.stopFn(ctx, id, req)
}
func (m *mockContainerService) Remove(ctx context.Context, id string, q model.RemoveContainerQuery) error {
	return m.removeFn(ctx, id, q)
}
func (m *mockContainerService) Logs(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error) {
	return m.logsFn(ctx, id, q)
}
func (m *mockContainerService) Ping(ctx context.Context) error {
	return m.pingFn(ctx)
}

// --- helpers ---

// testErrorHandler mirrors router.ErrorHandler without importing router (avoids cycle).
func testErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := "internal server error"
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		msg = e.Message
	}
	return c.Status(code).JSON(model.ErrorResponse{
		Error:   http.StatusText(code),
		Message: msg,
		Status:  code,
	})
}

func newTestApp(handler fiber.Handler, method, path string) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: testErrorHandler,
	})
	switch method {
	case http.MethodGet:
		app.Get(path, handler)
	case http.MethodPost:
		app.Post(path, handler)
	case http.MethodDelete:
		app.Delete(path, handler)
	}
	return app
}

func doRequest(t *testing.T, app *fiber.App, method, url string, body string) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, url, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
}

// --- classifyDockerError tests ---

func TestClassifyDockerError_NotFound(t *testing.T) {
	err := classifyDockerError(errors.New("container not found"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusNotFound {
		t.Errorf("expected 404, got %d", fe.Code)
	}
}

func TestClassifyDockerError_NoSuchContainer(t *testing.T) {
	err := classifyDockerError(errors.New("No such container: abc123"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusNotFound {
		t.Errorf("expected 404, got %d", fe.Code)
	}
}

func TestClassifyDockerError_Conflict(t *testing.T) {
	err := classifyDockerError(errors.New("name conflict"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusConflict {
		t.Errorf("expected 409, got %d", fe.Code)
	}
}

func TestClassifyDockerError_AlreadyInUse(t *testing.T) {
	err := classifyDockerError(errors.New("name already in use"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusConflict {
		t.Errorf("expected 409, got %d", fe.Code)
	}
}

func TestClassifyDockerError_NotModified(t *testing.T) {
	err := classifyDockerError(errors.New("container not modified"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusNotModified {
		t.Errorf("expected 304, got %d", fe.Code)
	}
}

func TestClassifyDockerError_Generic(t *testing.T) {
	err := classifyDockerError(errors.New("something went wrong"))
	var fe *fiber.Error
	if !errors.As(err, &fe) {
		t.Fatal("expected *fiber.Error")
	}
	if fe.Code != fiber.StatusInternalServerError {
		t.Errorf("expected 500, got %d", fe.Code)
	}
}

// --- CreateContainer tests ---

func TestCreateContainer_Success(t *testing.T) {
	mock := &mockContainerService{
		createFn: func(ctx context.Context, req model.CreateContainerRequest) (model.CreateContainerResponse, error) {
			if req.Image != "nginx" {
				t.Errorf("Image = %s", req.Image)
			}
			return model.CreateContainerResponse{ID: "abc123"}, nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers", `{"image":"nginx"}`)
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	var body model.CreateContainerResponse
	decodeJSON(t, resp, &body)
	if body.ID != "abc123" {
		t.Errorf("ID = %s", body.ID)
	}
}

func TestCreateContainer_InvalidJSON(t *testing.T) {
	mock := &mockContainerService{}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers", `{invalid`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateContainer_MissingImage(t *testing.T) {
	mock := &mockContainerService{}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers", `{"name":"test"}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	var body model.ErrorResponse
	decodeJSON(t, resp, &body)
	if !strings.Contains(body.Message, "image is required") {
		t.Errorf("expected 'image is required', got: %s", body.Message)
	}
}

func TestCreateContainer_InvalidRestartPolicy(t *testing.T) {
	mock := &mockContainerService{}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers",
		`{"image":"nginx","restart_policy":{"name":"invalid"}}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateContainer_NegativeCPUs(t *testing.T) {
	mock := &mockContainerService{}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers",
		`{"image":"nginx","resources":{"cpus":-1}}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateContainer_NegativeMemory(t *testing.T) {
	mock := &mockContainerService{}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers",
		`{"image":"nginx","resources":{"memory_mb":-1}}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateContainer_MissingContainerPort(t *testing.T) {
	mock := &mockContainerService{}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers",
		`{"image":"nginx","ports":[{"host_port":"8080"}]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateContainer_MissingVolumeTarget(t *testing.T) {
	mock := &mockContainerService{}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers",
		`{"image":"nginx","volumes":[{"source":"/host"}]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateContainer_ServiceError_NotFound(t *testing.T) {
	mock := &mockContainerService{
		createFn: func(ctx context.Context, req model.CreateContainerRequest) (model.CreateContainerResponse, error) {
			return model.CreateContainerResponse{}, errors.New("image not found")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers", `{"image":"bad"}`)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCreateContainer_ServiceError_Conflict(t *testing.T) {
	mock := &mockContainerService{
		createFn: func(ctx context.Context, req model.CreateContainerRequest) (model.CreateContainerResponse, error) {
			return model.CreateContainerResponse{}, errors.New("name conflict")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.CreateContainer, http.MethodPost, "/containers")
	resp := doRequest(t, app, http.MethodPost, "/containers", `{"image":"nginx"}`)
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}

// --- ListContainers tests ---

func TestListContainers_Success(t *testing.T) {
	mock := &mockContainerService{
		listFn: func(ctx context.Context, all bool) (model.ContainerListResponse, error) {
			return model.ContainerListResponse{
				Containers: []model.ContainerSummary{{ID: "c1"}},
			}, nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.ListContainers, http.MethodGet, "/containers")
	resp := doRequest(t, app, http.MethodGet, "/containers", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var body model.ContainerListResponse
	decodeJSON(t, resp, &body)
	if len(body.Containers) != 1 || body.Containers[0].ID != "c1" {
		t.Errorf("unexpected containers: %+v", body.Containers)
	}
}

func TestListContainers_AllQuery(t *testing.T) {
	var capturedAll bool
	mock := &mockContainerService{
		listFn: func(ctx context.Context, all bool) (model.ContainerListResponse, error) {
			capturedAll = all
			return model.ContainerListResponse{}, nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.ListContainers, http.MethodGet, "/containers")
	resp := doRequest(t, app, http.MethodGet, "/containers?all=true", "")
	resp.Body.Close()
	if !capturedAll {
		t.Error("expected all=true to be passed")
	}
}

func TestListContainers_ServiceError(t *testing.T) {
	mock := &mockContainerService{
		listFn: func(ctx context.Context, all bool) (model.ContainerListResponse, error) {
			return model.ContainerListResponse{}, errors.New("daemon error")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.ListContainers, http.MethodGet, "/containers")
	resp := doRequest(t, app, http.MethodGet, "/containers", "")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- InspectContainer tests ---

func TestInspectContainer_Success(t *testing.T) {
	mock := &mockContainerService{
		inspectFn: func(ctx context.Context, id string) (container.InspectResponse, error) {
			if id != "abc" {
				t.Errorf("expected id abc, got %s", id)
			}
			return container.InspectResponse{ID: "abc"}, nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.InspectContainer, http.MethodGet, "/containers/:id")
	resp := doRequest(t, app, http.MethodGet, "/containers/abc", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestInspectContainer_NotFound(t *testing.T) {
	mock := &mockContainerService{
		inspectFn: func(ctx context.Context, id string) (container.InspectResponse, error) {
			return container.InspectResponse{}, errors.New("No such container: xyz")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.InspectContainer, http.MethodGet, "/containers/:id")
	resp := doRequest(t, app, http.MethodGet, "/containers/xyz", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- StopContainer tests ---

func TestStopContainer_Success(t *testing.T) {
	mock := &mockContainerService{
		stopFn: func(ctx context.Context, id string, req model.StopContainerRequest) error {
			if id != "c1" {
				t.Errorf("expected id c1, got %s", id)
			}
			return nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.StopContainer, http.MethodPost, "/containers/:id/stop")
	resp := doRequest(t, app, http.MethodPost, "/containers/c1/stop", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var body model.MessageResponse
	decodeJSON(t, resp, &body)
	if body.Message != "container stopped" {
		t.Errorf("Message = %s", body.Message)
	}
}

func TestStopContainer_WithBody(t *testing.T) {
	var capturedTimeout *int
	mock := &mockContainerService{
		stopFn: func(ctx context.Context, id string, req model.StopContainerRequest) error {
			capturedTimeout = req.Timeout
			return nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.StopContainer, http.MethodPost, "/containers/:id/stop")
	resp := doRequest(t, app, http.MethodPost, "/containers/c1/stop", `{"timeout":30}`)
	resp.Body.Close()
	if capturedTimeout == nil || *capturedTimeout != 30 {
		t.Errorf("expected timeout=30, got %v", capturedTimeout)
	}
}

func TestStopContainer_NotFound(t *testing.T) {
	mock := &mockContainerService{
		stopFn: func(ctx context.Context, id string, req model.StopContainerRequest) error {
			return errors.New("No such container: bad")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.StopContainer, http.MethodPost, "/containers/:id/stop")
	resp := doRequest(t, app, http.MethodPost, "/containers/bad/stop", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- RemoveContainer tests ---

func TestRemoveContainer_Success(t *testing.T) {
	mock := &mockContainerService{
		removeFn: func(ctx context.Context, id string, q model.RemoveContainerQuery) error {
			return nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.RemoveContainer, http.MethodDelete, "/containers/:id")
	resp := doRequest(t, app, http.MethodDelete, "/containers/c1", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var body model.MessageResponse
	decodeJSON(t, resp, &body)
	if body.Message != "container removed" {
		t.Errorf("Message = %s", body.Message)
	}
}

func TestRemoveContainer_ForceQuery(t *testing.T) {
	var capturedQ model.RemoveContainerQuery
	mock := &mockContainerService{
		removeFn: func(ctx context.Context, id string, q model.RemoveContainerQuery) error {
			capturedQ = q
			return nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.RemoveContainer, http.MethodDelete, "/containers/:id")
	resp := doRequest(t, app, http.MethodDelete, "/containers/c1?force=true&v=true", "")
	resp.Body.Close()
	if !capturedQ.Force {
		t.Error("expected Force=true")
	}
	if !capturedQ.RemoveVolumes {
		t.Error("expected RemoveVolumes=true")
	}
}

func TestRemoveContainer_NotFound(t *testing.T) {
	mock := &mockContainerService{
		removeFn: func(ctx context.Context, id string, q model.RemoveContainerQuery) error {
			return errors.New("No such container: bad")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.RemoveContainer, http.MethodDelete, "/containers/:id")
	resp := doRequest(t, app, http.MethodDelete, "/containers/bad", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- GetContainerLogs tests ---

func TestGetContainerLogs_NonFollow(t *testing.T) {
	mock := &mockContainerService{
		logsFn: func(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("line1\nline2\n")), nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.GetContainerLogs, http.MethodGet, "/containers/:id/logs")
	resp := doRequest(t, app, http.MethodGet, "/containers/c1/logs", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Lines []string `json:"lines"`
	}
	decodeJSON(t, resp, &body)
	if len(body.Lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(body.Lines))
	}
	if body.Lines[0] != "line1" || body.Lines[1] != "line2" {
		t.Errorf("lines = %v", body.Lines)
	}
}

func TestGetContainerLogs_NonFollow_Empty(t *testing.T) {
	mock := &mockContainerService{
		logsFn: func(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader("")), nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.GetContainerLogs, http.MethodGet, "/containers/:id/logs")
	resp := doRequest(t, app, http.MethodGet, "/containers/c1/logs", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	var body struct {
		Lines []string `json:"lines"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	// lines should be nil or empty
	if body.Lines != nil && len(body.Lines) != 0 {
		t.Errorf("expected nil or empty lines, got %v", body.Lines)
	}
}

func TestGetContainerLogs_Follow_SSE(t *testing.T) {
	mock := &mockContainerService{
		logsFn: func(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error) {
			if !q.Follow {
				t.Error("expected Follow=true")
			}
			return io.NopCloser(strings.NewReader("log1\nlog2\n")), nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.GetContainerLogs, http.MethodGet, "/containers/:id/logs")
	resp := doRequest(t, app, http.MethodGet, "/containers/c1/logs?follow=true", "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	content := string(body)
	if !strings.Contains(content, "data: log1") {
		t.Errorf("expected 'data: log1' in SSE output, got: %s", content)
	}
	if !strings.Contains(content, "data: log2") {
		t.Errorf("expected 'data: log2' in SSE output, got: %s", content)
	}
}

func TestGetContainerLogs_ServiceError(t *testing.T) {
	mock := &mockContainerService{
		logsFn: func(ctx context.Context, id string, q model.LogsQuery) (io.ReadCloser, error) {
			return nil, errors.New("No such container: bad")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.GetContainerLogs, http.MethodGet, "/containers/:id/logs")
	resp := doRequest(t, app, http.MethodGet, "/containers/bad/logs", "")
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

// --- HealthCheck tests ---

func TestHealthCheck_OK(t *testing.T) {
	mock := &mockContainerService{
		pingFn: func(ctx context.Context) error {
			return nil
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.HealthCheck, http.MethodGet, "/health")
	resp := doRequest(t, app, http.MethodGet, "/health", "")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var body model.HealthResponse
	decodeJSON(t, resp, &body)
	if body.Status != "ok" {
		t.Errorf("Status = %s", body.Status)
	}
	if body.Docker != "connected" {
		t.Errorf("Docker = %s", body.Docker)
	}
}

func TestHealthCheck_Degraded(t *testing.T) {
	mock := &mockContainerService{
		pingFn: func(ctx context.Context) error {
			return errors.New("connection refused")
		},
	}
	h := NewContainerHandler(mock)
	app := newTestApp(h.HealthCheck, http.MethodGet, "/health")
	resp := doRequest(t, app, http.MethodGet, "/health", "")
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
	var body model.HealthResponse
	decodeJSON(t, resp, &body)
	if body.Status != "degraded" {
		t.Errorf("Status = %s", body.Status)
	}
	if body.Docker != "connection refused" {
		t.Errorf("Docker = %s", body.Docker)
	}
}
