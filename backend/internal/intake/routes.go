package intake

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

const (
	signatureHeader = "X-Intake-Signature"
	timestampHeader = "X-Intake-Timestamp" // Unix seconds, bound into the signature
	maxClockSkew    = 5 * time.Minute      // replay window
)

// RegisterRoutes mounts the intake webhook under /api/v1/intake. The endpoint is
// secret-gated: with no secret it is NOT mounted at all (intake disabled, secure by
// default), since an unauthenticated endpoint that creates applications is an abuse
// vector. With a secret, every request must carry a valid HMAC signature.
func RegisterRoutes(app *fiber.App, h *Handler, secret string) {
	if secret == "" {
		log.Info().Msg("intake webhook disabled (INTAKE_WEBHOOK_SECRET unset)")
		return
	}
	grp := app.Group("/api/v1/intake")
	grp.Use(verifyHMAC(secret))
	grp.Post("/:source", h.Submit)
}

// verifyHMAC authenticates an inbound intake request. The signer sends a Unix-
// seconds X-Intake-Timestamp and X-Intake-Signature = hex(HMAC-SHA256(secret,
// "<timestamp>\n<rawBody>")). Binding the timestamp into the MAC means it cannot be
// stripped, and rejecting stale timestamps (±5min) blocks replay of a captured
// request. Compared in constant time; reading c.Body() does not consume it.
func verifyHMAC(secret string) fiber.Handler {
	key := []byte(secret)
	return func(c *fiber.Ctx) error {
		ts := c.Get(timestampHeader)
		got := c.Get(signatureHeader)
		if ts == "" || got == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing webhook timestamp or signature")
		}
		secs, err := strconv.ParseInt(ts, 10, 64)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid webhook timestamp")
		}
		if skew := time.Since(time.Unix(secs, 0)); skew > maxClockSkew || skew < -maxClockSkew {
			log.Warn().Str("path", c.Path()).Msg("intake: webhook timestamp outside replay window")
			return fiber.NewError(fiber.StatusUnauthorized, "webhook timestamp expired")
		}
		gotMAC, err := hex.DecodeString(got)
		if err != nil {
			log.Warn().Str("path", c.Path()).Msg("intake: webhook signature not valid hex")
			return fiber.NewError(fiber.StatusUnauthorized, "invalid webhook signature")
		}
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(ts))
		mac.Write([]byte{'\n'})
		mac.Write(c.Body())
		if !hmac.Equal(mac.Sum(nil), gotMAC) {
			log.Warn().Str("path", c.Path()).Msg("intake: webhook signature mismatch")
			return fiber.NewError(fiber.StatusUnauthorized, "invalid webhook signature")
		}
		return c.Next()
	}
}
