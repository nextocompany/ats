package reengage

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/nexto/hr-ats/pkg/queue"
)

// Enqueuer is the subset of asynq.Client the trigger needs (enables mocking).
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Trigger enqueues re-engagement work. It satisfies peoplesoft.ReengageTrigger so
// the PeopleSoft vacancy-opened webhook can fire re-engagement without importing
// this package.
type Trigger struct{ q Enqueuer }

// NewTrigger builds a re-engagement trigger over the asynq client.
func NewTrigger(q Enqueuer) *Trigger { return &Trigger{q: q} }

// OnVacancyOpened enqueues a re-engagement job for the position.
func (t *Trigger) OnVacancyOpened(_ context.Context, positionID uuid.UUID) error {
	task, err := queue.NewReengageVacancyTask(queue.ReengageVacancyPayload{PositionID: positionID.String()})
	if err != nil {
		return err
	}
	if _, err := t.q.Enqueue(task); err != nil {
		return fmt.Errorf("reengage: enqueue: %w", err)
	}
	return nil
}
