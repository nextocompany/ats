package applications

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Default + max page size for the inbox.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ListFilter is the inbox query (all fields optional).
type ListFilter struct {
	Status        string
	SourceChannel string
	MinScore      *float64
	StoreID       *int
	From          *time.Time
	To            *time.Time
	Page          int
	Limit         int
}

// normalize clamps pagination into sane bounds.
func (f *ListFilter) normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
}

const appColumns = `
	id, candidate_id, position_id, status,
	COALESCE(raw_file_blob_url,''), COALESCE(raw_file_type,''),
	COALESCE(ocr_text_blob_url,''), COALESCE(parsed_profile_blob_url,''),
	ocr_confidence, COALESCE(needs_manual_review,false),
	COALESCE(queue_task_id,''), parsed_at,
	ai_score, must_have_passed, assigned_store_id,
	COALESCE(talent_pool,false), COALESCE(dedup_state,''), created_at`

// List returns a ranked (ai_score desc), filtered, role-scoped, paginated page
// of applications plus the total count for pagination metadata.
func (r *pgRepository) List(ctx context.Context, f ListFilter, scope rbac.Scope) ([]Application, int, error) {
	f.normalize()

	var args []any
	add := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	var conds []string
	if f.Status != "" {
		conds = append(conds, "status = "+add(f.Status))
	}
	if f.MinScore != nil {
		conds = append(conds, "ai_score >= "+add(*f.MinScore))
	}
	if f.StoreID != nil {
		conds = append(conds, "assigned_store_id = "+add(*f.StoreID))
	}
	if f.SourceChannel != "" {
		conds = append(conds, "candidate_id IN (SELECT id FROM candidates WHERE source_channel = "+add(f.SourceChannel)+")")
	}
	if f.From != nil {
		conds = append(conds, "created_at >= "+add(*f.From))
	}
	if f.To != nil {
		conds = append(conds, "created_at <= "+add(*f.To))
	}
	if sc, scArgs := scope.ApplicationsClause(len(args) + 1); sc != "" {
		conds = append(conds, sc)
		args = append(args, scArgs...)
	}

	where := ""
	if len(conds) > 0 {
		where = "WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM applications "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("applications: list count: %w", err)
	}

	limitPH := add(f.Limit)
	offsetPH := add((f.Page - 1) * f.Limit)
	q := "SELECT " + appColumns + " FROM applications " + where +
		" ORDER BY ai_score DESC NULLS LAST, created_at DESC LIMIT " + limitPH + " OFFSET " + offsetPH

	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("applications: list: %w", err)
	}
	defer rows.Close()

	var out []Application
	for rows.Next() {
		var a Application
		if err := rows.Scan(
			&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
			&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
			&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt,
			&a.AIScore, &a.MustHavePassed, &a.AssignedStoreID, &a.TalentPool, &a.DedupState, &a.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("applications: list scan: %w", err)
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// ListByCandidate returns all applications for a candidate, newest first.
func (r *pgRepository) ListByCandidate(ctx context.Context, candidateID uuid.UUID) ([]Application, error) {
	q := "SELECT " + appColumns + " FROM applications WHERE candidate_id = $1 ORDER BY created_at DESC"
	rows, err := r.pool.Query(ctx, q, candidateID)
	if err != nil {
		return nil, fmt.Errorf("applications: list by candidate: %w", err)
	}
	defer rows.Close()

	var out []Application
	for rows.Next() {
		var a Application
		if err := rows.Scan(
			&a.ID, &a.CandidateID, &a.PositionID, &a.Status,
			&a.RawFileBlobURL, &a.RawFileType, &a.OCRTextBlobURL, &a.ParsedProfileBlobURL,
			&a.OCRConfidence, &a.NeedsManualReview, &a.QueueTaskID, &a.ParsedAt,
			&a.AIScore, &a.MustHavePassed, &a.AssignedStoreID, &a.TalentPool, &a.DedupState, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("applications: list by candidate scan: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
