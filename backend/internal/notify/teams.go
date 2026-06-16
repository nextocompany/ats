package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// sendTeams posts a plain-text message to an MS Teams Incoming Webhook. The
// webhook URL is the secret (no auth header). Mirrors the LINE push HTTP pattern
// in rest.go. Teams replies 200 with body "1" on success — any 2xx is success.
func sendTeams(ctx context.Context, hc *http.Client, webhookURL, text string) error {
	payload := map[string]string{"text": text}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("notify: marshal teams card: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("notify: build teams request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("notify: teams post: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("notify: teams post status %d", resp.StatusCode)
	}
	return nil
}
