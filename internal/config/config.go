package config

import (
	"log/slog"
	"os"
)

type Config struct {
	ListenAddr string
	APIKey     string
}

func Load() Config {
	cfg := Config{
		ListenAddr: envOrDefault("LISTEN_ADDR", ":3000"),
		APIKey:     os.Getenv("API_KEY"),
	}
	if cfg.APIKey == "" {
		slog.Error("API_KEY environment variable is required")
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
