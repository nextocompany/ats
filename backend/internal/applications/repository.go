package applications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexto/hr-ats/internal/letters"
	"github.com/nexto/hr-ats/internal/rbac"
)

// ErrNotFound is returned by account-scoped portal lookups when the token is
// unknown or the application is not owned by the requesting account (the two are
// deliberately indistinguishable to the caller — no IDOR oracle).
var ErrNotFound = errors.New("applications: not found")

// Repository is the application data-access contract.
type Repository interface {
	Create(ctx context.Context, a Application) (Application, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Application, error)
	List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Application, int, error)
	ListByCandidate(ctx context.Context, candidateID uuid.UUID) ([]Application, error)
	// ListByAccountForPortal returns the candidate-facing application history for a
	// portal account, aggregated across every linked candidate row (newest first).
	// Projection is minimal + non-sensitive (no AI score / internal fields): the
	// position title, status, applied-at, and the opaque public status token so the
	// portal can deep-link each row to /status.
	ListByAccountForPortal(ctx context.Context, accountID uuid.UUID) ([]PortalApplication, error)
	// PortalTimelineByToken returns the account-scoped status timeline input for a
	// single application identified by its public token. Returns ErrNotFound when
	// the token is unknown OR the application is not owned by accountID (no IDOR).
	PortalTimelineByToken(ctx context.Context, token string, accountID uuid.UUID) (*PortalTimeline, error)
	SetRawFile(ctx context.Context, id uuid.UUID, blobURL string) error
	SetQueueTaskID(ctx context.Context, id uuid.UUID, taskID string) error
	SetStatus(ctx context.Context, id uuid.UUID, status string) error
	// SetRejection sets status=rejected with an internal reason (never sent to the
	// candidate). The reason is mandatory at the handler layer.
	SetRejection(ctx context.Context, id uuid.UUID, reason string) error
	SetParseResults(ctx context.Context, id uuid.UUID, r ParseResult) error
	// Interview appointments (human interview scheduling, state-machine feature).
	// CreateAppointment assigns the next round_no for the application atomically.
	CreateAppointment(ctx context.Context, a Appointment) (Appointment, error)
	// FindAppointment returns the latest round's appointment (nil if none).
	FindAppointment(ctx context.Context, applicationID uuid.UUID) (*Appointment, error)
	// ListAppointments returns every round for an application, ordered by round_no.
	ListAppointments(ctx context.Context, applicationID uuid.UUID) ([]Appointment, error)
	// ListUpcomingInterviews returns scheduled interviews across applications for
	// the HR calendar, role-scoped (the cross-store privacy boundary).
	ListUpcomingInterviews(ctx context.Context, f UpcomingFilter, scope rbac.Scope) ([]UpcomingInterview, int, error)
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
	// SetVacancy links an application to the open vacancy it was matched to, so the
	// requisition scope can resolve application → vacancy → owning hiring manager.
	SetVacancy(ctx context.Context, id uuid.UUID, vacancyID *uuid.UUID) error
	// ReleaseStalePoolCandidates moves store-specific applications that no store HR
	// picked up within graceDays back to the central pool. Returns the count moved.
	ReleaseStalePoolCandidates(ctx context.Context, graceDays int) (int, error)
	// MarkPickedUp stamps the first store-HR pickup on a candidate's still-unpicked
	// store-specific applications, stopping their pool-release timer. Idempotent.
	MarkPickedUp(ctx context.Context, candidateID, byUser uuid.UUID) (int, error)
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
	// Letter generation (Module-3 3.3): interview/offer PDF letters.
	GatherLetterData(ctx context.Context, applicationID uuid.UUID, letterType string) (letters.LetterData, error)
	UpsertLetter(ctx context.Context, applicationID, createdBy uuid.UUID, letterType, blobURL string) (Letter, error)
	GetLettersByApplication(ctx context.Context, applicationID uuid.UUID) ([]Letter, error)
	GetLetterByID(ctx context.Context, id uuid.UUID) (*Letter, error)
	ListLettersByAccount(ctx context.Context, accountID uuid.UUID) ([]Letter, error)
	// Onboarding documents (Module-3 3.8): one document per (application, doc_type),
	// candidate-uploaded, HR-reviewed. ReviewOnboardingDocument is a single
	// transaction; FindHiredApplicationByAccount scopes the candidate endpoints
	// server-side so the client never passes an application id.
	UpsertOnboardingDocument(ctx context.Context, applicationID uuid.UUID, docType, blobURL, fileName, fileType string, uploadedBy uuid.UUID) (OnboardingDocument, error)
	ListOnboardingByApplication(ctx context.Context, applicationID uuid.UUID) ([]OnboardingDocument, error)
	GetOnboardingDocByID(ctx context.Context, id uuid.UUID) (*OnboardingDocument, error)
	ReviewOnboardingDocument(ctx context.Context, docID, applicationID, reviewerID uuid.UUID, approve bool, reason string) (OnboardingDocument, error)
	FindHiredApplicationByAccount(ctx context.Context, accountID uuid.UUID) (uuid.UUID, error)
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
		       ai_suggested_positions, COALESCE(rejection_reason,''),
		       COALESCE(public_token,'')
		FROM applications WHERE id = $1`
	var a Application
	var breakdownRaw, suggestedRaw []byte
	err := r.pool.QueryRow(ctx, q, id).Scan(
		&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
		&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
		&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt,
		&a.AIScore, &a.MustHavePassed, &a.AssignedStoreID, &a.TalentPool, &a.DedupState, &a.CreatedAt,
		&breakdownRaw, &a.AISummary, &a.AIRedFlags, &suggestedRaw, &a.RejectionReason,
		&a.PublicToken,
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
	// round_no is assigned atomically as max+1 for the application; the unique
	// index (application_id, round_no) is the backstop against a concurrent double.
	const q = `
		INSERT INTO interview_appointments
			(application_id, round_no, scheduled_at, duration_min, mode, location_text, online_join_url, calendar_event_id, created_by)
		VALUES (
			$1,
			(SELECT COALESCE(MAX(round_no), 0) + 1 FROM interview_appointments WHERE application_id = $1),
			$2, $3, $4, NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), $8
		)
		RETURNING id, round_no, created_at`
	err := r.pool.QueryRow(ctx, q,
		a.ApplicationID, a.ScheduledAt, a.DurationMin, a.Mode,
		a.LocationText, a.OnlineJoinURL, a.CalendarEventID, a.CreatedBy,
	).Scan(&a.ID, &a.RoundNo, &a.CreatedAt)
	if err != nil {
		return Appointment{}, fmt.Errorf("applications: create appointment: %w", err)
	}
	return a, nil
}

const appointmentCols = `id, application_id, round_no, scheduled_at, duration_min, mode,
		       COALESCE(location_text,''), COALESCE(online_join_url,''),
		       COALESCE(calendar_event_id,''), created_at`

func scanAppointment(row pgx.Row, a *Appointment) error {
	return row.Scan(
		&a.ID, &a.ApplicationID, &a.RoundNo, &a.ScheduledAt, &a.DurationMin, &a.Mode,
		&a.LocationText, &a.OnlineJoinURL, &a.CalendarEventID, &a.CreatedAt,
	)
}

func (r *pgRepository) FindAppointment(ctx context.Context, applicationID uuid.UUID) (*Appointment, error) {
	q := `SELECT ` + appointmentCols + `
		FROM interview_appointments
		WHERE application_id = $1
		ORDER BY round_no DESC
		LIMIT 1`
	var a Appointment
	err := scanAppointment(r.pool.QueryRow(ctx, q, applicationID), &a)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no appointment yet — not an error
	}
	if err != nil {
		return nil, fmt.Errorf("applications: find appointment: %w", err)
	}
	return &a, nil
}

func (r *pgRepository) ListAppointments(ctx context.Context, applicationID uuid.UUID) ([]Appointment, error) {
	q := `SELECT ` + appointmentCols + `
		FROM interview_appointments
		WHERE application_id = $1
		ORDER BY round_no ASC`
	rows, err := r.pool.Query(ctx, q, applicationID)
	if err != nil {
		return nil, fmt.Errorf("applications: list appointments: %w", err)
	}
	defer rows.Close()

	var out []Appointment
	for rows.Next() {
		var a Appointment
		if err := scanAppointment(rows, &a); err != nil {
			return nil, fmt.Errorf("applications: scan appointment: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListUpcomingInterviews returns scheduled interviews (>= From) joined with
// candidate/position/store for the HR calendar, role-scoped via the same
// ApplicationsClause the inbox uses. Optional To window + Mine (by created_by).
// Returns the page + total count for paging. The scope clause's bare
// assigned_store_id is unambiguous here (only `applications` has that column).
func (r *pgRepository) ListUpcomingInterviews(ctx context.Context, f UpcomingFilter, scope rbac.Scope) ([]UpcomingInterview, int, error) {
	args := []any{f.From}
	where := "ia.scheduled_at >= $1"
	if f.To != nil {
		args = append(args, *f.To)
		where += fmt.Sprintf(" AND ia.scheduled_at <= $%d", len(args))
	}
	if f.Mine {
		args = append(args, f.ActorID)
		where += fmt.Sprintf(" AND ia.created_by = $%d", len(args))
	}
	if clause, cargs := scope.ApplicationsClause(len(args) + 1); clause != "" {
		where += " AND " + clause
		args = append(args, cargs...)
	}

	var total int
	countQ := `SELECT COUNT(*) FROM interview_appointments ia
		JOIN applications a ON a.id = ia.application_id WHERE ` + where
	if err := r.pool.QueryRow(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("applications: count upcoming interviews: %w", err)
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	page := f.Page
	if page < 1 {
		page = 1
	}
	args = append(args, limit)
	limitPH := len(args)
	args = append(args, (page-1)*limit)
	offsetPH := len(args)

	q := `SELECT ia.id, ia.application_id, ia.round_no, ia.scheduled_at, ia.duration_min, ia.mode,
	             COALESCE(ia.location_text,''), COALESCE(ia.online_join_url,''),
	             COALESCE(c.full_name,''), COALESCE(NULLIF(p.title_en,''), p.title_th, ''),
	             COALESCE(s.store_name,''), a.assigned_store_id
	      FROM interview_appointments ia
	      JOIN applications a ON a.id = ia.application_id
	      JOIN candidates c ON c.id = a.candidate_id
	      LEFT JOIN positions p ON p.id = a.position_id
	      LEFT JOIN stores s ON s.store_no = a.assigned_store_id
	      WHERE ` + where + fmt.Sprintf(" ORDER BY ia.scheduled_at ASC LIMIT $%d OFFSET $%d", limitPH, offsetPH)
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("applications: list upcoming interviews: %w", err)
	}
	defer rows.Close()

	out := make([]UpcomingInterview, 0)
	for rows.Next() {
		var it UpcomingInterview
		if err := rows.Scan(
			&it.ID, &it.ApplicationID, &it.RoundNo, &it.ScheduledAt, &it.DurationMin, &it.Mode,
			&it.LocationText, &it.OnlineJoinURL, &it.CandidateName, &it.PositionTitle,
			&it.StoreName, &it.AssignedStoreID,
		); err != nil {
			return nil, 0, fmt.Errorf("applications: scan upcoming interview: %w", err)
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
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

func (r *pgRepository) SetVacancy(ctx context.Context, id uuid.UUID, vacancyID *uuid.UUID) error {
	const q = `UPDATE applications SET vacancy_id = $2, updated_at = NOW() WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, vacancyID); err != nil {
		return fmt.Errorf("applications: set vacancy: %w", err)
	}
	return nil
}

func (r *pgRepository) ReleaseStalePoolCandidates(ctx context.Context, graceDays int) (int, error) {
	const q = `
		UPDATE applications
		SET assigned_store_id = NULL, talent_pool = TRUE, released_to_pool_at = now(), updated_at = now()
		WHERE assigned_store_id IS NOT NULL
		  AND talent_pool = FALSE
		  AND picked_up_at IS NULL
		  AND released_to_pool_at IS NULL
		  AND created_at < now() - make_interval(days => $1)`
	tag, err := r.pool.Exec(ctx, q, graceDays)
	if err != nil {
		return 0, fmt.Errorf("applications: release stale pool candidates: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *pgRepository) MarkPickedUp(ctx context.Context, candidateID, byUser uuid.UUID) (int, error) {
	const q = `
		UPDATE applications
		SET picked_up_at = now(), picked_up_by = $2, updated_at = now()
		WHERE candidate_id = $1
		  AND assigned_store_id IS NOT NULL
		  AND talent_pool = FALSE
		  AND picked_up_at IS NULL`
	tag, err := r.pool.Exec(ctx, q, candidateID, byUser)
	if err != nil {
		return 0, fmt.Errorf("applications: mark picked up: %w", err)
	}
	return int(tag.RowsAffected()), nil
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
