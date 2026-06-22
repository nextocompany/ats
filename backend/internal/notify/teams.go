package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// teamsBrandHeader is shown at the top of every card so the source is
// unmistakable. The channel "sender" identity is fixed by Power Automate (the
// Workflows app) and cannot be set from the card payload, so we brand the card
// body instead.
const teamsBrandHeader = "HR ATS System"

// sendTeams posts an Adaptive Card to an MS Teams channel webhook. The payload is
// the {"type":"message","attachments":[adaptive card]} shape that a Power Automate
// "Workflows" flow posts to a channel natively (Microsoft's supported replacement
// for the retired O365 Incoming Webhook connector, which accepted plain
// {"text":...}). A brand header leads, then title renders as a bold heading; text
// is split into one TextBlock per line so multi-field HR notifications keep their
// layout. Workflows replies 202 Accepted on success, so any 2xx is success.
func sendTeams(ctx context.Context, hc *http.Client, webhookURL, title, text string) error {
	blocks := make([]map[string]any, 0, 8)
	blocks = append(blocks, map[string]any{
		"type":    "TextBlock",
		"text":    teamsBrandHeader,
		"weight":  "Bolder",
		"color":   "Accent",
		"size":    "Small",
		"spacing": "None",
		"wrap":    true,
	})
	if strings.TrimSpace(title) != "" {
		blocks = append(blocks, map[string]any{
			"type":   "TextBlock",
			"text":   title,
			"weight": "Bolder",
			"size":   "Medium",
			"wrap":   true,
		})
	}
	// One TextBlock per non-empty line: a TextBlock does not render "\n" as a
	// break, so the body's per-field lines must become separate blocks.
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		blocks = append(blocks, map[string]any{
			"type":    "TextBlock",
			"text":    line,
			"wrap":    true,
			"spacing": "Small",
		})
	}

	payload := map[string]any{
		"type": "message",
		"attachments": []map[string]any{{
			"contentType": "application/vnd.microsoft.card.adaptive",
			"content": map[string]any{
				"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
				"type":    "AdaptiveCard",
				"version": "1.4",
				"body":    blocks,
			},
		}},
	}
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
