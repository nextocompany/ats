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
)

const linePushURL = "https://api.line.me/v2/bot/message/push"

// restNotifier sends real notifications. LINE push is implemented over the
// Messaging API; email is a deploy-time SMTP wiring point. Constructed only when
// NOTIFY_PROVIDER=real, so missing creds never affect mock/CI.
type restNotifier struct {
	lineToken string
	emailFrom string
	http      *http.Client
}

func newRESTNotifier(cfg *config.Config) restNotifier {
	return restNotifier{
		lineToken: cfg.NotifyLINEToken,
		emailFrom: cfg.NotifyEmailFrom,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (n restNotifier) Send(ctx context.Context, m Message) error {
	switch m.Channel {
	case ChannelLINE:
		return n.sendLINE(ctx, m)
	case ChannelEmail:
		// SMTP/provider wiring is deploy-time; fail loudly rather than silently drop.
		return fmt.Errorf("notify: email channel not configured (NOTIFY_EMAIL_FROM=%q)", n.emailFrom)
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
