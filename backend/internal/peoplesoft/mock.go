package peoplesoft

import (
	"context"

	"github.com/rs/zerolog/log"
)

// mockClient records the sync (no external call) so local/CI exercise the full
// hired flow without PeopleSoft.
type mockClient struct{}

func (mockClient) SyncHired(_ context.Context, a Applicant) error {
	log.Info().
		Str("ps_vacancy_id", a.PSVacancyID).
		Str("source_of_hire", a.SourceOfHire).
		Float64("ai_score", a.AIScore).
		Msg("[mock peoplesoft] create_applicant")
	return nil
}
