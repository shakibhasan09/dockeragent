package config

import (
	"testing"
)

func TestLoadWithError_Success(t *testing.T) {
	t.Setenv("API_KEY", "test-key")
	t.Setenv("LISTEN_ADDR", "")

	cfg, err := LoadWithError()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %s, want test-key", cfg.APIKey)
	}
	if cfg.ListenAddr != ":3000" {
		t.Errorf("ListenAddr = %s, want :3000", cfg.ListenAddr)
	}
}

func TestLoadWithError_CustomListenAddr(t *testing.T) {
	t.Setenv("API_KEY", "key")
	t.Setenv("LISTEN_ADDR", ":8080")

	cfg, err := LoadWithError()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %s, want :8080", cfg.ListenAddr)
	}
}

func TestLoadWithError_MissingAPIKey(t *testing.T) {
	t.Setenv("API_KEY", "")

	_, err := LoadWithError()
	if err == nil {
		t.Fatal("expected error for missing API_KEY")
	}
	if err.Error() != "API_KEY environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}
