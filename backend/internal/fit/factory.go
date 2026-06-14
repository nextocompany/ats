package fit

import "github.com/nexto/hr-ats/pkg/config"

// New selects the fit summarizer by config. Azure OpenAI when configured;
// otherwise a deterministic mock (local/dev/CI default). Gemini is intentionally
// not implemented here, matching internal/interview.
func New(cfg *config.Config) Summarizer {
	if cfg.UsesAzureAI() {
		return newAzureSummarizer(cfg)
	}
	return mockSummarizer{}
}
