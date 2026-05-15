package config

import (
	"fmt"
	"log/slog"
	"os"
)

// MinAPIKeyLength is the minimum allowed API key length. Anything shorter
// is rejected at startup because the API grants effectively unrestricted
// access to the host's Docker daemon and filesystem, so a weak key is a
// production hazard.
const MinAPIKeyLength = 32

type Config struct {
	ListenAddr string
	APIKey     string
}

func LoadWithError() (Config, error) {
	cfg := Config{
		ListenAddr: envOrDefault("LISTEN_ADDR", ":3000"),
		APIKey:     os.Getenv("API_KEY"),
	}
	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("API_KEY environment variable is required")
	}
	if len(cfg.APIKey) < MinAPIKeyLength {
		return Config{}, fmt.Errorf("API_KEY must be at least %d characters", MinAPIKeyLength)
	}
	return cfg, nil
}

func Load() Config {
	cfg, err := LoadWithError()
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	return cfg
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
