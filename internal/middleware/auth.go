package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/extractors"
	"github.com/gofiber/fiber/v3/middleware/keyauth"

	"github.com/shakibhasan09/dockeragent/internal/config"
	"github.com/shakibhasan09/dockeragent/internal/model"
)

func NewAPIKeyAuth(cfg config.Config) fiber.Handler {
	expected := []byte(cfg.APIKey)
	return keyauth.New(keyauth.Config{
		Extractor: extractors.FromHeader("X-API-Key"),
		Validator: func(c fiber.Ctx, key string) (bool, error) {
			if subtle.ConstantTimeCompare([]byte(key), expected) == 1 {
				return true, nil
			}
			return false, keyauth.ErrMissingOrMalformedAPIKey
		},
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(fiber.StatusUnauthorized).JSON(model.ErrorResponse{
				Error:   "unauthorized",
				Message: "missing or invalid API key",
				Status:  fiber.StatusUnauthorized,
			})
		},
	})
}
