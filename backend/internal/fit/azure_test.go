package fit

import (
	"testing"

	"github.com/google/uuid"
)

func TestParseFit_NumberAsStringAndBogusID(t *testing.T) {
	valid := uuid.New()
	bogus := uuid.New()
	catalogue := []PositionCard{{ID: valid, Title: "พนักงานขาย"}}

	content := `{
		"overall_fit":"strong",
		"summary":"เหมาะสมดี",
		"strengths":["ขยัน"],
		"concerns":["ประสบการณ์น้อย"],
		"recommended":[
			{"position_id":"` + valid.String() + `","title":"พนักงานขาย","fit_score":"86","reasons":["ตรงสายงาน"]},
			{"position_id":"` + bogus.String() + `","title":"ผี","fit_score":120,"reasons":["มั่ว"]},
			{"position_id":"not-a-uuid","title":"x","fit_score":10,"reasons":[]}
		],
		"no_match_reason":""
	}`

	a, err := parseFit(content, catalogue)
	if err != nil {
		t.Fatalf("parseFit: %v", err)
	}
	if a.OverallFit != OverallStrong {
		t.Errorf("overall_fit = %q, want strong", a.OverallFit)
	}
	if len(a.Recommended) != 1 {
		t.Fatalf("recommended len = %d, want 1 (bogus + non-uuid dropped)", len(a.Recommended))
	}
	r := a.Recommended[0]
	if r.PositionID != valid {
		t.Errorf("position_id = %v, want %v", r.PositionID, valid)
	}
	if r.FitScore != 86 { // coerced from string "86"
		t.Errorf("fit_score = %d, want 86", r.FitScore)
	}
}

func TestParseFit_ClampAndUnknownOverall(t *testing.T) {
	valid := uuid.New()
	catalogue := []PositionCard{{ID: valid, Title: "แคชเชียร์"}}
	content := `{"overall_fit":"banana","recommended":[{"position_id":"` + valid.String() + `","fit_score":250,"reasons":[]}]}`

	a, err := parseFit(content, catalogue)
	if err != nil {
		t.Fatalf("parseFit: %v", err)
	}
	if a.OverallFit != OverallWeak {
		t.Errorf("unknown overall_fit should normalize to weak, got %q", a.OverallFit)
	}
	if a.Recommended[0].FitScore != 100 {
		t.Errorf("fit_score should clamp to 100, got %d", a.Recommended[0].FitScore)
	}
	// title falls back to the catalogue title when omitted by the LLM
	if a.Recommended[0].Title != "แคชเชียร์" {
		t.Errorf("title fallback = %q, want แคชเชียร์", a.Recommended[0].Title)
	}
}

func TestParseFit_NoneVerdict(t *testing.T) {
	content := `{"overall_fit":"none","summary":"ไม่ตรงสายงาน","no_match_reason":"ประสบการณ์ไม่ตรงกับตำแหน่งใดเลย","recommended":[]}`
	a, err := parseFit(content, nil)
	if err != nil {
		t.Fatalf("parseFit: %v", err)
	}
	if a.OverallFit != OverallNone {
		t.Errorf("overall_fit = %q, want none", a.OverallFit)
	}
	if a.NoMatchReason == "" {
		t.Error("no_match_reason should be preserved")
	}
	if len(a.Recommended) != 0 {
		t.Errorf("recommended should be empty, got %d", len(a.Recommended))
	}
}

func TestParseFit_EmptyRecommendedForcesNone(t *testing.T) {
	// LLM claims a strong fit but provides no usable recommendation → force none.
	content := `{"overall_fit":"strong","recommended":[]}`
	a, err := parseFit(content, nil)
	if err != nil {
		t.Fatalf("parseFit: %v", err)
	}
	if a.OverallFit != OverallNone {
		t.Errorf("overall_fit = %q, want none (no usable recommendations)", a.OverallFit)
	}
	if a.NoMatchReason == "" {
		t.Error("a forced-none verdict should carry a no_match_reason")
	}
}
