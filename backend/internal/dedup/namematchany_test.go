package dedup

import "testing"

// TestNameMatchesAny is the guard on the th-OR-en resume-name gate: a CV is in one
// language, so it should match against either stored name — but an EMPTY stored
// name must never auto-pass (NameLooselyMatches returns true for a blank arg).
func TestNameMatchesAny(t *testing.T) {
	const (
		th = "สมชาย ใจดี"
		en = "Somchai Jaidee"
	)
	tests := []struct {
		name   string
		resume string
		names  []string
		want   bool
	}{
		{"thai resume matches thai name, en blank", th, []string{th, ""}, true},
		{"english resume matches en name, th blank", en, []string{"", en}, true},
		{"thai resume matches when both present", th, []string{th, en}, true},
		{"english resume matches when both present", en, []string{th, en}, true},
		{"neither matches, both present -> no match", "John Smith", []string{th, en}, false},
		{"all names empty -> no match (must not auto-pass)", th, []string{"", ""}, false},
		{"blank name alone never auto-passes a mismatching resume", "Jane Doe", []string{"", ""}, false},
		{"order/partial tolerated via NameLooselyMatches", "ใจดี สมชาย", []string{th, ""}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NameMatchesAny(tc.resume, tc.names...); got != tc.want {
				t.Errorf("NameMatchesAny(%q, %v) = %v, want %v", tc.resume, tc.names, got, tc.want)
			}
		})
	}
}
