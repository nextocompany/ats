// Package email is the outbound transactional-email seam for candidate
// membership (passwordless email-OTP). It is mock by default — local/CI need no
// credentials — and uses Azure Communication Services (ACS) Email over REST when
// EMAIL_PROVIDER=real. ACS has no Go SDK, so acsSender signs requests with the
// access key (shared-key HMAC, the same scheme as Azure Storage).
package email

import (
	"context"

	"github.com/nexto/hr-ats/pkg/config"
)

// Message is a single transactional email. HTML is optional; PlainText is always
// sent so text-only clients render the OTP.
type Message struct {
	To        string
	Subject   string
	PlainText string
	HTML      string
}

// Sender delivers a Message. Implementations must return a non-nil error when the
// provider does not accept the request (the caller decides how to surface it).
type Sender interface {
	Send(ctx context.Context, m Message) error
}

// NewSender selects the sender by config: ACS (real) or mock (log-only default).
func NewSender(cfg *config.Config) Sender {
	if cfg.UsesRealEmail() {
		return newACSSender(cfg.ACSEmailEndpoint, cfg.ACSEmailAccessKey, cfg.ACSEmailSender)
	}
	return mockSender{logBody: cfg.IsDevelopment()}
}
