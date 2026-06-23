package scoring

import "math"

// Per-dimension max caps: the intrinsic 0..cap range each sub-score is computed on
// (see rules.go / scorer.go). They are the normalization denominators, NOT the
// weights. A candidate who maxes every dimension has ratio 1.0 on each.
const (
	CapExperience = 30
	CapSkills     = 20
	CapEducation  = 10
	CapLanguage   = 10
	CapLocation   = 20
)

// WeightsTotal is the percentage budget weights must sum to.
const WeightsTotal = 100

// Weights are per-position importance percentages for the five screening
// dimensions. They sum to WeightsTotal (100) and turn the composite into a true
// 0-100 weighted score: total = Σ weight_i * (subscore_i / cap_i).
type Weights struct {
	Experience int `json:"experience"`
	Skills     int `json:"skills"`
	Education  int `json:"education"`
	Language   int `json:"language"`
	Location   int `json:"location"`
}

// DefaultWeights approximates the legacy cap ratio 30:20:10:10:20 (= 3:2:1:1:2)
// renormalized to sum 100. Largest-remainder rounding puts the spare +1 on
// experience. These reproduce the legacy relative ranking on a 0-100 scale.
func DefaultWeights() Weights {
	return Weights{Experience: 34, Skills: 22, Education: 11, Language: 11, Location: 22}
}

// Sum returns the total of all five weights.
func (w Weights) Sum() int {
	return w.Experience + w.Skills + w.Education + w.Language + w.Location
}

// Valid reports whether every weight is in [0,100] and they sum to exactly 100.
func (w Weights) Valid() bool {
	for _, v := range []int{w.Experience, w.Skills, w.Education, w.Language, w.Location} {
		if v < 0 || v > WeightsTotal {
			return false
		}
	}
	return w.Sum() == WeightsTotal
}

// orDefault returns w when valid, else the default weights. Defensive: scoring
// must never apply a malformed/zero-value weight set.
func (w Weights) orDefault() Weights {
	if w.Valid() {
		return w
	}
	return DefaultWeights()
}

// ratio normalizes a sub-score to [0,1] against its cap.
func ratio(sub, cap int) float64 {
	if cap <= 0 {
		return 0
	}
	r := float64(sub) / float64(cap)
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// WeightedTotal combines the per-dimension sub-scores into a 0-100 total using the
// given weights (falling back to defaults if invalid). The per-dimension sub-scores
// keep their own caps; the weights only re-balance their contribution.
func WeightedTotal(bd Breakdown, w Weights) int {
	w = w.orDefault()
	total := float64(w.Experience)*ratio(bd.Experience, CapExperience) +
		float64(w.Skills)*ratio(bd.Skills, CapSkills) +
		float64(w.Education)*ratio(bd.Education, CapEducation) +
		float64(w.Language)*ratio(bd.Language, CapLanguage) +
		float64(w.Location)*ratio(bd.Location, CapLocation)
	return clamp(int(math.Round(total)), 0, 100)
}
