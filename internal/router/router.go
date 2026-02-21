package router

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"github.com/shakibhasan09/dockeragent/internal/config"
	"github.com/shakibhasan09/dockeragent/internal/handler"
	"github.com/shakibhasan09/dockeragent/internal/middleware"
	"github.com/shakibhasan09/dockeragent/internal/model"
)

func ErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := "internal server error"

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		msg = e.Message
	}

	slog.Error("request error",
		"status", code,
		"error", err.Error(),
		"method", c.Method(),
		"path", c.Path(),
	)

	return c.Status(code).JSON(model.ErrorResponse{
		Error:   http.StatusText(code),
		Message: msg,
		Status:  code,
	})
}

func Setup(app *fiber.App, h *handler.ContainerHandler, cfg config.Config) {
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(requestLogger())

	app.Get("/health", h.HealthCheck)

	api := app.Group("/api/v1", middleware.NewAPIKeyAuth(cfg))

	containers := api.Group("/containers")
	containers.Post("/", h.CreateContainer)
	containers.Get("/", h.ListContainers)
	containers.Get("/:id", h.InspectContainer)
	containers.Post("/:id/stop", h.StopContainer)
	containers.Delete("/:id", h.RemoveContainer)
	containers.Get("/:id/logs", h.GetContainerLogs)
}

func requestLogger() fiber.Handler {
	return func(c fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		slog.Info("request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency_ms", time.Since(start).Milliseconds(),
			"request_id", c.Locals("requestid"),
		)
		return err
	}
}
