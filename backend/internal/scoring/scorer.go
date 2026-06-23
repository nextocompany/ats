package scoring

import (
	"context"

	"github.com/nexto/hr-ats/internal/ai"
	"github.com/nexto/hr-ats/pkg/config"
)

// LLMPart is the qualitative contribution from the language model.
type LLMPart struct {
	SkillsScore        int      // 0–20
	Strengths          []string // genuine positives only, 0-4 Thai bullets (gaps go to RedFlags)
	RedFlags           []string
	SuggestedPositions []string
}

// llmEvaluator abstracts the LLM call so it can be mocked.
type llmEvaluator interface {
	evaluate(ctx context.Context, p ai.Profile, jd JD) (LLMPart, error)
}

// Scorer scores a profile against a JD. locationScore (0–20) is computed by the
// caller (branch logic) and folded into the total.
type Scorer interface {
	Score(ctx context.Context, p ai.Profile, jd JD, locationScore int) (Result, error)
}

// compositeScorer combines deterministic rules with the LLM part.
type compositeScorer struct {
	llm llmEvaluator
}

// NewScorer selects the LLM backend by config (mock by default, no Azure keys).
func NewScorer(cfg *config.Config) Scorer {
	if cfg.UsesGeminiAI() {
		return compositeScorer{llm: newGeminiLLM(cfg)}
	}
	if cfg.UsesAzureAI() {
		return compositeScorer{llm: newAzureLLM(cfg)}
	}
	return compositeScorer{llm: mockLLM{}}
}

func (s compositeScorer) Score(ctx context.Context, p ai.Profile, jd JD, locationScore int) (Result, error) {
	locationScore = clamp(locationScore, 0, 20)
	eduOrd := maxEducation(p)
	months := totalExperienceMonths(p)
	// The effective weights (position config, or default when unset/invalid) drive
	// the Total and are recorded on the Result for explainability.
	weights := jd.Weights.orDefault()

	bd := Breakdown{
		Experience: experienceScore(months, jd.MinExperienceMonths),
		Education:  educationScore(eduOrd, jd.MinEducationLevel),
		Language:   languageScore(p),
		Location:   locationScore,
	}

	// Must-have gate fails short-circuit: record deterministic parts, skip the
	// LLM call (no token spend on a candidate we are auto-rejecting).
	if !gatePassed(p, jd) {
		bd.Skills = 0
		return Result{
			MustHavePassed: false,
			Breakdown:      bd,
			Weights:        weights,
			Total:          WeightedTotal(bd, weights),
		}, nil
	}

	part, err := s.llm.evaluate(ctx, p, jd)
	if err != nil {
		return Result{}, err
	}
	bd.Skills = clamp(part.SkillsScore, 0, 20)

	return Result{
		MustHavePassed:     true,
		Breakdown:          bd,
		Weights:            weights,
		Total:              WeightedTotal(bd, weights),
		Strengths:          part.Strengths,
		RedFlags:           part.RedFlags,
		SuggestedPositions: part.SuggestedPositions,
	}, nil
}
