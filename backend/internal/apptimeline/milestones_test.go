package apptimeline

import (
	"strings"
	"testing"
	"time"
)

// fixed reference times for deterministic assertions.
var (
	t0 = time.Date(2026, 6, 10, 9, 0, 0, 0, time.UTC) // applied (created_at)
	t1 = time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)
	t2 = time.Date(2026, 6, 12, 9, 0, 0, 0, time.UTC)
	t3 = time.Date(2026, 6, 13, 9, 0, 0, 0, time.UTC)
)

// byKey indexes a milestone slice for assertion convenience.
func byKey(ms []Milestone) map[string]Milestone {
	m := make(map[string]Milestone, len(ms))
	for _, x := range ms {
		m[x.Key] = x
	}
	return m
}

func TestBuild_AlwaysSynthesizesAppliedFromCreatedAt(t *testing.T) {
	// Fresh application, no recorded transitions yet (the "pending" default is an
	// INSERT, not an UPDATE, so the trigger records nothing). "applied" must still
	// appear, dated from created_at.
	ms := Build(nil, t0, "pending")
	m := byKey(ms)

	applied, ok := m[KeyApplied]
	if !ok {
		t.Fatalf("expected an %q milestone, got keys %v", KeyApplied, keys(ms))
	}
	if applied.State != StateCurrent {
		t.Errorf("applied state = %q, want %q (pending == applied is the current step)", applied.State, StateCurrent)
	}
	if applied.ReachedAt == nil || !applied.ReachedAt.Equal(t0) {
		t.Errorf("applied.ReachedAt = %v, want created_at %v", applied.ReachedAt, t0)
	}
	// The road ahead should be visible as upcoming steps.
	if sc := m[KeyScreening]; sc.State != StateUpcoming {
		t.Errorf("screening state = %q, want %q", sc.State, StateUpcoming)
	}
}

func TestBuild_NeverLeaksInternalChurnAsItsOwnStep(t *testing.T) {
	// parsed/scored/name_mismatch are internal; none may surface as a distinct
	// candidate milestone key.
	events := []Event{
		{To: "parsed", At: t1},
		{To: "scored", At: t2},
	}
	ms := Build(events, t0, "scored")
	for _, x := range ms {
		switch x.Key {
		case "parsed", "scored", "name_mismatch", "invalid_resume", "failed", "shortlisted", "ai_interviewed", "interviewed":
			t.Errorf("internal/raw status %q leaked as a milestone key", x.Key)
		}
	}
	m := byKey(ms)
	// parsed+scored collapse into a single "screening" milestone, dated from the
	// EARLIEST contributing event (parsed at t1).
	sc, ok := m[KeyScreening]
	if !ok {
		t.Fatalf("expected %q milestone", KeyScreening)
	}
	if sc.State != StateCurrent {
		t.Errorf("screening state = %q, want %q", sc.State, StateCurrent)
	}
	if sc.ReachedAt == nil || !sc.ReachedAt.Equal(t1) {
		t.Errorf("screening.ReachedAt = %v, want earliest contributing event %v", sc.ReachedAt, t1)
	}
	if a := m[KeyApplied]; a.State != StateDone {
		t.Errorf("applied state = %q, want %q", a.State, StateDone)
	}
}

func TestBuild_HappyPathProgress(t *testing.T) {
	events := []Event{
		{To: "parsed", At: t1},
		{To: "scored", At: t2},
		{To: "ai_interview", At: t3},
	}
	ms := Build(events, t0, "ai_interview")
	m := byKey(ms)

	if a := m[KeyApplied]; a.State != StateDone {
		t.Errorf("applied = %q, want done", a.State)
	}
	if sc := m[KeyScreening]; sc.State != StateDone {
		t.Errorf("screening = %q, want done", sc.State)
	}
	if ai := m[KeyAIInterview]; ai.State != StateCurrent {
		t.Errorf("ai_interview = %q, want current", ai.State)
	}
	if iv := m[KeyInterview]; iv.State != StateUpcoming {
		t.Errorf("interview = %q, want upcoming", iv.State)
	}
}

func TestBuild_RejectedBranchTerminatesAndHidesUpcoming(t *testing.T) {
	events := []Event{
		{To: "parsed", At: t1},
		{To: "scored", At: t2},
		{To: "rejected", At: t3},
	}
	ms := Build(events, t0, "rejected")
	m := byKey(ms)

	ns, ok := m[KeyNotSelected]
	if !ok {
		t.Fatalf("expected %q terminal milestone", KeyNotSelected)
	}
	if ns.State != StateCurrent {
		t.Errorf("not_selected state = %q, want current", ns.State)
	}
	if ns.ReachedAt == nil || !ns.ReachedAt.Equal(t3) {
		t.Errorf("not_selected.ReachedAt = %v, want rejection time %v", ns.ReachedAt, t3)
	}
	// Steps reached before the rejection stay done.
	if sc := m[KeyScreening]; sc.State != StateDone {
		t.Errorf("screening = %q, want done", sc.State)
	}
	// A rejected journey must NOT dangle future happy-path steps.
	for _, forbidden := range []string{KeyAIInterview, KeyInterview, KeyDecision, KeyOffer, KeyHired} {
		if _, present := m[forbidden]; present {
			t.Errorf("rejected timeline must not include upcoming step %q", forbidden)
		}
	}
}

func TestBuild_ActionNeededForRecoverableUploadProblem(t *testing.T) {
	for _, st := range []string{"invalid_resume", "name_mismatch", "failed"} {
		events := []Event{{To: st, At: t1}}
		ms := Build(events, t0, st)
		m := byKey(ms)
		an, ok := m[KeyActionNeeded]
		if !ok {
			t.Fatalf("status %q: expected %q milestone, got keys %v", st, KeyActionNeeded, keys(ms))
		}
		if an.State != StateCurrent {
			t.Errorf("status %q: action_needed state = %q, want current", st, an.State)
		}
		if a := m[KeyApplied]; a.State != StateDone {
			t.Errorf("status %q: applied state = %q, want done", st, a.State)
		}
		// No happy-path future steps while the candidate must act.
		if _, present := m[KeyScreening]; present {
			t.Errorf("status %q: must not show screening while action is needed", st)
		}
	}
}

func TestBuild_HiredIsTerminalPositive(t *testing.T) {
	events := []Event{
		{To: "scored", At: t1},
		{To: "interview", At: t2},
		{To: "offer", At: t3},
		{To: "hired", At: t3.Add(24 * time.Hour)},
	}
	ms := Build(events, t0, "hired")
	m := byKey(ms)
	if h := m[KeyHired]; h.State != StateCurrent {
		t.Errorf("hired = %q, want current", h.State)
	}
	if o := m[KeyOffer]; o.State != StateDone {
		t.Errorf("offer = %q, want done", o.State)
	}
}

func TestBuild_EveryMilestoneHasNonEmptyLabelAndDetail(t *testing.T) {
	// Label + detail are the candidate-facing copy; an empty one is a release bug.
	ms := Build([]Event{{To: "scored", At: t1}}, t0, "scored")
	for _, x := range ms {
		if x.Label == "" {
			t.Errorf("milestone %q has empty label", x.Key)
		}
		if x.Detail == "" {
			t.Errorf("milestone %q has empty detail", x.Key)
		}
	}
}

func TestBuild_ActionNeededDetailCarriesReapplyInstruction(t *testing.T) {
	// Regression guard: the action_needed step is the whole point of the branch —
	// its detail must tell the candidate to re-apply with their resume. The status
	// arrives via an UPDATE (so the milestone has a reached date); the instruction
	// must travel on the milestone regardless, not be gated behind a missing date.
	ms := Build([]Event{{To: "invalid_resume", At: t1}}, t0, "invalid_resume")
	var an *Milestone
	for i := range ms {
		if ms[i].Key == KeyActionNeeded {
			an = &ms[i]
		}
	}
	if an == nil {
		t.Fatalf("no %q milestone", KeyActionNeeded)
	}
	if an.ReachedAt == nil {
		t.Fatal("precondition: action_needed should have a reached date here")
	}
	if an.Detail == "" || !strings.Contains(an.Detail, "สมัครใหม่") {
		t.Errorf("action_needed.Detail = %q, want re-apply instruction", an.Detail)
	}
}

// keys is a small assertion helper.
func keys(ms []Milestone) []string {
	out := make([]string, len(ms))
	for i, x := range ms {
		out[i] = x.Key
	}
	return out
}
