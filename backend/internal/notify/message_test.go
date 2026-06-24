package notify

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestInvalidResumeMessage(t *testing.T) {
	// No handle → skipped (empty Recipient), so bulk uploads no-op.
	if m := InvalidResumeMessage("", "สมชาย", "https://x", ""); m.Recipient != "" {
		t.Errorf("no LINE handle should skip, got %q", m.Recipient)
	}
	if m := InvalidResumeEmailMessage("", "สมชาย", "https://x", ""); m.Recipient != "" {
		t.Errorf("no email should skip, got %q", m.Recipient)
	}
	// With a handle → gentle, recoverable copy pointing to re-upload.
	m := InvalidResumeMessage("U-1", "สมชาย", "https://careers.example.com", "")
	if m.Recipient != "U-1" || m.Channel != ChannelLINE {
		t.Fatalf("unexpected message: %+v", m)
	}
	if !strings.Contains(m.Body, "อาจไม่ใช่เรซูเม่") {
		t.Errorf("body should be gentle (อาจไม่ใช่), got %q", m.Body)
	}
	if !strings.Contains(m.Body, "/status") {
		t.Errorf("body should link to /status, got %q", m.Body)
	}
	em := InvalidResumeEmailMessage("a@b.com", "สมชาย", "https://careers.example.com", "")
	if em.Channel != ChannelEmail || em.Body != m.Body {
		t.Errorf("email body must match LINE body (no drift): %q vs %q", em.Body, m.Body)
	}
}

func TestStatusMessage_Notifiable(t *testing.T) {
	// hired/offer carry their own deep links (/account, /offers) — asserted
	// separately; these two link to /status.
	for _, status := range []string{"shortlisted", "interview"} {
		m := StatusMessage("U-line-123", "สมชาย", status, "https://careers.example.com", "", uuid.Nil)
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
	// Internal-only states with no candidate-facing copy → empty Message.
	// (scored/rejected/ai_interview/... ARE notifiable as of item 3.)
	for _, status := range []string{"pending", "parsed", "failed", "weird"} {
		m := StatusMessage("U-line-123", "สมชาย", status, "https://x", "", uuid.Nil)
		if m.Recipient != "" {
			t.Errorf("status %q should not notify, got recipient %q", status, m.Recipient)
		}
	}
}

func TestStatusMessage_HiredDirectsToOnboardingUpload(t *testing.T) {
	m := StatusMessage("U-1", "สมชาย", "hired", "https://careers.example.com", "", uuid.Nil)
	if !strings.Contains(m.Body, "https://careers.example.com/account") {
		t.Errorf("hired copy must direct the candidate to /account to upload docs: %q", m.Body)
	}
}

func TestStatusMessage_NoLineHandle(t *testing.T) {
	m := StatusMessage("", "สมชาย", "hired", "https://x", "", uuid.Nil)
	if m.Recipient != "" {
		t.Errorf("no LINE id should yield empty message, got %+v", m)
	}
}

func TestStatusMessage_NoNameStillNotifies(t *testing.T) {
	m := StatusMessage("U-1", "", "hired", "https://x", "", uuid.Nil)
	if m.Recipient == "" || !strings.Contains(m.Body, "สวัสดี") {
		t.Errorf("missing name should still produce a greeting body: %+v", m)
	}
}
