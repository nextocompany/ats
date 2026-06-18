package applications

import "math"

// Scorecard aggregation: combine the TA and Line-Manager scorecards into a single
// summary for the application detail view, plus a composite ranking score used by
// the Top-5 shortlist. Pure functions (no time/rand) so they are unit-testable.

// PerspectiveAgg is the averaged scorecard for one perspective.
type PerspectiveAgg struct {
	Count           int                `json:"count"`
	AvgOverall      float64            `json:"avg_overall"`      // mean of overall_rating (1..5)
	AvgCompetencies map[string]float64 `json:"avg_competencies"` // only dimensions actually rated (>0)
	Recommendations map[string]int     `json:"recommendations"`  // pass/hold/fail tally
}

// ScorecardSummary is the combined view across perspectives for one application.
type ScorecardSummary struct {
	TA          *PerspectiveAgg `json:"ta"`           // nil when no TA scorecard yet
	LineManager *PerspectiveAgg `json:"line_manager"` // nil when no LM scorecard yet
	// CompositeScore blends the AI score (0..100) with the TA average rating,
	// matching the shortlist ranking. nil when there is no AI score.
	CompositeScore *float64 `json:"composite_score"`
}

// composite weights — AI screening dominates, TA interview rating refines.
const (
	compositeAIWeight = 0.6
	compositeTAWeight = 0.4
)

// CompositeScore returns the shortlist ranking score for an AI score (0..100) and
// a TA average overall rating (1..5; 0 = none). When there is no TA rating the
// TA term collapses and the composite equals the AI score.
func CompositeScore(aiScore, taAvgOverall float64) float64 {
	if taAvgOverall <= 0 {
		return round1(aiScore)
	}
	return round1(aiScore*compositeAIWeight + taAvgOverall*20*compositeTAWeight)
}

// SummarizeFeedback aggregates a per-application feedback list into a summary.
// aiScore may be nil (no screening score yet).
func SummarizeFeedback(list []InterviewFeedback, aiScore *float64) ScorecardSummary {
	ta := aggregate(filterPerspective(list, PerspectiveTA))
	lm := aggregate(filterPerspective(list, PerspectiveLineManager))

	var composite *float64
	if aiScore != nil {
		taAvg := 0.0
		if ta != nil {
			taAvg = ta.AvgOverall
		}
		v := CompositeScore(*aiScore, taAvg)
		composite = &v
	}
	return ScorecardSummary{TA: ta, LineManager: lm, CompositeScore: composite}
}

func filterPerspective(list []InterviewFeedback, p string) []InterviewFeedback {
	out := make([]InterviewFeedback, 0, len(list))
	for _, f := range list {
		fp := f.Perspective
		if fp == "" {
			fp = PerspectiveTA // legacy rows
		}
		if fp == p {
			out = append(out, f)
		}
	}
	return out
}

// aggregate averages a single perspective's rows. Returns nil when empty.
func aggregate(list []InterviewFeedback) *PerspectiveAgg {
	if len(list) == 0 {
		return nil
	}
	agg := &PerspectiveAgg{
		Count:           len(list),
		AvgCompetencies: map[string]float64{},
		Recommendations: map[string]int{},
	}
	var overallSum int
	// competency sums + counts (only dimensions rated >0 contribute, so an
	// unrated dimension never drags the average toward 0).
	sums := map[string]int{}
	counts := map[string]int{}
	for _, f := range list {
		overallSum += f.OverallRating
		if f.Recommendation != "" {
			agg.Recommendations[f.Recommendation]++
		}
		for name, v := range competencyMap(f.Competencies) {
			if v > 0 {
				sums[name] += v
				counts[name]++
			}
		}
	}
	agg.AvgOverall = round1(float64(overallSum) / float64(len(list)))
	for name, total := range sums {
		agg.AvgCompetencies[name] = round1(float64(total) / float64(counts[name]))
	}
	return agg
}

func competencyMap(c InterviewCompetencies) map[string]int {
	return map[string]int{
		"communication":    c.Communication,
		"technical":        c.Technical,
		"experience":       c.Experience,
		"attitude":         c.Attitude,
		"culture_fit":      c.CultureFit,
		"growth_potential": c.GrowthPotential,
		"leadership":       c.Leadership,
	}
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
