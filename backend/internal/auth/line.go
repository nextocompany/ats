// Package auth provides external-candidate authentication. Sprint 3 adds LINE
// Login verification behind a mock-default seam (real verification opt-in).
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nexto/hr-ats/pkg/config"
)

// LineUser is the minimal identity returned by LINE verification.
type LineUser struct {
	Subject string `json:"sub"`
	Name    string `json:"name"`
	Email   string `json:"email"`
}

// Verifier verifies a LINE id token and returns the candidate identity.
type Verifier interface {
	Verify(ctx context.Context, idToken string) (LineUser, error)
}

// NewVerifier selects the LINE verifier by config (mock by default).
func NewVerifier(cfg *config.Config) Verifier {
	if cfg.UsesRealLINE() {
		return realVerifier{channelID: cfg.LINEChannelID, http: &http.Client{Timeout: 10 * time.Second}}
	}
	return mockVerifier{}
}

// mockVerifier accepts any non-empty token and returns a deterministic user.
type mockVerifier struct{}

func (mockVerifier) Verify(_ context.Context, idToken string) (LineUser, error) {
	if strings.TrimSpace(idToken) == "" {
		return LineUser{}, fmt.Errorf("auth: missing LINE id token")
	}
	return LineUser{Subject: "U-dev-" + idToken, Name: "Dev Candidate", Email: ""}, nil
}

// realVerifier verifies the id token against LINE's endpoint.
type realVerifier struct {
	channelID string
	http      *http.Client
}

func (v realVerifier) Verify(ctx context.Context, idToken string) (LineUser, error) {
	if idToken == "" {
		return LineUser{}, fmt.Errorf("auth: missing LINE id token")
	}
	form := url.Values{"id_token": {idToken}, "client_id": {v.channelID}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.line.me/oauth2/v2.1/verify", strings.NewReader(form.Encode()))
	if err != nil {
		return LineUser{}, fmt.Errorf("auth: line request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := v.http.Do(req)
	if err != nil {
		return LineUser{}, fmt.Errorf("auth: line verify: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return LineUser{}, fmt.Errorf("auth: line verify status %d", resp.StatusCode)
	}
	var u LineUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return LineUser{}, fmt.Errorf("auth: line decode: %w", err)
	}
	return u, nil
}
