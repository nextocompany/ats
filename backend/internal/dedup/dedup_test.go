package dedup

import (
	"testing"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/candidates"
)

func TestDecide(t *testing.T) {
	newID := uuid.New()
	existing := uuid.New()

	tests := []struct {
		name      string
		inName    string
		inPhone   string
		inEmail   string
		inIDCard  string
		cands     []candidates.Candidate
		wantState string
		wantCanon uuid.UUID
	}{
		{
			name:      "exact id card auto-merges",
			inIDCard:  "1234567890123",
			cands:     []candidates.Candidate{{ID: existing, FullName: "Somchai", IDCard: "1234567890123"}},
			wantState: StateAutoMerged,
			wantCanon: existing,
		},
		{
			name:      "contact + fuzzy name auto-merges",
			inName:    "สมชาย ใจดี",
			inPhone:   "0812345678",
			cands:     []candidates.Candidate{{ID: existing, FullName: "สมชาย ใจด", Phone: "0812345678"}},
			wantState: StateAutoMerged,
			wantCanon: existing,
		},
		{
			name:      "contact only without name match → review",
			inName:    "Totally Different Name Here",
			inEmail:   "a@b.com",
			cands:     []candidates.Candidate{{ID: existing, FullName: "Someone Else Entirely", Email: "a@b.com"}},
			wantState: StatePendingReview,
			wantCanon: newID,
		},
		{
			name:      "no match → none, new is canonical",
			inName:    "Unique Person",
			inPhone:   "0999999999",
			cands:     nil,
			wantState: StateNone,
			wantCanon: newID,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := decide(newID, tc.inName, tc.inPhone, tc.inEmail, tc.inIDCard, tc.cands)
			if d.State != tc.wantState {
				t.Errorf("state: got %q want %q (conf %v)", d.State, tc.wantState, d.Confidence)
			}
			if d.CanonicalID != tc.wantCanon {
				t.Errorf("canonical: got %v want %v", d.CanonicalID, tc.wantCanon)
			}
		})
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"สมชาย", "สมชา", 1},
		{"", "abc", 3},
	}
	for _, c := range cases {
		if got := levenshtein(c.a, c.b); got != c.want {
			t.Errorf("levenshtein(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}
