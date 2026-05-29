// Package middleware holds cross-cutting HTTP middleware (request logging,
// dev auth, and — in Sprint 1+ — RBAC and rate limiting).
package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// RequestIDHeader is the header used to propagate a request id.
const RequestIDHeader = "X-Request-ID"

// RequestLogger assigns a request id (reusing an inbound one when present) and
// logs each request's method, path, status, and latency.
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		reqID := c.Get(RequestIDHeader)
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Locals("request_id", reqID)
		c.Set(RequestIDHeader, reqID)

		err := c.Next()

		log.Info().
			Str("request_id", reqID).
			Str("method", c.Method()).
			Str("path", c.Path()).
			Int("status", c.Response().StatusCode()).
			Dur("latency", time.Since(start)).
			Msg("request")

		return err
	}
}
