package notify

import (
	"strings"
	"testing"
)

func TestDocumentReviewedMessage_Approved(t *testing.T) {
	m := DocumentReviewedMessage("U1", "สมชาย", "id_card", true, "", "http://portal")
	if m.Recipient != "U1" || m.Channel != ChannelLINE {
		t.Fatalf("unexpected envelope: %+v", m)
	}
	if !strings.Contains(m.Body, "อนุมัติ") || !strings.Contains(m.Body, "บัตรประชาชน") {
		t.Fatalf("approved body missing expected copy: %q", m.Body)
	}
}

func TestDocumentReviewedMessage_Rejected(t *testing.T) {
	m := DocumentReviewedEmailMessage("c@example.com", "", "education_certificate", false, "ภาพไม่ชัด", "http://portal")
	if m.Recipient != "c@example.com" || m.Channel != ChannelEmail {
		t.Fatalf("unexpected envelope: %+v", m)
	}
	if !strings.Contains(m.Body, "ตีกลับ") || !strings.Contains(m.Body, "ภาพไม่ชัด") {
		t.Fatalf("rejected body missing reason: %q", m.Body)
	}
}

func TestDocumentReviewedMessage_EmptyRecipientSkipped(t *testing.T) {
	if m := DocumentReviewedMessage("", "x", "id_card", true, "", "http://portal"); m.Recipient != "" {
		t.Fatalf("no LINE handle should yield a zero Message, got %+v", m)
	}
	if m := DocumentReviewedEmailMessage("", "x", "id_card", true, "", "http://portal"); m.Recipient != "" {
		t.Fatalf("no email should yield a zero Message, got %+v", m)
	}
}

func TestOnboardingDocUploadedHR(t *testing.T) {
	msgs := OnboardingDocUploadedHR([]string{"hr1@example.com", "hr2@example.com"}, true, "bank_book", "http://dash/applications/1")
	// 2 emails + 1 Teams
	if len(msgs) != 3 {
		t.Fatalf("expected 2 email + 1 teams = 3 messages, got %d", len(msgs))
	}
	teams := 0
	for _, m := range msgs {
		if m.Channel == ChannelTeams {
			teams++
		}
		if !strings.Contains(m.Body, "สมุดบัญชีธนาคาร") {
			t.Fatalf("body missing doc label: %q", m.Body)
		}
	}
	if teams != 1 {
		t.Fatalf("expected exactly one Teams message, got %d", teams)
	}
}

func TestOnboardingDocUploadedHR_NoTeams(t *testing.T) {
	msgs := OnboardingDocUploadedHR([]string{"hr@example.com"}, false, "photo", "http://dash")
	if len(msgs) != 1 || msgs[0].Channel != ChannelEmail {
		t.Fatalf("teams disabled should yield a single email, got %+v", msgs)
	}
}
