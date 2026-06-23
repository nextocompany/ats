package scoring

import (
	"context"
	"strings"
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
	// Sub-scores: experience 36 >= 12*2 → 30; education bachelor(3) vs min diploma(2) → +1 → 7;
	// language thai+eng → 10; skills overlap 2 → 10+2*3=16; location 20.
	// Weighted by DEFAULT weights {34,22,11,11,22}: 34*(30/30) + 22*(16/20) + 11*(7/10)
	// + 11*(10/10) + 22*(20/20) = 34 + 17.6 + 7.7 + 11 + 22 = 92.3 → 92.
	if res.Total != 92 {
		t.Errorf("expected total 92, got %d (breakdown %+v)", res.Total, res.Breakdown)
	}
	if res.Weights != DefaultWeights() {
		t.Errorf("expected default weights recorded, got %+v", res.Weights)
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

func TestJD_PromptText(t *testing.T) {
	tests := []struct {
		name     string
		jd       JD
		contains []string
		absent   []string
	}{
		{
			name: "full Master JD prose",
			jd: JD{
				Title:            "ผู้จัดการแผนกเนื้อสัตว์",
				Responsibilities: "• บริหารจัดการแผนกเนื้อสัตว์",
				Qualifications:   "• ปริญญาตรี",
				Keywords:         []string{"butchery", "meat"},
			},
			contains: []string{"Position: ผู้จัดการแผนกเนื้อสัตว์", "Responsibilities:", "บริหารจัดการแผนกเนื้อสัตว์", "Qualifications:", "Keywords: butchery, meat"},
		},
		{
			name:     "keyword-only fallback (pre-Master-JD position)",
			jd:       JD{Keywords: []string{"cashier", "POS"}},
			contains: []string{"Keywords: cashier, POS"},
			absent:   []string{"Responsibilities:", "Qualifications:", "Position:"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.jd.promptText()
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("promptText() missing %q\n--- got ---\n%s", want, got)
				}
			}
			for _, no := range tt.absent {
				if strings.Contains(got, no) {
					t.Errorf("promptText() should not contain %q\n--- got ---\n%s", no, got)
				}
			}
		})
	}
}

// TestScore_EnglishEducationAndLanguage validates the deterministic rules on an
// English CV (the Thai path is covered by HappyPath) — English "Bachelor" must
// clear a diploma gate and an English-only language list scores 5.
func TestScore_EnglishEducationAndLanguage(t *testing.T) {
	s := compositeScorer{llm: mockLLM{}}
	prof := ai.Profile{
		Personal:   ai.Personal{Name: "John Smith"},
		Experience: []ai.Experience{{DurationMonths: 24}},
		Education:  []ai.Education{{Degree: "Bachelor's Degree"}},
		Languages:  []ai.Language{{Language: "English"}},
		Skills:     []string{"cashier"},
	}
	jd := JD{MinEducationLevel: eduDiploma, MinExperienceMonths: 12}

	res, err := s.Score(context.Background(), prof, jd, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !res.MustHavePassed {
		t.Error("English bachelor should clear a diploma gate")
	}
	if res.Breakdown.Language != 5 {
		t.Errorf("English-only language should score 5, got %d", res.Breakdown.Language)
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
