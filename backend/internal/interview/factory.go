package interview

import "github.com/nexto/hr-ats/pkg/config"

// New returns the Interviewer implementation selected by configuration. Mock is
// the default so local and CI runs require no Azure credentials; the real
// interviewer reuses the same Azure OpenAI deployment as resume scoring. This is
// the single place provider choice lives, mirroring ai.New.
func New(cfg *config.Config) Interviewer {
	if cfg.UsesAzureAI() {
		return newAzureInterviewer(cfg)
	}
	return mockInterviewer{}
}
