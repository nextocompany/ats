package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Repository is the application data-access contract.
type Repository interface {
	Create(ctx context.Context, a Application) (Application, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Application, int, error)
	ListByCandidate(ctx context.Context, candidateID uuid.UUID) ([]Application, error)
	SetRawFile(ctx context.Context, id uuid.UUID, blobURL string) error
	SetQueueTaskID(ctx context.Context, id uuid.UUID, taskID string) error
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
	// SetRejection sets status=rejected with an internal reason (never sent to the
	// candidate). The reason is mandatory at the handler layer.
	SetRejection(ctx context.Context, id uuid.UUID, reason string) error
	SetParseResults(ctx context.Context, id uuid.UUID, r ParseResult) error
	// Interview appointments (human interview scheduling, state-machine feature).
	CreateAppointment(ctx context.Context, a Appointment) (Appointment, error)
	FindAppointment(ctx context.Context, applicationID uuid.UUID) (*Appointment, error)
	// Interview feedback (structured panel outcome; many rows per application).
	CreateFeedback(ctx context.Context, f InterviewFeedback) (InterviewFeedback, error)
	ListFeedback(ctx context.Context, applicationID uuid.UUID) ([]InterviewFeedback, error)
	// Shortlist returns the top-N shortlisted applications visible to scope,
	// composite-ranked (AI score + TA interview rating) — powers the LM view.
	Shortlist(ctx context.Context, scope rbac.Scope, limit int) ([]ShortlistItem, error)
	// Sprint 2:
	SetCanonicalCandidate(ctx context.Context, id, candidateID uuid.UUID) error
	SetDedupState(ctx context.Context, id uuid.UUID, state string, confidence float64) error
	SetScore(ctx context.Context, id uuid.UUID, s Score) error
	SetAssignment(ctx context.Context, id uuid.UUID, storeNo *int, talentPool bool) error
	// Sprint 3:
	SetHired(ctx context.Context, id uuid.UUID) error
	SetPSSynced(ctx context.Context, id uuid.UUID) error
	SetPublicToken(ctx context.Context, id uuid.UUID, token string) error
	FindByPublicToken(ctx context.Context, token string) (*Application, error)
	// ExistsInScope reports whether the application is visible to the given RBAC
	// scope. Reuses the same scoping clause as the list endpoints (handles
	// all/subregion/store), so per-record authorization stays consistent with
	// list visibility.
	ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error)
	// Approval workflow (Module-3 3.5): four-level hiring sign-off chain. Create
	// and Decide are each a single transaction so the approval tables and
	// applications.status never diverge (see approval_repository.go).
	CreateApprovalRequest(ctx context.Context, applicationID, createdBy uuid.UUID, slaHours int) (ApprovalRequest, error)
	GetApprovalRequest(ctx context.Context, applicationID uuid.UUID) (*ApprovalRequest, error)
	GetApprovalRequestByID(ctx context.Context, id uuid.UUID) (*ApprovalRequest, error)
	DecideApproval(ctx context.Context, args approvalDecideArgs) (ApprovalRequest, error)
	ListPendingApprovals(ctx context.Context, scope rbac.Scope) ([]ApprovalQueueItem, error)
	ListOverdueApprovalSteps(ctx context.Context) ([]OverdueApprovalStep, error)
	MarkApprovalStepEscalated(ctx context.Context, stepID uuid.UUID) error
	// Offer management (Module-3 3.6): one offer per application. RespondOffer is a
	// single transaction so the offer and the application status never diverge.
	CreateOffer(ctx context.Context, applicationID, createdBy uuid.UUID, in OfferInput) (Offer, error)
	UpdateOffer(ctx context.Context, applicationID uuid.UUID, in OfferInput) (Offer, error)
	GetOfferByApplication(ctx context.Context, applicationID uuid.UUID) (*Offer, error)
	GetOfferByID(ctx context.Context, id uuid.UUID) (*Offer, error)
	SendOffer(ctx context.Context, applicationID uuid.UUID) (Offer, error)
	RespondOffer(ctx context.Context, offerID, accountID uuid.UUID, accept bool, reason string) (Offer, error)
	ListOffersByAccount(ctx context.Context, accountID uuid.UUID) ([]OfferView, error)
}

type pgRepository struct {
	pool *pgxpool.Pool
}

// NewRepository builds a Postgres-backed application repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) Create(ctx context.Context, a Application) (Application, error) {
	const q = `
		INSERT INTO applications (candidate_id, position_id, status, raw_file_type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, q, a.CandidateID, a.PositionID, a.Status, a.RawFileType).
		Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return Application{}, fmt.Errorf("applications: create: %w", err)
	}
	return a, nil
}

func (r *pgRepository) FindByID(ctx context.Context, id uuid.UUID) (*Application, error) {
	const q = `
		SELECT id, candidate_id, position_id, status,
		       COALESCE(raw_file_blob_url,''), COALESCE(raw_file_type,''),
		       COALESCE(ocr_text_blob_url,''), COALESCE(parsed_profile_blob_url,''),
		       ocr_confidence, COALESCE(needs_manual_review,false),
		       COALESCE(queue_task_id,''), parsed_at,
		       ai_score, must_have_passed, assigned_store_id,
		       COALESCE(talent_pool,false), COALESCE(dedup_state,''), created_at,
		       ai_score_breakdown, COALESCE(ai_summary,''), COALESCE(ai_red_flags,''),
		       ai_suggested_positions, COALESCE(rejection_reason,'')
		FROM applications WHERE id = $1`
	var a Application
	var breakdownRaw, suggestedRaw []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
		&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
		&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt,
		&a.AIScore, &a.MustHavePassed, &a.AssignedStoreID, &a.TalentPool, &a.DedupState, &a.CreatedAt,
		&breakdownRaw, &a.AISummary, &a.AIRedFlags, &suggestedRaw, &a.RejectionReason,
	)
	if err != nil {
		return nil, fmt.Errorf("applications: find by id: %w", err)
	}
	// Explainability JSONB columns are NULL until the application is scored;
	// unmarshal only when present so an unscored record stays clean.
	if len(breakdownRaw) > 0 {
		var bd ScoreBreakdown
		if jsonErr := json.Unmarshal(breakdownRaw, &bd); jsonErr == nil {
			a.AIScoreBreakdown = &bd
		}
	}
	if len(suggestedRaw) > 0 {
		_ = json.Unmarshal(suggestedRaw, &a.AISuggestedPositions)
	}
	return &a, nil
}

func (r *pgRepository) SetRawFile(ctx context.Context, id uuid.UUID, blobURL string) error {
	const q = `UPDATE applications SET raw_file_blob_url = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, blobURL); err != nil {
		return fmt.Errorf("applications: set raw file: %w", err)
	}
	return nil
}

func (r *pgRepository) SetQueueTaskID(ctx context.Context, id uuid.UUID, taskID string) error {
	const q = `UPDATE applications SET queue_task_id = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, taskID); err != nil {
		return fmt.Errorf("applications: set queue task id: %w", err)
	}
	return nil
}

func (r *pgRepository) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	const q = `UPDATE applications SET status = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, status); err != nil {
		return fmt.Errorf("applications: set status: %w", err)
	}
	return nil
}

func (r *pgRepository) SetRejection(ctx context.Context, id uuid.UUID, reason string) error {
	const q = `UPDATE applications SET status = $2, rejection_reason = $3, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, StatusRejected, reason); err != nil {
		return fmt.Errorf("applications: set rejection: %w", err)
	}
	return nil
}

func (r *pgRepository) CreateAppointment(ctx context.Context, a Appointment) (Appointment, error) {
	const q = `
		INSERT INTO interview_appointments
			(application_id, scheduled_at, duration_min, mode, location_text, online_join_url, calendar_event_id, created_by)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), $8)
		RETURNING id, created_at`
	err := r.pool.QueryRow(ctx, q,
		a.ApplicationID, a.ScheduledAt, a.DurationMin, a.Mode,
		a.LocationText, a.OnlineJoinURL, a.CalendarEventID, a.CreatedBy,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return Appointment{}, fmt.Errorf("applications: create appointment: %w", err)
	}
	return a, nil
}

func (r *pgRepository) FindAppointment(ctx context.Context, applicationID uuid.UUID) (*Appointment, error) {
	const q = `
		SELECT id, application_id, scheduled_at, duration_min, mode,
		       COALESCE(location_text,''), COALESCE(online_join_url,''),
		       COALESCE(calendar_event_id,''), created_at
		FROM interview_appointments
		WHERE application_id = $1
		ORDER BY created_at DESC
		LIMIT 1`
	var a Appointment
	err := r.pool.QueryRow(ctx, q, applicationID).Scan(
		&a.ID, &a.ApplicationID, &a.ScheduledAt, &a.DurationMin, &a.Mode,
		&a.LocationText, &a.OnlineJoinURL, &a.CalendarEventID, &a.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no appointment yet — not an error
	}
	if err != nil {
		return nil, fmt.Errorf("applications: find appointment: %w", err)
	}
	return &a, nil
}

func (r *pgRepository) CreateFeedback(ctx context.Context, f InterviewFeedback) (InterviewFeedback, error) {
	comp, err := json.Marshal(f.Competencies)
	if err != nil {
		return InterviewFeedback{}, fmt.Errorf("applications: marshal competencies: %w", err)
	}
	if f.Perspective == "" {
		f.Perspective = PerspectiveTA
	}
	const q = `
		INSERT INTO interview_feedback
			(application_id, appointment_id, interviewer_id, perspective, overall_rating, recommendation,
			 competencies, strengths, concerns, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,''), NULLIF($9,''), NULLIF($10,''))
		RETURNING id, created_at`
	err = r.pool.QueryRow(ctx, q,
		f.ApplicationID, f.AppointmentID, f.InterviewerID, f.Perspective, f.OverallRating, f.Recommendation,
		comp, f.Strengths, f.Concerns, f.Notes,
	).Scan(&f.ID, &f.CreatedAt)
	if err != nil {
		return InterviewFeedback{}, fmt.Errorf("applications: create feedback: %w", err)
	}
	return f, nil
}

func (r *pgRepository) ListFeedback(ctx context.Context, applicationID uuid.UUID) ([]InterviewFeedback, error) {
	const q = `
		SELECT f.id, f.application_id, f.appointment_id, COALESCE(f.perspective,'ta'), f.overall_rating, f.recommendation,
		       f.competencies, COALESCE(f.strengths,''), COALESCE(f.concerns,''), COALESCE(f.notes,''),
		       COALESCE(u.full_name, u.email, ''), f.created_at
		FROM interview_feedback f
		LEFT JOIN users u ON u.id = f.interviewer_id
		WHERE f.application_id = $1
		ORDER BY f.created_at DESC`
	rows, err := r.pool.Query(ctx, q, applicationID)
	if err != nil {
		return nil, fmt.Errorf("applications: list feedback: %w", err)
	}
	defer rows.Close()

	out := make([]InterviewFeedback, 0)
	for rows.Next() {
		var f InterviewFeedback
		var comp []byte
		if err := rows.Scan(
			&f.ID, &f.ApplicationID, &f.AppointmentID, &f.Perspective, &f.OverallRating, &f.Recommendation,
			&comp, &f.Strengths, &f.Concerns, &f.Notes, &f.InterviewerName, &f.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("applications: scan feedback: %w", err)
		}
		if len(comp) > 0 {
			if err := json.Unmarshal(comp, &f.Competencies); err != nil {
				return nil, fmt.Errorf("applications: decode competencies: %w", err)
			}
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate feedback: %w", err)
	}
	return out, nil
}

// Shortlist returns the top-N shortlisted applications visible to the given scope,
// ranked by a composite of AI score and the TA average interview rating. A
// store-scoped role (sgm = line manager) only ever sees its own store's shortlist.
func (r *pgRepository) Shortlist(ctx context.Context, scope rbac.Scope, limit int) ([]ShortlistItem, error) {
	if limit <= 0 {
		limit = defaultShortlistLimit
	}
	var args []any
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	where := "a.status = 'shortlisted'"
	if clause, cargs := scope.ApplicationsClause(len(args) + 1); clause != "" {
		where += " AND " + clause
		args = append(args, cargs...)
	}
	limitPH := add(limit)

	q := `
		SELECT a.id, COALESCE(c.full_name, ''), a.position_id::text,
		       COALESCE(NULLIF(p.title_en,''), p.title_th, ''), a.assigned_store_id,
		       a.ai_score, ta.avg_overall
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id
		JOIN positions p ON p.id = a.position_id
		LEFT JOIN (
			SELECT application_id, AVG(overall_rating) AS avg_overall
			FROM interview_feedback WHERE perspective = 'ta' GROUP BY application_id
		) ta ON ta.application_id = a.id
		WHERE ` + where + `
		ORDER BY (CASE WHEN ta.avg_overall IS NULL THEN COALESCE(a.ai_score,0)
		               ELSE COALESCE(a.ai_score,0)*0.6 + ta.avg_overall*20*0.4 END) DESC,
		         a.ai_score DESC NULLS LAST, a.created_at DESC
		LIMIT ` + limitPH
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("applications: shortlist: %w", err)
	}
	defer rows.Close()

	out := make([]ShortlistItem, 0)
	for rows.Next() {
		var it ShortlistItem
		if err := rows.Scan(
			&it.ApplicationID, &it.CandidateName, &it.PositionID, &it.PositionTitle,
			&it.AssignedStoreID, &it.AIScore, &it.TAAvgOverall,
		); err != nil {
			return nil, fmt.Errorf("applications: scan shortlist: %w", err)
		}
		ai, ta := 0.0, 0.0
		if it.AIScore != nil {
			ai = *it.AIScore
		}
		if it.TAAvgOverall != nil {
			ta = *it.TAAvgOverall
		}
		it.Composite = CompositeScore(ai, ta)
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate shortlist: %w", err)
	}
	return out, nil
}

func (r *pgRepository) SetParseResults(ctx context.Context, id uuid.UUID, res ParseResult) error {
	const q = `
		UPDATE applications SET
			status                  = $2,
			ocr_text_blob_url       = $3,
			parsed_profile_blob_url = $4,
			ocr_confidence          = $5,
			needs_manual_review     = $6,
			parsed_at               = NOW(),
			updated_at              = NOW()
		WHERE id = $1`
	_, err := r.pool.Exec(ctx, q, id, StatusParsed,
		res.OCRTextBlobURL, res.ParsedProfileBlobURL, res.OCRConfidence, res.NeedsManualReview)
	if err != nil {
		return fmt.Errorf("applications: set parse results: %w", err)
	}
	return nil
}

func (r *pgRepository) SetCanonicalCandidate(ctx context.Context, id, candidateID uuid.UUID) error {
	const q = `UPDATE applications SET candidate_id = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, candidateID); err != nil {
		return fmt.Errorf("applications: set canonical candidate: %w", err)
	}
	return nil
}

func (r *pgRepository) SetDedupState(ctx context.Context, id uuid.UUID, state string, confidence float64) error {
	const q = `UPDATE applications SET dedup_state = $2, dedup_confidence = $3, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, state, confidence); err != nil {
		return fmt.Errorf("applications: set dedup state: %w", err)
	}
	return nil
}

func (r *pgRepository) SetScore(ctx context.Context, id uuid.UUID, s Score) error {
	const q = `
		UPDATE applications SET
			status                 = $2,
			must_have_passed       = $3,
			ai_score               = $4,
			ai_score_breakdown     = $5,
			ai_summary             = $6,
			ai_red_flags           = $7,
			ai_suggested_positions = $8,
			updated_at             = NOW()
		WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, s.Status, s.MustHavePassed, s.Total,
		s.BreakdownJSON, s.Summary, s.RedFlags, s.SuggestedJSON); err != nil {
		return fmt.Errorf("applications: set score: %w", err)
	}
	return nil
}

func (r *pgRepository) SetAssignment(ctx context.Context, id uuid.UUID, storeNo *int, talentPool bool) error {
	const q = `UPDATE applications SET assigned_store_id = $2, talent_pool = $3, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, storeNo, talentPool); err != nil {
		return fmt.Errorf("applications: set assignment: %w", err)
	}
	return nil
}

func (r *pgRepository) SetHired(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET status = $2, hired_at = NOW(), updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, StatusHired); err != nil {
		return fmt.Errorf("applications: set hired: %w", err)
	}
	return nil
}

func (r *pgRepository) SetPSSynced(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE applications SET ps_synced_at = NOW(), updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id); err != nil {
		return fmt.Errorf("applications: set ps synced: %w", err)
	}
	return nil
}

func (r *pgRepository) SetPublicToken(ctx context.Context, id uuid.UUID, token string) error {
	const q = `UPDATE applications SET public_token = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, token); err != nil {
		return fmt.Errorf("applications: set public token: %w", err)
	}
	return nil
}

func (r *pgRepository) FindByPublicToken(ctx context.Context, token string) (*Application, error) {
	const q = `
		SELECT id, candidate_id, position_id, status,
		       COALESCE(raw_file_blob_url,''), COALESCE(raw_file_type,''),
		       COALESCE(ocr_text_blob_url,''), COALESCE(parsed_profile_blob_url,''),
		       ocr_confidence, COALESCE(needs_manual_review,false),
		       COALESCE(queue_task_id,''), parsed_at,
		       ai_score, must_have_passed, assigned_store_id,
		       COALESCE(talent_pool,false), COALESCE(dedup_state,''), created_at
		FROM applications WHERE public_token = $1`
	var a Application
	err := r.pool.QueryRow(ctx, q, token).Scan(
		&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
		&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
		&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt,
		&a.AIScore, &a.MustHavePassed, &a.AssignedStoreID, &a.TalentPool, &a.DedupState, &a.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("applications: find by public token: %w", err)
	}
	return &a, nil
}

func (r *pgRepository) ExistsInScope(ctx context.Context, id uuid.UUID, scope rbac.Scope) (bool, error) {
	clause, args := scope.ApplicationsClause(2) // $1 is the id
	q := "SELECT EXISTS(SELECT 1 FROM applications WHERE id = $1"
	allArgs := []any{id}
	if clause != "" {
		q += " AND " + clause
		allArgs = append(allArgs, args...)
	}
	q += ")"
	var ok bool
	if err := r.pool.QueryRow(ctx, q, allArgs...).Scan(&ok); err != nil {
		return false, fmt.Errorf("applications: exists in scope: %w", err)
	}
	return ok, nil
}
