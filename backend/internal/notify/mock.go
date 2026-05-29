package notify

import (
	"context"

	"github.com/rs/zerolog/log"
)

// mockNotifier records the send (no external call) so local/CI exercise the full
// re-engagement / report-delivery flow without LINE or email credentials.
type mockNotifier struct{}

func (mockNotifier) Send(_ context.Context, m Message) error {
	log.Info().
		Str("channel", m.Channel).
		Str("to", m.Recipient).
		Str("subject", m.Subject).
		Msg("[mock notify] send")
	return nil
}
