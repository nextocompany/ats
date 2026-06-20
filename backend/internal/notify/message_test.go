package notify

import (
	"strings"
	"testing"
)

func TestStatusMessage_Notifiable(t *testing.T) {
	// hired/offer carry their own deep links (/account, /offers) — asserted
	// separately; these two link to /status.
	for _, status := range []string{"shortlisted", "interview"} {
		m := StatusMessage("U-line-123", "สมชาย", status, "https://careers.example.com")
		if m.Channel != ChannelLINE {
			t.Errorf("%s: channel = %q, want %q", status, m.Channel, ChannelLINE)
		}
		if m.Recipient != "U-line-123" {
			t.Errorf("%s: recipient = %q, want LINE id", status, m.Recipient)
		}
		if m.Subject == "" || m.Body == "" {
			t.Errorf("%s: empty subject/body", status)
		}
		if !strings.Contains(m.Body, "สมชาย") {
			t.Errorf("%s: body missing candidate name: %q", status, m.Body)
		}
		if !strings.Contains(m.Body, "https://careers.example.com/status") {
			t.Errorf("%s: body missing status link: %q", status, m.Body)
		}
	}
}

func TestStatusMessage_NotNotifiableStatus(t *testing.T) {
	// Statuses we deliberately do NOT push (incl. rejected) → empty Message.
	for _, status := range []string{"pending", "parsed", "scored", "rejected", "weird"} {
		m := StatusMessage("U-line-123", "สมชาย", status, "https://x")
		if m.Recipient != "" {
			t.Errorf("status %q should not notify, got recipient %q", status, m.Recipient)
		}
	}
}

func TestStatusMessage_HiredDirectsToOnboardingUpload(t *testing.T) {
	m := StatusMessage("U-1", "สมชาย", "hired", "https://careers.example.com")
	if !strings.Contains(m.Body, "https://careers.example.com/account") {
		t.Errorf("hired copy must direct the candidate to /account to upload docs: %q", m.Body)
	}
}

func TestStatusMessage_NoLineHandle(t *testing.T) {
	m := StatusMessage("", "สมชาย", "hired", "https://x")
	if m.Recipient != "" {
		t.Errorf("no LINE id should yield empty message, got %+v", m)
	}
}

func TestStatusMessage_NoNameStillNotifies(t *testing.T) {
	m := StatusMessage("U-1", "", "hired", "https://x")
	if m.Recipient == "" || !strings.Contains(m.Body, "สวัสดี") {
		t.Errorf("missing name should still produce a greeting body: %+v", m)
	}
}
