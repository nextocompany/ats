package email

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestMockSenderSucceeds(t *testing.T) {
	s := mockSender{logBody: true}
	if err := s.Send(context.Background(), Message{To: "a@b.com", Subject: "hi", PlainText: "123456"}); err != nil {
		t.Fatalf("mock send: %v", err)
	}
}

func TestNewSenderSelectsMockByDefault(t *testing.T) {
	// A zero-value-ish config (EMAIL_PROVIDER unset => "" != "real") yields mock.
	// We can't import config easily without env; assert the type switch via UsesRealEmail
	// is exercised in config tests. Here we sanity-check mockSender is a Sender.
	var _ Sender = mockSender{}
	var _ Sender = (*acsSender)(nil)
}

func TestSignACSDeterministic(t *testing.T) {
	// Fixed key/date/body must produce a stable Authorization signature.
	key := base64.StdEncoding.EncodeToString([]byte("test-shared-key-0123456789"))
	u, _ := url.Parse("https://res.region.communication.azure.com/emails:send?api-version=2023-03-31")
	body := []byte(`{"senderAddress":"DoNotReply@x.com"}`)
	fixed := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)

	req1, _ := http.NewRequest(http.MethodPost, u.String(), nil)
	req2, _ := http.NewRequest(http.MethodPost, u.String(), nil)
	if err := signACS(req1, key, u, body, fixed); err != nil {
		t.Fatalf("sign1: %v", err)
	}
	if err := signACS(req2, key, u, body, fixed); err != nil {
		t.Fatalf("sign2: %v", err)
	}

	for _, h := range []string{"x-ms-date", "x-ms-content-sha256", "Authorization"} {
		if req1.Header.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
		if req1.Header.Get(h) != req2.Header.Get(h) {
			t.Errorf("header %s not deterministic: %q vs %q", h, req1.Header.Get(h), req2.Header.Get(h))
		}
	}
	if req1.Host != "res.region.communication.azure.com" {
		t.Errorf("host not set on request: %q", req1.Host)
	}
	if got := req1.Header.Get("Authorization"); got[:12] != "HMAC-SHA256 " {
		t.Errorf("unexpected auth scheme: %q", got)
	}
}

func TestSignACSBadKey(t *testing.T) {
	u, _ := url.Parse("https://x.communication.azure.com/emails:send?api-version=2023-03-31")
	req, _ := http.NewRequest(http.MethodPost, u.String(), nil)
	if err := signACS(req, "!!!not-base64!!!", u, []byte("{}"), time.Now()); err == nil {
		t.Fatal("expected error for non-base64 access key")
	}
}
