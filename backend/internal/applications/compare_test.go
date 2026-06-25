package applications

import "testing"

func TestCompareScore(t *testing.T) {
	cases := []struct {
		name      string
		screening float64
		interview float64
		want      float64
	}{
		{"both mid", 90, 87, 88.5},
		{"zero", 0, 0, 0},
		{"max", 100, 100, 100},
		{"example", 82, 86, 84.0},
		{"rounding half", 83, 86, 84.5},
		{"screening higher", 90, 80, 85.0},
		{"interview higher", 70, 99, 84.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareScore(tc.screening, tc.interview); got != tc.want {
				t.Errorf("CompareScore(%g,%g) = %g, want %g", tc.screening, tc.interview, got, tc.want)
			}
		})
	}
}

func TestEligibleCompareStatuses(t *testing.T) {
	in := make(map[string]bool, len(eligibleCompareStatuses))
	for _, s := range eligibleCompareStatuses {
		in[s] = true
	}
	// Must include every status that guarantees screening passed AND AI interview done.
	for _, want := range []string{
		StatusAIInterviewed, StatusShortlisted, StatusInterview,
		StatusInterviewed, StatusPendingApproval, StatusOffer, StatusHired,
	} {
		if !in[want] {
			t.Errorf("eligible set missing %q", want)
		}
	}
	// Must exclude pre-AI-interview / failure statuses.
	for _, bad := range []string{
		StatusPending, StatusParsed, StatusScored, StatusAIInterview,
		StatusRejected, StatusFailed, StatusInvalidResume, StatusNameMismatch,
	} {
		if in[bad] {
			t.Errorf("eligible set must not include %q (interview not completed / not screened-pass)", bad)
		}
	}
}
