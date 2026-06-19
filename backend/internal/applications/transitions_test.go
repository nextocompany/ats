package applications

import "testing"

func TestCanTransition(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		want     bool
	}{
		// screened must go through the AI interview first — no direct shortlist.
		{"scored cannot shortlist", StatusScored, StatusShortlisted, false},
		{"scored cannot interview", StatusScored, StatusInterview, false},
		// after the AI interview, three choices open up.
		{"ai_interviewed → shortlist", StatusAIInterviewed, StatusShortlisted, true},
		{"ai_interviewed → interview", StatusAIInterviewed, StatusInterview, true},
		{"ai_interviewed → reject", StatusAIInterviewed, StatusRejected, true},
		{"ai_interviewed cannot hire", StatusAIInterviewed, StatusOffer, false},
		// shortlist → interview or reject only.
		{"shortlisted → interview", StatusShortlisted, StatusInterview, true},
		{"shortlisted → reject", StatusShortlisted, StatusRejected, true},
		{"shortlisted cannot hire", StatusShortlisted, StatusOffer, false},
		// scheduled interview → mark done or reject.
		{"interview → interviewed", StatusInterview, StatusInterviewed, true},
		{"interview → reject", StatusInterview, StatusRejected, true},
		{"interview cannot hire directly", StatusInterview, StatusOffer, false},
		// after the interview → reject, or submit into the approval chain (NOT a
		// direct offer PATCH anymore — that routes through the approval workflow).
		{"interviewed cannot offer directly", StatusInterviewed, StatusOffer, false},
		{"interviewed → reject", StatusInterviewed, StatusRejected, true},
		// pending_approval has no generic transitions — the approval decide endpoint
		// owns its only exits (offer / rejected).
		{"pending_approval no generic offer", StatusPendingApproval, StatusOffer, false},
		{"pending_approval no generic reject", StatusPendingApproval, StatusRejected, false},
		// offer → reject only.
		{"offer → reject", StatusOffer, StatusRejected, true},
		{"offer cannot reopen", StatusOffer, StatusShortlisted, false},
		// rejected is terminal.
		{"rejected is terminal", StatusRejected, StatusShortlisted, false},
		// ai_interview has no manual transitions (waits for completion).
		{"ai_interview no manual moves", StatusAIInterview, StatusShortlisted, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CanTransition(tc.from, tc.to); got != tc.want {
				t.Fatalf("CanTransition(%q,%q)=%v want %v", tc.from, tc.to, got, tc.want)
			}
		})
	}
}

func TestRequiresScheduleAndReason(t *testing.T) {
	if !RequiresSchedule(StatusInterview) {
		t.Fatal("interview must require a schedule")
	}
	if RequiresSchedule(StatusShortlisted) {
		t.Fatal("shortlist must not require a schedule")
	}
	if !RequiresReason(StatusRejected) {
		t.Fatal("reject must require a reason")
	}
	if RequiresReason(StatusOffer) {
		t.Fatal("offer must not require a reason")
	}
}

func TestCanRequestApproval(t *testing.T) {
	if !CanRequestApproval(StatusInterviewed) {
		t.Fatal("approval must be requestable from interviewed")
	}
	for _, s := range []string{StatusShortlisted, StatusInterview, StatusPendingApproval, StatusOffer, StatusScored} {
		if CanRequestApproval(s) {
			t.Fatalf("approval must not be requestable from %q", s)
		}
	}
}

func TestCanScheduleInterview(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{StatusAIInterviewed, true}, // first round (transition)
		{StatusShortlisted, true},   // first round (transition)
		{StatusInterview, true},     // additional round
		{StatusInterviewed, true},   // additional round after marking done
		{StatusScored, false},       // too early
		{StatusOffer, false},        // past interviews
		{StatusRejected, false},
	}
	for _, c := range cases {
		if got := CanScheduleInterview(c.status); got != c.want {
			t.Errorf("CanScheduleInterview(%q) = %v, want %v", c.status, got, c.want)
		}
	}
}
