package notify

import (
	"testing"

	"github.com/google/uuid"
)

// TestStatusPath_TokenFallback is the highest-risk path: an empty token (bulk /
// PeopleSoft / legacy rows that never went through portal apply) must yield a
// bare /status, never a broken "?token=".
func TestStatusPath_TokenFallback(t *testing.T) {
	if got := statusPath("https://p", ""); got != "https://p/status" {
		t.Errorf("empty token: got %q, want https://p/status", got)
	}
	if got := statusPath("https://p", "abc123"); got != "https://p/status?token=abc123" {
		t.Errorf("token: got %q, want …?token=abc123", got)
	}
}

// TestStatusPath_EncodesToken proves the base64 token (which can contain / + =)
// is query-escaped so the link is valid.
func TestStatusPath_EncodesToken(t *testing.T) {
	got := statusPath("https://p", "a/b+c=")
	if want := "https://p/status?token=a%2Fb%2Bc%3D"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestStatusURL_Routing covers the per-status destinations: offer deep-links to
// the authed /offers/<appID>, hired to the authed onboarding /account, and every
// status view to the token status page.
func TestStatusURL_Routing(t *testing.T) {
	appID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	cases := []struct {
		status, token, want string
	}{
		{"offer", "tok", "https://p/offers/11111111-1111-1111-1111-111111111111"},
		{"hired", "tok", "https://p/account"},
		{"scored", "tok", "https://p/status?token=tok"},
		{"rejected", "", "https://p/status"},
		{"interview", "tok", "https://p/status?token=tok"},
	}
	for _, c := range cases {
		if got := statusURL("https://p", c.status, c.token, appID); got != c.want {
			t.Errorf("statusURL(%q, token=%q) = %q, want %q", c.status, c.token, got, c.want)
		}
	}
}

// TestStatusMessage_ScoredCarriesToken proves a notifiable status with a token
// produces a deep link the candidate can tap straight through.
func TestStatusMessage_ScoredCarriesToken(t *testing.T) {
	m := StatusMessage("U-1", "สมชาย", "scored", "https://p", "tok-xyz", uuid.New())
	if m.Recipient == "" {
		t.Fatal("scored should notify")
	}
	if want := "https://p/status?token=tok-xyz"; !contains(m.Body, want) {
		t.Errorf("body missing deep link %q: %q", want, m.Body)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
