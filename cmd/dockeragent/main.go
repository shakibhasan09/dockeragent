package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/shakibhasan09/dockeragent/internal/config"
	"github.com/shakibhasan09/dockeragent/internal/docker"
	"github.com/shakibhasan09/dockeragent/internal/handler"
	"github.com/shakibhasan09/dockeragent/internal/router"
	"github.com/shakibhasan09/dockeragent/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()

	dockerClient, err := docker.NewClient()
	if err != nil {
		slog.Error("failed to create docker client", "error", err)
		os.Exit(1)
	}

	containerSvc := service.NewContainerService(dockerClient)
	containerHandler := handler.NewContainerHandler(containerSvc)

	app := fiber.New(fiber.Config{
		ErrorHandler: router.ErrorHandler,
	})

	router.Setup(app, containerHandler, cfg)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app.Hooks().OnPreShutdown(func() error {
		slog.Info("shutting down, closing docker client")
		return dockerClient.Close()
	})

	slog.Info("starting dockeragent", "addr", cfg.ListenAddr)

	if err := app.Listen(cfg.ListenAddr, fiber.ListenConfig{
		GracefulContext: ctx,
		ShutdownTimeout: 10 * time.Second,
	}); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
