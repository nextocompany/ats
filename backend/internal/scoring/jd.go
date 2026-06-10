package scoring

import (
	"fmt"
	"strings"
)

// JD is the scoring view of a position's Master JD. The pipeline maps a
// positions.Position into this struct, keeping scoring decoupled from the
// positions repository.
//
// Title/Responsibilities/Qualifications are the Master JD prose: the LLM
// evaluators compare the candidate against this full job description, while
// Keywords remain a lightweight hint and the deterministic mock's signal.
type JD struct {
	Title               string
	MinEducationLevel   int
	MinExperienceMonths int
	Keywords            []string
	Responsibilities    string
	Qualifications      string
}

// promptText renders the JD as a compact block for the LLM user prompt. Falls
// back to keywords alone when no prose is present (pre-Master-JD positions).
func (j JD) promptText() string {
	var b strings.Builder
	if j.Title != "" {
		fmt.Fprintf(&b, "Position: %s\n", j.Title)
	}
	if j.Responsibilities != "" {
		fmt.Fprintf(&b, "Responsibilities:\n%s\n", j.Responsibilities)
	}
	if j.Qualifications != "" {
		fmt.Fprintf(&b, "Qualifications:\n%s\n", j.Qualifications)
	}
	if len(j.Keywords) > 0 {
		fmt.Fprintf(&b, "Keywords: %s\n", strings.Join(j.Keywords, ", "))
	}
	return strings.TrimSpace(b.String())
}
