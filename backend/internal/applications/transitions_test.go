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
		// after the interview → hire (offer) or reject.
		{"interviewed → offer", StatusInterviewed, StatusOffer, true},
		{"interviewed → reject", StatusInterviewed, StatusRejected, true},
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
