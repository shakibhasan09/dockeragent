package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"

	"github.com/shakibhasan09/dockeragent/internal/config"
	"github.com/shakibhasan09/dockeragent/internal/model"
)

func newAuthTestApp(apiKey string) *fiber.App {
	app := fiber.New()
	cfg := config.Config{APIKey: apiKey}
	app.Use(NewAPIKeyAuth(cfg))
	app.Get("/protected", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func TestAPIKeyAuth_ValidKey(t *testing.T) {
	app := newAuthTestApp("mykey")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "mykey")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_InvalidKey(t *testing.T) {
	app := newAuthTestApp("mykey")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("X-API-Key", "wrongkey")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_MissingKey(t *testing.T) {
	app := newAuthTestApp("mykey")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAPIKeyAuth_ResponseFormat(t *testing.T) {
	app := newAuthTestApp("mykey")
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	var body model.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Error != "unauthorized" {
		t.Errorf("Error = %s", body.Error)
	}
	if body.Message != "missing or invalid API key" {
		t.Errorf("Message = %s", body.Message)
	}
	if body.Status != 401 {
		t.Errorf("Status = %d", body.Status)
	}
}
