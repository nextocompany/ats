package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// Compare candidates for one position: rank the post-AI-interview pool by a
// composite of the screening score and the AI pre-interview score, for a
// side-by-side hiring decision. Distinct from the shortlist (which is
// status='shortlisted' only and blends screening with the HUMAN TA rating).

// compare composite weights — screening and AI interview count equally.
const (
	compareScreeningWeight = 0.5
	compareInterviewWeight = 0.5
)

// eligibleCompareStatuses are the application statuses that guarantee BOTH the
// screening gate was passed (scored) AND the AI pre-interview was completed
// (ai_interviewed). Derived from the state machine in transitions.go: shortlisted
// / interview / ... are reachable only via ai_interviewed, and the machine cannot
// be bypassed. scored and ai_interview (invited, not completed) are excluded.
var eligibleCompareStatuses = []string{
	StatusAIInterviewed,
	StatusShortlisted,
	StatusInterview,
	StatusInterviewed,
	StatusPendingApproval,
	StatusOffer,
	StatusHired,
}

// CompareScore blends the screening composite (0..100) and the AI pre-interview
// score (0..100) 50/50. Both candidates in a comparison have completed both
// stages, so an equal blend reflects overall standing.
func CompareScore(screening, interview float64) float64 {
	return round1(screening*compareScreeningWeight + interview*compareInterviewWeight)
}

// CompareItem is one ranked candidate in the per-position comparison. It carries
// both AI scores, the per-dimension screening breakdown, the AI interview
// recommendation, and the risk signals needed for a side-by-side decision.
type CompareItem struct {
	ApplicationID    uuid.UUID       `json:"application_id"`
	CandidateName    string          `json:"candidate_name"`
	Status           string          `json:"status"`
	AppliedAt        time.Time       `json:"applied_at"`
	AssignedStoreID  *int            `json:"assigned_store_id"`
	StoreName        string          `json:"store_name"`
	ScreeningScore   *float64        `json:"screening_score"` // applications.ai_score
	Breakdown        *ScoreBreakdown `json:"breakdown,omitempty"`
	MustHavePassed   *bool           `json:"must_have_passed"`
	AISummary        string          `json:"ai_summary,omitempty"`
	AIRedFlags       string          `json:"ai_red_flags,omitempty"`
	InterviewScore   *float64        `json:"interview_score"` // interview_sessions.interview_score
	Recommendation   string          `json:"recommendation"`  // strong_recommend|recommend|neutral|caution
	InterviewSummary string          `json:"interview_summary,omitempty"`
	Composite        float64         `json:"composite"`
}

// CompareByPosition returns the eligible candidates for one position, ranked by
// the screening+AI-interview composite (highest first), scoped to what the caller
// may see. The INNER JOIN on a completed interview session is the eligibility
// guard: it both confirms the AI interview is done and guarantees interview_score
// is present for the composite.
func (r *pgRepository) CompareByPosition(ctx context.Context, positionID uuid.UUID, scope rbac.Scope, limit int) ([]CompareItem, error) {
	if limit <= 0 {
		limit = 50
	}
	args := []any{positionID, eligibleCompareStatuses}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }

	// c.is_duplicate_of IS NULL excludes deduped candidate rows so a person never
	// double-appears in the ranking (mirrors the /candidates roster filter).
	where := "a.position_id = $1 AND a.status = ANY($2) AND c.is_duplicate_of IS NULL"
	if clause, cargs := scope.ApplicationsClause(len(args) + 1); clause != "" {
		where += " AND " + clause
		args = append(args, cargs...)
	}
	limitPH := add(limit)

	q := `
		SELECT a.id, COALESCE(c.full_name, ''), a.status, a.created_at, a.assigned_store_id,
		       COALESCE(st.store_name, ''), a.ai_score, a.ai_score_breakdown, a.must_have_passed,
		       COALESCE(a.ai_summary, ''), COALESCE(a.ai_red_flags, ''),
		       s.interview_score, COALESCE(s.recommendation, ''), COALESCE(s.summary, '')
		FROM applications a
		JOIN candidates c ON c.id = a.candidate_id
		JOIN interview_sessions s ON s.application_id = a.id AND s.status = 'completed'
		LEFT JOIN stores st ON st.store_no = a.assigned_store_id
		WHERE ` + where + `
		ORDER BY (COALESCE(a.ai_score,0)*` + fmt.Sprintf("%g", compareScreeningWeight) +
		` + COALESCE(s.interview_score,0)*` + fmt.Sprintf("%g", compareInterviewWeight) + `) DESC,
		         a.ai_score DESC NULLS LAST, a.created_at DESC
		LIMIT ` + limitPH
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("applications: compare by position: %w", err)
	}
	defer rows.Close()

	out := make([]CompareItem, 0)
	for rows.Next() {
		var it CompareItem
		var breakdownRaw []byte
		if err := rows.Scan(
			&it.ApplicationID, &it.CandidateName, &it.Status, &it.AppliedAt, &it.AssignedStoreID,
			&it.StoreName, &it.ScreeningScore, &breakdownRaw, &it.MustHavePassed,
			&it.AISummary, &it.AIRedFlags,
			&it.InterviewScore, &it.Recommendation, &it.InterviewSummary,
		); err != nil {
			return nil, fmt.Errorf("applications: scan compare: %w", err)
		}
		if len(breakdownRaw) > 0 {
			var bd ScoreBreakdown
			if jsonErr := json.Unmarshal(breakdownRaw, &bd); jsonErr == nil {
				it.Breakdown = &bd
			}
		}
		screening, interview := 0.0, 0.0
		if it.ScreeningScore != nil {
			screening = *it.ScreeningScore
		}
		if it.InterviewScore != nil {
			interview = *it.InterviewScore
		}
		it.Composite = CompareScore(screening, interview)
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("applications: iterate compare: %w", err)
	}
	return out, nil
}
