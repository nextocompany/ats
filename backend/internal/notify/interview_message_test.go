package notify

import (
	"strings"
	"testing"
	"time"
)

func TestInterviewScheduledMessage_OnlineRound1(t *testing.T) {
	// 07:00 UTC must render as 14:00 Asia/Bangkok.
	when := time.Date(2026, 6, 25, 7, 0, 0, 0, time.UTC)
	m := InterviewScheduledMessage("U1", "สมชาย", 1, 60, when, "online", "", "https://teams.example/join", "https://careers.example.com")
	if m.Recipient != "U1" || m.Channel != ChannelLINE {
		t.Fatalf("unexpected envelope: %+v", m)
	}
	for _, want := range []string{"สวัสดีคุณสมชาย", "25 มิถุนายน 2569", "14:00", "https://teams.example/join", "https://careers.example.com/status"} {
		if !strings.Contains(m.Body, want) {
			t.Fatalf("body missing %q: %q", want, m.Body)
		}
	}
	if strings.Contains(m.Body, "รอบ") {
		t.Fatalf("round 1 must not carry a round label: %q", m.Body)
	}
}

func TestInterviewScheduledMessage_OnsiteRound2(t *testing.T) {
	when := time.Date(2026, 7, 1, 3, 30, 0, 0, time.UTC) // 10:30 ICT
	m := InterviewScheduledMessage("U1", "", 2, 0, when, "onsite", "สาขาลาดพร้าว ชั้น 3", "", "https://p")
	for _, want := range []string{"รอบ 2", "สาขาลาดพร้าว ชั้น 3", "10:30", "สวัสดีค่ะ"} {
		if !strings.Contains(m.Body, want) {
			t.Fatalf("body missing %q: %q", want, m.Body)
		}
	}
	if strings.Contains(m.Body, "เข้าร่วมที่") {
		t.Fatalf("onsite must not carry an online join clause: %q", m.Body)
	}
}

func TestInterviewScheduledMessage_OnlineNoJoinURL(t *testing.T) {
	// Graph mock failed → empty join URL: the message must still send, sans link.
	when := time.Date(2026, 6, 25, 7, 0, 0, 0, time.UTC)
	m := InterviewScheduledMessage("U1", "x", 1, 60, when, "online", "", "", "https://p")
	if m.Recipient == "" {
		t.Fatal("missing join URL must not suppress the message")
	}
	if !strings.Contains(m.Body, "ออนไลน์") || strings.Contains(m.Body, "เข้าร่วมที่") {
		t.Fatalf("expected online label without a join clause: %q", m.Body)
	}
}

func TestInterviewScheduledMessage_EmailTwin(t *testing.T) {
	when := time.Date(2026, 6, 25, 7, 0, 0, 0, time.UTC)
	m := InterviewScheduledEmailMessage("c@example.com", "สมชาย", 1, 60, when, "online", "", "https://j", "https://p")
	if m.Recipient != "c@example.com" || m.Channel != ChannelEmail {
		t.Fatalf("unexpected envelope: %+v", m)
	}
	if !strings.Contains(m.Body, "14:00") {
		t.Fatalf("email body missing time: %q", m.Body)
	}
}

func TestInterviewScheduledMessage_EmptyRecipientSkipped(t *testing.T) {
	when := time.Date(2026, 6, 25, 7, 0, 0, 0, time.UTC)
	if m := InterviewScheduledMessage("", "x", 1, 60, when, "online", "", "https://j", "https://p"); m.Recipient != "" {
		t.Fatalf("no LINE handle should yield a zero Message, got %+v", m)
	}
	if m := InterviewScheduledEmailMessage("", "x", 1, 60, when, "online", "", "https://j", "https://p"); m.Recipient != "" {
		t.Fatalf("no email should yield a zero Message, got %+v", m)
	}
}
