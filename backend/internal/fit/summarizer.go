package fit

import "context"

// Summarizer turns the gathered Inputs into a cross-position Analysis. The mock
// implementation is deterministic; the Azure implementation calls the LLM.
type Summarizer interface {
	Summarize(ctx context.Context, in Inputs) (Analysis, error)
}
