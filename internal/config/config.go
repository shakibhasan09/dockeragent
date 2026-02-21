package config

import (
	"fmt"
	"log/slog"
	"os"
)

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
