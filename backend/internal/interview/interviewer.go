package interview

import "context"

// InterviewContext is the grounding the LLM needs to conduct and evaluate the
// interview. The service assembles it from the position JD and the candidate's
// resume summary, passing primitives only so this seam stays decoupled from the
// applications/positions/candidates packages.
type InterviewContext struct {
	PositionTitle    string
	Responsibilities string
	Qualifications   string
	CandidateName    string
	ProfileSummary   string // short resume summary (reuses the resume AI summary)
	MaxTurns         int    // hard cap on the number of questions
}

// Interviewer conducts the conversation and scores it. Mock (deterministic) is
// the default; Azure OpenAI is selected by config — mirroring ai.New / NewScorer.
type Interviewer interface {
	// NextTurn returns the AI interviewer's next message given the conversation so
	// far, and whether the interview is complete (the LLM may end early; the
	// service also enforces MaxTurns regardless).
	NextTurn(ctx context.Context, ic InterviewContext, history []Turn) (reply string, done bool, err error)
	// Evaluate scores the completed conversation.
	Evaluate(ctx context.Context, ic InterviewContext, history []Turn) (Evaluation, error)
}
