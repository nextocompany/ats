// Package notify is the outbound-notification seam (re-engagement messages,
// scheduled report delivery). Mock (log-only) is the default so local/CI need no
// credentials; the real implementation (LINE push / email) is selected by config.
// This mirrors the peoplesoft/ai/line integration seams.
package notify

import (
	"context"

	"github.com/nexto/hr-ats/pkg/config"
)

// Channel identifies the delivery medium for a Message.
const (
	ChannelLINE  = "line"
	ChannelEmail = "email"
)

// Message is a single outbound notification.
type Message struct {
	Channel   string // ChannelLINE | ChannelEmail
	Recipient string // LINE user id or email address
	Subject   string
	Body      string
}

// Notifier sends outbound notifications.
type Notifier interface {
	Send(ctx context.Context, m Message) error
}

// NewNotifier selects the implementation by config (mock by default — no creds
// needed for local/CI).
func NewNotifier(cfg *config.Config) Notifier {
	if cfg.UsesRealNotify() {
		return newRESTNotifier(cfg)
	}
	return mockNotifier{}
}
