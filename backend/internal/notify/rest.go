package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nexto/hr-ats/pkg/config"
	"github.com/nexto/hr-ats/pkg/email"
	"github.com/nexto/hr-ats/pkg/emailtmpl"
)

const linePushURL = "https://api.line.me/v2/bot/message/push"

// restNotifier sends real notifications: LINE push over the Messaging API, email
// via the shared ACS sender (pkg/email — mock when EMAIL_PROVIDER!=real), and MS
// Teams via an Incoming Webhook. Constructed only when NOTIFY_PROVIDER=real, so
// missing creds never affect mock/CI.
type restNotifier struct {
	lineToken    string
	email        email.Sender
	teamsWebhook string
	http         *http.Client
}

func newRESTNotifier(cfg *config.Config) restNotifier {
	return restNotifier{
		lineToken:    cfg.NotifyLINEToken,
		email:        email.NewSender(cfg), // ACS (real) or mock log-only — safe in CI
		teamsWebhook: cfg.TeamsWebhookURL,
		http:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (n restNotifier) Send(ctx context.Context, m Message) error {
	switch m.Channel {
	case ChannelLINE:
		return n.sendLINE(ctx, m)
	case ChannelEmail:
		// Always send a branded HTML part alongside the plain body (multipart). A
		// builder may supply its own HTML; otherwise wrap the plain body in the
		// branded shell so even un-upgraded emails are on-brand.
		html := m.HTML
		if html == "" {
			html = emailtmpl.RenderPlain(m.Body)
		}
		return n.email.Send(ctx, email.Message{To: m.Recipient, Subject: m.Subject, PlainText: m.Body, HTML: html})
	case ChannelTeams:
		if n.teamsWebhook == "" {
			return fmt.Errorf("notify: teams webhook not configured (TEAMS_WEBHOOK_URL empty)")
		}
		return sendTeams(ctx, n.http, n.teamsWebhook, m.Subject, m.Body)
	default:
		return fmt.Errorf("notify: unknown channel %q", m.Channel)
	}
}

func (n restNotifier) sendLINE(ctx context.Context, m Message) error {
	payload := map[string]any{
		"to":       m.Recipient,
		"messages": []map[string]string{{"type": "text", "text": m.Body}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: marshal line push: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linePushURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: build line request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.lineToken)

	resp, err := n.http.Do(req)
	if err != nil {
		return fmt.Errorf("notify: line push: %w", err)
	}
	defer func() {
		// Drain so the connection can return to the pool (matters under retries).
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: line push status %d", resp.StatusCode)
	}
	return nil
}
