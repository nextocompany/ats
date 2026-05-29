package peoplesoft

import (
	"context"

	"github.com/nexto/hr-ats/pkg/config"
)

// Client pushes hired candidates to PeopleSoft.
type Client interface {
	SyncHired(ctx context.Context, a Applicant) error
}

// NewClient selects the implementation by config (mock by default — no PS creds
// needed for local/CI).
func NewClient(cfg *config.Config) Client {
	if cfg.UsesRealPeopleSoft() {
		return newRESTClient(cfg)
	}
	return mockClient{}
}
