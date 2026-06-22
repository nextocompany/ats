package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nexto/hr-ats/pkg/email"
)

func TestSendTeams_PostsAdaptiveCard(t *testing.T) {
	var got struct {
		Type        string `json:"type"`
		Attachments []struct {
			ContentType string `json:"contentType"`
			Content     struct {
				Type string `json:"type"`
				Body []struct {
					Text   string `json:"text"`
					Weight string `json:"weight"`
				} `json:"body"`
			} `json:"content"`
		} `json:"attachments"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusAccepted) // Workflows replies 202
	}))
	defer srv.Close()

	if err := sendTeams(context.Background(), srv.Client(), srv.URL, "หัวข้อ", "บรรทัด 1\nบรรทัด 2"); err != nil {
		t.Fatalf("sendTeams err: %v", err)
	}
	if got.Type != "message" {
		t.Fatalf("payload type=%q, want message", got.Type)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].ContentType != "application/vnd.microsoft.card.adaptive" {
		t.Fatalf("attachment not an adaptive card: %+v", got.Attachments)
	}
	body := got.Attachments[0].Content.Body
	if got.Attachments[0].Content.Type != "AdaptiveCard" || len(body) != 3 {
		t.Fatalf("want AdaptiveCard with 3 blocks (title + 2 lines), got type=%q blocks=%d",
			got.Attachments[0].Content.Type, len(body))
	}
	if body[0].Text != "หัวข้อ" || body[0].Weight != "Bolder" {
		t.Fatalf("first block should be the bold title, got %+v", body[0])
	}
	if body[1].Text != "บรรทัด 1" || body[2].Text != "บรรทัด 2" {
		t.Fatalf("body lines not split per TextBlock: %+v", body)
	}
}

func TestSendTeams_Non2xxIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if err := sendTeams(context.Background(), srv.Client(), srv.URL, "subj", "x"); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

// fakeSender records the last email.Message it received.
type fakeSender struct{ last email.Message }

func (f *fakeSender) Send(_ context.Context, m email.Message) error { f.last = m; return nil }

func TestRestNotifier_EmailMapsFields(t *testing.T) {
	fs := &fakeSender{}
	n := restNotifier{email: fs, http: &http.Client{Timeout: time.Second}}
	err := n.Send(context.Background(), Message{
		Channel: ChannelEmail, Recipient: "a@b.com", Subject: "subj", Body: "body",
	})
	if err != nil {
		t.Fatalf("send email err: %v", err)
	}
	if fs.last.To != "a@b.com" || fs.last.Subject != "subj" || fs.last.PlainText != "body" {
		t.Fatalf("email mapping wrong: %+v", fs.last)
	}
}

func TestRestNotifier_TeamsRequiresWebhook(t *testing.T) {
	n := restNotifier{http: &http.Client{Timeout: time.Second}} // empty teamsWebhook
	if err := n.Send(context.Background(), Message{Channel: ChannelTeams, Body: "x"}); err == nil {
		t.Fatal("expected error when teams webhook unconfigured")
	}
}

func TestStatusEmailMessage_Gating(t *testing.T) {
	tests := []struct {
		name, addr, status string
		wantRecipient      bool
	}{
		{"notifiable + addr", "a@b.com", "interview", true},
		{"shortlisted", "a@b.com", "shortlisted", true},
		{"empty addr", "", "interview", false},
		{"rejected not notifiable", "a@b.com", "rejected", false},
		{"unknown status", "a@b.com", "scored", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := StatusEmailMessage(tc.addr, "Somchai", tc.status, "https://portal")
			if (m.Recipient != "") != tc.wantRecipient {
				t.Fatalf("recipient=%q, wantRecipient=%v", m.Recipient, tc.wantRecipient)
			}
			if m.Recipient != "" && m.Channel != ChannelEmail {
				t.Fatalf("channel=%q, want email", m.Channel)
			}
		})
	}
}

func TestNewScoredHR(t *testing.T) {
	msgs := NewScoredHR([]string{"hr1@x.com", "hr2@x.com"}, true, "สมชาย", "พนักงานขาย", 82, "https://dash/applications/1")
	emails, teams := 0, 0
	for _, m := range msgs {
		switch m.Channel {
		case ChannelEmail:
			emails++
		case ChannelTeams:
			teams++
		}
	}
	if emails != 2 || teams != 1 {
		t.Fatalf("got emails=%d teams=%d, want 2/1", emails, teams)
	}
}

func TestFeedbackRecordedHR_NoTeamsWhenDisabled(t *testing.T) {
	msgs := FeedbackRecordedHR([]string{"hr@x.com"}, false, "", "", "gm@x.com", "pass", "https://dash/applications/1")
	for _, m := range msgs {
		if m.Channel == ChannelTeams {
			t.Fatal("teams message present despite teamsEnabled=false")
		}
	}
	if len(msgs) != 1 {
		t.Fatalf("want 1 email message, got %d", len(msgs))
	}
}

func TestHRMessages_EmptyAddrsSkipped(t *testing.T) {
	msgs := NewScoredHR([]string{"", ""}, false, "x", "y", 1, "z")
	if len(msgs) != 0 {
		t.Fatalf("want 0 messages (empty addrs, no teams), got %d", len(msgs))
	}
}
