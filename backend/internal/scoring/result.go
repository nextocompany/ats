// Package scoring implements F03 (Step 4): a hybrid scorer combining
// deterministic rules (gate, experience, education, language, location) with an
// LLM contribution (semantic skills match, Thai strengths, red flags). The LLM
// part sits behind a mock-default seam, mirroring the Sprint 1 AI providers.
package scoring

// Breakdown is the per-dimension score (PRP §3 weights).
type Breakdown struct {
	Experience int `json:"experience"` // 0–30
	Skills     int `json:"skills"`     // 0–20 (LLM)
	Education  int `json:"education"`  // 0–10
	Language   int `json:"language"`   // 0–10
	Location   int `json:"location"`   // 0–20
}

// Result is the full scoring outcome written to the application.
type Result struct {
	MustHavePassed     bool      `json:"must_have_passed"`
	Total              int       `json:"total"`
	Breakdown          Breakdown `json:"breakdown"`
	Weights            Weights   `json:"weights"`   // effective per-position weights applied to Total
	Strengths          []string  `json:"strengths"` // genuine positives only, 0-4 Thai bullets (gaps go to RedFlags)
	RedFlags           []string  `json:"red_flags"`
	SuggestedPositions []string  `json:"suggested_positions"`
}
