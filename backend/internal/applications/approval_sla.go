package applications

import (
	"context"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/nexto/hr-ats/internal/notify"
)

// slaStore is the narrow repository slice the SLA sweep needs.
type slaStore interface {
	ListOverdueApprovalSteps(ctx context.Context) ([]OverdueApprovalStep, error)
	MarkApprovalStepEscalated(ctx context.Context, stepID uuid.UUID) error
}

// ApprovalSLAService escalates approval steps left pending past their SLA. It is
// wired into the worker as the handler for queue.TypeApprovalSLASweep.
type ApprovalSLAService struct {
	store        slaStore
	notifier     notify.Notifier
	hr           HRDirectory
	dashURL      string
	teamsEnabled bool
}

// NewApprovalSLAService builds the SLA sweep service.
func NewApprovalSLAService(store slaStore, notifier notify.Notifier, hr HRDirectory, dashURL string, teamsEnabled bool) *ApprovalSLAService {
	return &ApprovalSLAService{store: store, notifier: notifier, hr: hr, dashURL: dashURL, teamsEnabled: teamsEnabled}
}

// HandleApprovalSLASweep finds overdue, not-yet-escalated active steps, reminds the
// responsible approvers, and marks each escalated so it is never re-notified. A
// single step's failure (email/mark) is logged and skipped — only a failure to load
// the overdue set returns an error so asynq retries the whole sweep.
func (s *ApprovalSLAService) HandleApprovalSLASweep(ctx context.Context, _ *asynq.Task) error {
	steps, err := s.store.ListOverdueApprovalSteps(ctx)
	if err != nil {
		return err
	}
	escalated := 0
	for _, st := range steps {
		emails, err := s.hr.EmailsForRoleStore(ctx, st.Role, st.StoreID)
		if err != nil {
			log.Warn().Err(err).Str("step", st.StepID.String()).Msg("approval sla: resolve approvers failed (skip)")
			continue
		}
		if len(emails) > 0 || s.teamsEnabled {
			label := levelLabelForRole(st.Role)
			msgs := notify.ApprovalEscalationHR(emails, s.teamsEnabled, st.CandidateName, label, s.dashURL+"/approvals")
			dispatchHR(ctx, s.notifier, msgs)
		}
		if err := s.store.MarkApprovalStepEscalated(ctx, st.StepID); err != nil {
			log.Warn().Err(err).Str("step", st.StepID.String()).Msg("approval sla: mark escalated failed (skip)")
			continue
		}
		escalated++
	}
	log.Info().Int("overdue", len(steps)).Int("escalated", escalated).Msg("approval sla sweep complete")
	return nil
}

// levelLabelForRole maps an approval role back to its level label for notifications.
func levelLabelForRole(role string) string {
	for _, c := range approvalChain {
		if c.Role == role {
			return levelLabel(c.Level)
		}
	}
	return ""
}
