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
	ChannelTeams = "teams" // MS Teams Incoming Webhook (Recipient unused — webhook is the target)
)

// Message is a single outbound notification.
type Message struct {
	Channel   string // ChannelLINE | ChannelEmail | ChannelTeams
	Recipient string // LINE user id or email address (unused for Teams)
	Subject   string
	Body      string
	HTML      string // optional branded HTML body for ChannelEmail; ignored for LINE/Teams
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
