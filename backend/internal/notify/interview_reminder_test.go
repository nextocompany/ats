package notify

import (
	"strings"
	"testing"
	"time"
)

func TestInterviewReminderMessage_LINE(t *testing.T) {
	at := time.Date(2026, 6, 26, 14, 0, 0, 0, time.UTC)

	// Empty LINE handle → zero message (callers skip it).
	if m := InterviewReminderMessage("", "สมชาย", 1, 60, at, "onsite", "สาขา CM", "", "https://p", "tok"); m.Recipient != "" {
		t.Errorf("empty lineUserID should yield zero Message, got %+v", m)
	}

	m := InterviewReminderMessage("U123", "สมชาย", 1, 60, at, "online", "", "https://teams/x", "https://p", "tok")
	if m.Channel != ChannelLINE || m.Recipient != "U123" {
		t.Errorf("channel/recipient = %q/%q", m.Channel, m.Recipient)
	}
	if m.Body == "" {
		t.Fatal("empty body")
	}
	// Online appointment must surface the join link in the reminder body.
	if !strings.Contains(m.Body, "https://teams/x") {
		t.Errorf("online reminder body missing join link:\n%s", m.Body)
	}
	// Reminder copy must read as a reminder (distinct from the booking message).
	if !strings.Contains(m.Body, "เตือน") {
		t.Errorf("reminder body should signal a reminder:\n%s", m.Body)
	}
}

func TestInterviewReminderEmailMessage(t *testing.T) {
	at := time.Date(2026, 6, 26, 9, 30, 0, 0, time.UTC)

	if m := InterviewReminderEmailMessage("", "สมหญิง", 2, 45, at, "onsite", "สาขา", "", "https://p", "tok"); m.Recipient != "" {
		t.Errorf("empty email should yield zero Message, got %+v", m)
	}

	m := InterviewReminderEmailMessage("a@b.com", "สมหญิง", 2, 45, at, "onsite", "สาขา CM Central", "", "https://p", "tok")
	if m.Channel != ChannelEmail || m.Recipient != "a@b.com" {
		t.Errorf("channel/recipient = %q/%q", m.Channel, m.Recipient)
	}
	if m.HTML == "" {
		t.Error("email reminder should carry branded HTML")
	}
	// Onsite appointment must surface the location.
	if !strings.Contains(m.Body, "สาขา CM Central") {
		t.Errorf("onsite reminder body missing location:\n%s", m.Body)
	}
}
