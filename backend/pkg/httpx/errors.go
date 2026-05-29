package httpx

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// ErrorHandler is the central Fiber error handler. Client errors (4xx) surface
// their message; server errors (5xx) are logged in full but masked in the
// response so internal details never leak to clients.
func ErrorHandler(c *fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	msg := "internal server error"

	var fe *fiber.Error
	if errors.As(err, &fe) {
		status = fe.Code
		if status < fiber.StatusInternalServerError {
			msg = fe.Message
		}
	}

	if status >= fiber.StatusInternalServerError {
		log.Error().
			Err(err).
			Int("status", status).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Msg("request failed")
	}

	return Fail(c, status, msg)
}
