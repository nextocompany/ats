package scoring

import (
	"context"
	"testing"

	"github.com/nexto/hr-ats/internal/ai"
)

func qualifiedProfile() ai.Profile {
	return ai.Profile{
		Personal:   ai.Personal{Name: "สมชาย"},
		Experience: []ai.Experience{{DurationMonths: 36}},
		Education:  []ai.Education{{Degree: "ปริญญาตรี"}},
		Languages:  []ai.Language{{Language: "Thai"}, {Language: "English"}},
		Skills:     []string{"cashier", "POS"},
	}
}

func TestScore_GateFailSkipsLLM(t *testing.T) {
	s := compositeScorer{llm: failLLM{}} // would error if called
	jd := JD{MinEducationLevel: eduMaster, MinExperienceMonths: 0}

	res, err := s.Score(context.Background(), qualifiedProfile(), jd, 10)
	if err != nil {
		t.Fatalf("unexpected error (LLM should be skipped): %v", err)
	}
	if res.MustHavePassed {
		t.Error("expected gate to fail (needs master, has bachelor)")
	}
	if res.Breakdown.Skills != 0 {
		t.Errorf("expected skills 0 on gate fail, got %d", res.Breakdown.Skills)
	}
}

func TestScore_HappyPath(t *testing.T) {
	s := compositeScorer{llm: mockLLM{}}
	jd := JD{MinEducationLevel: eduDiploma, MinExperienceMonths: 12, Keywords: []string{"cashier", "POS"}}

	res, err := s.Score(context.Background(), qualifiedProfile(), jd, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !res.MustHavePassed {
		t.Fatal("expected gate to pass")
	}
	if res.Total <= 0 || res.Total > 100 {
		t.Errorf("total out of range: %d", res.Total)
	}
	if res.Breakdown.Location != 20 {
		t.Errorf("expected location 20, got %d", res.Breakdown.Location)
	}
	if len(res.Strengths) != 3 {
		t.Errorf("expected 3 Thai strengths, got %d", len(res.Strengths))
	}
	// experience 36 >= 12*2 → 30; education bachelor(3) vs min diploma(2) → +1 → 7;
	// language thai+eng → 10; skills overlap 2 → 10+2*3=16; location 20 → 83.
	if res.Total != 83 {
		t.Errorf("expected total 83, got %d (breakdown %+v)", res.Total, res.Breakdown)
	}
}

func TestScore_Clamp(t *testing.T) {
	s := compositeScorer{llm: bigLLM{}}
	jd := JD{MinEducationLevel: eduHigh, MinExperienceMonths: 1}
	res, err := s.Score(context.Background(), qualifiedProfile(), jd, 100)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total > 100 || res.Breakdown.Location > 20 || res.Breakdown.Skills > 20 {
		t.Errorf("expected clamping, got %+v total=%d", res.Breakdown, res.Total)
	}
}

// --- test doubles ---

type failLLM struct{}

func (failLLM) evaluate(context.Context, ai.Profile, JD) (LLMPart, error) {
	return LLMPart{}, context.Canceled // sentinel: must not be reached on gate fail
}

type bigLLM struct{}

func (bigLLM) evaluate(context.Context, ai.Profile, JD) (LLMPart, error) {
	return LLMPart{SkillsScore: 999}, nil
}
