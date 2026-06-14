package fit

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestMockSummarizer_NoPositions(t *testing.T) {
	a, err := mockSummarizer{}.Summarize(context.Background(), Inputs{})
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if a.OverallFit != OverallNone {
		t.Errorf("overall_fit = %q, want none", a.OverallFit)
	}
	if a.NoMatchReason == "" {
		t.Error("expected a no_match_reason when there are no positions")
	}
	if len(a.Recommended) != 0 {
		t.Errorf("recommended should be empty, got %d", len(a.Recommended))
	}
}

func TestMockSummarizer_WithPositions(t *testing.T) {
	in := Inputs{Positions: []PositionCard{
		{ID: uuid.New(), Title: "พนักงานขาย"},
		{ID: uuid.New(), Title: "แคชเชียร์"},
		{ID: uuid.New(), Title: "หัวหน้าแผนก"},
	}}
	a, err := mockSummarizer{}.Summarize(context.Background(), in)
	if err != nil {
		t.Fatalf("summarize: %v", err)
	}
	if a.OverallFit != OverallModerate {
		t.Errorf("overall_fit = %q, want moderate", a.OverallFit)
	}
	if len(a.Recommended) != 2 {
		t.Fatalf("recommended len = %d, want 2 (capped)", len(a.Recommended))
	}
	if a.Recommended[0].PositionID != in.Positions[0].ID {
		t.Errorf("first recommendation should be the first position")
	}
	if a.Recommended[0].FitScore < a.Recommended[1].FitScore {
		t.Error("recommendations should be ranked best-first")
	}
}
