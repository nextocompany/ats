package notify

import (
	"strings"
	"testing"
)

// These tests cover the branded-HTML additions: email twins carry HTML, LINE/Teams
// do not, user-derived values are escaped, and the no-drift plain body is preserved.

func TestEmailTwin_CarriesBrandedHTML_LineDoesNot(t *testing.T) {
	const portal = "https://careers.example.com"
	em := ApplicationReceivedEmailMessage("c@x.com", "สมชาย", "แคชเชียร์", portal)
	if em.HTML == "" {
		t.Fatal("email twin must carry HTML")
	}
	for _, want := range []string{"CP", "AXTRA", "สมชาย", portal + "/status", "cpaxtra.com"} {
		if !strings.Contains(em.HTML, want) {
			t.Errorf("branded HTML missing %q", want)
		}
	}
	ln := ApplicationReceivedMessage("U1", "สมชาย", "แคชเชียร์", portal)
	if ln.HTML != "" {
		t.Errorf("LINE message must not carry HTML, got %q", ln.HTML)
	}
	// no-drift: LINE plain body equals the email text/plain part.
	if ln.Body != em.Body {
		t.Errorf("LINE/email body drift:\n line: %q\nemail: %q", ln.Body, em.Body)
	}
}

func TestStatusEmail_EscapesUserValue(t *testing.T) {
	em := StatusEmailMessage("c@x.com", "สมชาย<script>alert(1)</script>", "scored", "https://x.com")
	if strings.Contains(em.HTML, "<script>") {
		t.Error("XSS: raw <script> from candidate name leaked into HTML")
	}
	if !strings.Contains(em.HTML, "&lt;script&gt;") {
		t.Error("candidate name not escaped in HTML")
	}
}

func TestDocumentReviewedEmail_RejectedEscapesReason(t *testing.T) {
	em := DocumentReviewedEmailMessage("c@x.com", "สมชาย", "photo", false, "<b>เบลอ</b>", "https://p")
	if em.HTML == "" {
		t.Fatal("rejected doc email must carry HTML")
	}
	if strings.Contains(em.HTML, "<b>เบลอ</b>") {
		t.Error("XSS: raw reason markup leaked into HTML")
	}
	if !strings.Contains(em.Body, "เบลอ") {
		t.Error("plain body should still carry the reason text")
	}
}

func TestInterviewInviteEmail_Twin(t *testing.T) {
	const portal = "https://careers.example.com"
	// No token → both channels skip.
	if m := InterviewInviteEmailMessage("c@x.com", "สมชาย", portal, ""); m.Recipient != "" {
		t.Error("email invite with no token must yield a zero Message")
	}
	if m := InterviewInviteMessage("U1", "สมชาย", portal, ""); m.Recipient != "" {
		t.Error("LINE invite with no token must yield a zero Message")
	}
	// No email → skip (the previously-missing path is now reachable, but still
	// guarded on an empty address).
	if m := InterviewInviteEmailMessage("", "สมชาย", portal, "tok123"); m.Recipient != "" {
		t.Error("email invite with no address must yield a zero Message")
	}

	em := InterviewInviteEmailMessage("c@x.com", "สมชาย", portal, "tok123")
	if em.Channel != ChannelEmail || em.HTML == "" {
		t.Fatalf("unexpected invite email: %+v", em)
	}
	// The opaque token must survive in the URL fragment in both the HTML href and
	// the plain body (it deliberately lives in #, out of server logs).
	if !strings.Contains(em.HTML, "#token=tok123") {
		t.Errorf("HTML CTA href must keep the token fragment: %q", em.HTML)
	}
	if !strings.Contains(em.Body, "#token=tok123") {
		t.Errorf("plain body must keep the token fragment: %q", em.Body)
	}
	// no-drift with the LINE invite.
	ln := InterviewInviteMessage("U1", "สมชาย", portal, "tok123")
	if ln.Body != em.Body {
		t.Errorf("LINE/email invite body drift:\n line: %q\nemail: %q", ln.Body, em.Body)
	}
}

func TestHRMessages_EmailHasHTML_TeamsDoesNot(t *testing.T) {
	msgs := NewScoredHR([]string{"hr@x.com"}, true, "สมชาย", "แคชเชียร์", 82, "https://dash.example.com/applications/1")
	var emailMsg, teamsMsg *Message
	for i := range msgs {
		switch msgs[i].Channel {
		case ChannelEmail:
			emailMsg = &msgs[i]
		case ChannelTeams:
			teamsMsg = &msgs[i]
		}
	}
	if emailMsg == nil || emailMsg.HTML == "" {
		t.Fatal("HR email must carry branded HTML")
	}
	if !strings.Contains(emailMsg.HTML, "82/100") {
		t.Errorf("HR HTML missing score: %q", emailMsg.HTML)
	}
	if teamsMsg == nil {
		t.Fatal("expected a Teams message when enabled")
	}
	if teamsMsg.HTML != "" {
		t.Errorf("Teams message must not carry HTML, got %q", teamsMsg.HTML)
	}
	if !strings.Contains(teamsMsg.Body, "82/100") {
		t.Errorf("Teams body missing score: %q", teamsMsg.Body)
	}
}

func TestApprovalDecidedHR_RejectedUsesDangerAccent(t *testing.T) {
	msgs := ApprovalDecidedHR([]string{"hr@x.com"}, false, "สมชาย", false, "https://dash.example.com/a/1")
	if len(msgs) == 0 || msgs[0].HTML == "" {
		t.Fatal("expected a branded HR email")
	}
	if !strings.Contains(msgs[0].HTML, "#D64545") {
		t.Errorf("rejected approval should use the danger accent color: %q", msgs[0].HTML)
	}
}
