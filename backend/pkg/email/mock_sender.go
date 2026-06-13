package email

import (
	"context"

	"github.com/rs/zerolog/log"
)

// mockSender logs instead of delivering. In development it logs the plain-text
// body so the OTP is readable for local/E2E flows; outside development it logs
// only metadata (never the OTP) so a misconfigured prod never leaks codes.
type mockSender struct {
	logBody bool
}

func (m mockSender) Send(_ context.Context, msg Message) error {
	e := log.Info().Str("to", msg.To).Str("subject", msg.Subject).Str("provider", "mock")
	if m.logBody {
		e = e.Str("plain_text", msg.PlainText)
	}
	e.Msg("email: mock send")
	return nil
}
