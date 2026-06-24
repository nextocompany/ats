package applications

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/nexto/hr-ats/internal/rbac"
)

// CreateApprovalRequest opens a four-level approval chain for an interviewed
// application. The creator is the Staff-level (level 1) sign-off, recorded approved
// at creation; level 2 becomes the first active pending step with an SLA due_at.
// Everything (request, four steps, application status) is written in one
// transaction so partial state can never leak. The application status guard is
// enforced in SQL too (only an 'interviewed' row flips).
func (r *pgRepository) CreateApprovalRequest(ctx context.Context, applicationID, createdBy uuid.UUID, slaHours int) (ApprovalRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("applications: begin approval request: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx,
		`UPDATE applications SET status = $2, updated_at = NOW()
		 WHERE id = $1 AND status = $3`,
		applicationID, StatusPendingApproval, StatusInterviewed)
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("applications: set pending approval: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Lost the race (status no longer 'interviewed', or a request already opened).
		return ApprovalRequest{}, ErrApprovalConflict
	}

	var reqID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO approval_requests (application_id, status, current_level, created_by)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		applicationID, ApprovalPending, 2, createdBy).Scan(&reqID)
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("applications: create approval request: %w", err)
	}

	for _, c := range approvalChain {
		switch c.Level {
		case 1: // Staff sign-off completed by the creator at submission time.
			_, err = tx.Exec(ctx,
				`INSERT INTO approval_steps (request_id, level, role, status, approver_id, decided_at)
				 VALUES ($1, $2, $3, $4, $5, NOW())`,
				reqID, c.Level, c.Role, ApprovalApproved, createdBy)
		case 2: // First active pending step; carries the SLA deadline.
			_, err = tx.Exec(ctx,
				`INSERT INTO approval_steps (request_id, level, role, status, due_at)
				 VALUES ($1, $2, $3, $4, NOW() + make_interval(hours => $5))`,
				reqID, c.Level, c.Role, ApprovalPending, slaHours)
		default: // Not yet active: no due_at until the previous level approves.
			_, err = tx.Exec(ctx,
				`INSERT INTO approval_steps (request_id, level, role, status)
				 VALUES ($1, $2, $3, $4)`,
				reqID, c.Level, c.Role, ApprovalPending)
		}
		if err != nil {
			return ApprovalRequest{}, fmt.Errorf("applications: create approval step %d: %w", c.Level, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ApprovalRequest{}, fmt.Errorf("applications: commit approval request: %w", err)
	}
	got, err := r.GetApprovalRequest(ctx, applicationID)
	if err != nil {
		return ApprovalRequest{}, err
	}
	if got == nil {
		return ApprovalRequest{}, fmt.Errorf("applications: approval request vanished after create")
	}
	return *got, nil
}

// GetApprovalRequest loads the latest approval request for an application plus its
// ordered steps, or (nil, nil) when none exists.
func (r *pgRepository) GetApprovalRequest(ctx context.Context, applicationID uuid.UUID) (*ApprovalRequest, error) {
	const q = `
		SELECT id, application_id, status, current_level, created_at, decided_at, COALESCE(decision_reason,'')
		FROM approval_requests
		WHERE application_id = $1
		ORDER BY created_at DESC
		LIMIT 1`
	var req ApprovalRequest
	err := r.pool.QueryRow(ctx, q, applicationID).Scan(
		&req.ID, &req.ApplicationID, &req.Status, &req.CurrentLevel,
		&req.CreatedAt, &req.DecidedAt, &req.DecisionReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("applications: get approval request: %w", err)
	}
	steps, err := r.loadApprovalSteps(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	req.Steps = steps
	return &req, nil
}

// GetApprovalRequestByID loads an approval request (with steps) by its own id.
func (r *pgRepository) GetApprovalRequestByID(ctx context.Context, id uuid.UUID) (*ApprovalRequest, error) {
	const q = `
		SELECT id, application_id, status, current_level, created_at, decided_at, COALESCE(decision_reason,'')
		FROM approval_requests WHERE id = $1`
	var req ApprovalRequest
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&req.ID, &req.ApplicationID, &req.Status, &req.CurrentLevel,
		&req.CreatedAt, &req.DecidedAt, &req.DecisionReason,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("applications: get approval request by id: %w", err)
	}
	steps, err := r.loadApprovalSteps(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	req.Steps = steps
	return &req, nil
}

func (r *pgRepository) loadApprovalSteps(ctx context.Context, requestID uuid.UUID) ([]ApprovalStep, error) {
	const q = `
		SELECT s.id, s.level, s.role, s.status, COALESCE(u.full_name, u.email, ''),
		       COALESCE(s.comment,''), s.due_at, s.escalated, s.decided_at
		FROM approval_steps s
		LEFT JOIN users u ON u.id = s.approver_id
		WHERE s.request_id = $1
		ORDER BY s.level`
	rows, err := r.pool.Query(ctx, q, requestID)
	if err != nil {
		return nil, fmt.Errorf("applications: list approval steps: %w", err)
	}
	defer rows.Close()

	out := make([]ApprovalStep, 0, approvalMaxLevel)
	for rows.Next() {
		var s ApprovalStep
		if err := rows.Scan(
			&s.ID, &s.Level, &s.Role, &s.Status, &s.ApproverName,
			&s.Comment, &s.DueAt, &s.Escalated, &s.DecidedAt,
		); err != nil {
			return nil, fmt.Errorf("applications: scan approval step: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate approval steps: %w", err)
	}
	return out, nil
}

// DecideApproval records a decision on the active step and advances or terminates
// the chain — all in one transaction. Approving the final level advances the
// application to offer; any reject rejects the request and the application (with
// the reason persisted to applications.rejection_reason).
func (r *pgRepository) DecideApproval(ctx context.Context, a approvalDecideArgs) (ApprovalRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("applications: begin decide approval: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var appID uuid.UUID
	if err := tx.QueryRow(ctx,
		`SELECT application_id FROM approval_requests WHERE id = $1 AND status = $2 AND current_level = $3 FOR UPDATE`,
		a.RequestID, ApprovalPending, a.Level).Scan(&appID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Already decided, or advanced past this level by a concurrent decide.
			return ApprovalRequest{}, ErrApprovalConflict
		}
		return ApprovalRequest{}, fmt.Errorf("applications: lock approval request: %w", err)
	}

	if a.Approve {
		tag, err := tx.Exec(ctx,
			`UPDATE approval_steps SET status = $3, approver_id = $4, comment = NULLIF($5,''), decided_at = NOW()
			 WHERE request_id = $1 AND level = $2`,
			a.RequestID, a.Level, ApprovalApproved, a.ApproverID, a.Comment)
		if err != nil {
			return ApprovalRequest{}, fmt.Errorf("applications: approve step: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ApprovalRequest{}, fmt.Errorf("applications: approval step %d not found for request", a.Level)
		}
		if a.Level >= approvalMaxLevel {
			if _, err := tx.Exec(ctx,
				`UPDATE approval_requests SET status = $2, decided_at = NOW() WHERE id = $1`,
				a.RequestID, ApprovalApproved); err != nil {
				return ApprovalRequest{}, fmt.Errorf("applications: finalize approval: %w", err)
			}
			if _, err := tx.Exec(ctx,
				`UPDATE applications SET status = $2, updated_at = NOW() WHERE id = $1`,
				appID, StatusOffer); err != nil {
				return ApprovalRequest{}, fmt.Errorf("applications: set offer: %w", err)
			}
		} else {
			next := a.Level + 1
			if _, err := tx.Exec(ctx,
				`UPDATE approval_requests SET current_level = $2 WHERE id = $1`,
				a.RequestID, next); err != nil {
				return ApprovalRequest{}, fmt.Errorf("applications: advance approval level: %w", err)
			}
			if _, err := tx.Exec(ctx,
				`UPDATE approval_steps SET due_at = NOW() + make_interval(hours => $3)
				 WHERE request_id = $1 AND level = $2`,
				a.RequestID, next, a.SLAHours); err != nil {
				return ApprovalRequest{}, fmt.Errorf("applications: arm next step sla: %w", err)
			}
		}
	} else {
		tag, err := tx.Exec(ctx,
			`UPDATE approval_steps SET status = $3, approver_id = $4, comment = NULLIF($5,''), decided_at = NOW()
			 WHERE request_id = $1 AND level = $2`,
			a.RequestID, a.Level, ApprovalRejected, a.ApproverID, a.Comment)
		if err != nil {
			return ApprovalRequest{}, fmt.Errorf("applications: reject step: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return ApprovalRequest{}, fmt.Errorf("applications: approval step %d not found for request", a.Level)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE approval_requests SET status = $2, decision_reason = $3, decided_at = NOW() WHERE id = $1`,
			a.RequestID, ApprovalRejected, a.Reason); err != nil {
			return ApprovalRequest{}, fmt.Errorf("applications: reject approval: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE applications SET status = $2, rejection_reason = $3, updated_at = NOW() WHERE id = $1`,
			appID, StatusRejected, a.Reason); err != nil {
			return ApprovalRequest{}, fmt.Errorf("applications: reject application: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return ApprovalRequest{}, fmt.Errorf("applications: commit decide approval: %w", err)
	}
	got, err := r.GetApprovalRequestByID(ctx, a.RequestID)
	if err != nil {
		return ApprovalRequest{}, err
	}
	if got == nil {
		return ApprovalRequest{}, fmt.Errorf("applications: approval request vanished after decide")
	}
	return *got, nil
}

// ListPendingApprovals returns every in-flight request whose active step is visible
// to scope, ranked by SLA urgency. The handler filters to the caller's level.
func (r *pgRepository) ListPendingApprovals(ctx context.Context, scope rbac.Scope) ([]ApprovalQueueItem, error) {
	var args []any
	where := "r.status = 'pending' AND s.level = r.current_level AND s.status = 'pending'"
	if clause, cargs := scope.ApplicationsClause(len(args) + 1); clause != "" {
		where += " AND " + clause
		args = append(args, cargs...)
	}
	q := `
		SELECT r.id, a.id, COALESCE(c.full_name,''),
		       COALESCE(NULLIF(p.title_en,''), p.title_th, ''),
		       a.assigned_store_id, s.level, s.role, a.ai_score, s.due_at, s.created_at
		FROM approval_requests r
		JOIN approval_steps s ON s.request_id = r.id
		JOIN applications a ON a.id = r.application_id
		JOIN candidates c ON c.id = a.candidate_id
		JOIN positions p ON p.id = a.position_id
		WHERE ` + where + `
		ORDER BY s.due_at ASC NULLS LAST, r.created_at ASC`
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("applications: list pending approvals: %w", err)
	}
	defer rows.Close()

	out := make([]ApprovalQueueItem, 0)
	for rows.Next() {
		var it ApprovalQueueItem
		if err := rows.Scan(
			&it.RequestID, &it.ApplicationID, &it.CandidateName, &it.PositionTitle,
			&it.StoreID, &it.ActiveLevel, &it.ActiveRole, &it.AIScore, &it.DueAt, &it.WaitingSince,
		); err != nil {
			return nil, fmt.Errorf("applications: scan pending approval: %w", err)
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate pending approvals: %w", err)
	}
	return out, nil
}

// ListOverdueApprovalSteps returns active pending steps past their SLA that have
// not been escalated yet — the input to the SLA reminder sweep.
func (r *pgRepository) ListOverdueApprovalSteps(ctx context.Context) ([]OverdueApprovalStep, error) {
	const q = `
		SELECT s.id, s.role, a.assigned_store_id, COALESCE(c.full_name,''),
		       COALESCE(NULLIF(p.title_en,''), p.title_th, '')
		FROM approval_steps s
		JOIN approval_requests r ON r.id = s.request_id AND r.status = 'pending' AND r.current_level = s.level
		JOIN applications a ON a.id = r.application_id
		JOIN candidates c ON c.id = a.candidate_id
		JOIN positions p ON p.id = a.position_id
		WHERE s.status = 'pending' AND s.escalated = FALSE
		  AND s.due_at IS NOT NULL AND s.due_at < NOW()`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("applications: list overdue approval steps: %w", err)
	}
	defer rows.Close()

	out := make([]OverdueApprovalStep, 0)
	for rows.Next() {
		var s OverdueApprovalStep
		if err := rows.Scan(&s.StepID, &s.Role, &s.StoreID, &s.CandidateName, &s.PositionTitle); err != nil {
			return nil, fmt.Errorf("applications: scan overdue approval step: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate overdue approval steps: %w", err)
	}
	return out, nil
}

// MarkApprovalStepEscalated flags a step so the sweep never re-notifies it.
func (r *pgRepository) MarkApprovalStepEscalated(ctx context.Context, stepID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx,
		`UPDATE approval_steps SET escalated = TRUE WHERE id = $1`, stepID); err != nil {
		return fmt.Errorf("applications: mark approval step escalated: %w", err)
	}
	return nil
}
