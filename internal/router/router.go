package router

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
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

func Setup(app *fiber.App, ch *handler.ContainerHandler, fh *handler.FileHandler, cfg config.Config) {
	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(requestLogger())
	app.Use(limiter.New(limiter.Config{
		Max:        100,
		Expiration: 1 * time.Minute,
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(model.ErrorResponse{
				Error:   http.StatusText(fiber.StatusTooManyRequests),
				Message: "rate limit exceeded, try again later",
				Status:  fiber.StatusTooManyRequests,
			})
		},
	}))

	app.Get("/health", ch.HealthCheck)

	api := app.Group("/api/v1", middleware.NewAPIKeyAuth(cfg))

	containers := api.Group("/containers")
	containers.Post("/", ch.CreateContainer)
	containers.Get("/", ch.ListContainers)
	containers.Get("/:id", ch.InspectContainer)
	containers.Post("/:id/stop", ch.StopContainer)
	containers.Delete("/:id", ch.RemoveContainer)
	containers.Get("/:id/logs", ch.GetContainerLogs)

	files := api.Group("/files")
	files.Post("/", fh.WriteFile)
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
