package peoplesoft

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

// signatureHeader carries the hex HMAC-SHA256 of the raw request body that
// PeopleSoft computes with the shared PS_WEBHOOK_SECRET.
const signatureHeader = "X-PS-Signature"

// VerifyHMAC returns middleware that authenticates inbound PeopleSoft webhooks by
// verifying that signatureHeader equals hex(HMAC-SHA256(secret, rawBody)) using a
// constant-time compare. It is mounted only on the state-changing POST routes;
// GET /health stays open. Reading c.Body() does not consume the buffered body, so
// the downstream handlers' BodyParser still works.
func VerifyHMAC(secret string) fiber.Handler {
	key := []byte(secret)
	return func(c *fiber.Ctx) error {
		got := c.Get(signatureHeader)
		if got == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing webhook signature")
		}
		gotMAC, err := hex.DecodeString(got)
		if err != nil {
			// Not valid hex — cannot be a correct signature. Reject without leaking detail.
			log.Warn().Str("path", c.Path()).Msg("peoplesoft: webhook signature not valid hex")
			return fiber.NewError(fiber.StatusUnauthorized, "invalid webhook signature")
		}
		mac := hmac.New(sha256.New, key)
		mac.Write(c.Body())
		want := mac.Sum(nil)
		// Compare raw MAC bytes (constant-time): accepts upper/lower-case hex from the signer.
		if !hmac.Equal(want, gotMAC) {
			log.Warn().Str("path", c.Path()).Msg("peoplesoft: webhook signature mismatch")
			return fiber.NewError(fiber.StatusUnauthorized, "invalid webhook signature")
		}
		return c.Next()
	}
}
