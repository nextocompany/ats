package reports

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/nexto/hr-ats/internal/rbac"
)

// ATS Reports (Module-3 3.9). Operational hiring-funnel metrics over the Module-3
// lifecycle, RBAC-scoped (store/subregion/all) and filtered by a date range. All
// read-only aggregation over existing tables — no new schema. Each section keys off
// its natural event timestamp; durations are in days; rates are 0 when the
// denominator is 0 (no divide-by-zero).

// ATSFilter is the half-open [From, To) reporting window.
type ATSFilter struct {
	From time.Time
	To   time.Time
}

// ATSFunnelStage is one reached-funnel stage with conversion vs the previous stage.
type ATSFunnelStage struct {
	Key           string  `json:"key"`
	Count         int     `json:"count"`
	ConversionPct float64 `json:"conversion_pct"` // vs previous stage; first stage = 100
}

// ATSFunnel is the reached-funnel (event-flag based, monotonic).
type ATSFunnel struct {
	Stages []ATSFunnelStage `json:"stages"`
}

// ATSTiming is time-to-hire + stage timing (days).
type ATSTiming struct {
	HiredCount           int     `json:"hired_count"`
	AvgDaysToHire        float64 `json:"avg_days_to_hire"`
	MedianDaysToHire     float64 `json:"median_days_to_hire"`
	AvgDaysToOffer       float64 `json:"avg_days_to_offer"`
	AvgOfferResponseDays float64 `json:"avg_offer_response_days"`
}

// ATSOfferOutcomes is offer accept/decline performance.
type ATSOfferOutcomes struct {
	Sent           int     `json:"sent"`
	Accepted       int     `json:"accepted"`
	Declined       int     `json:"declined"`
	AcceptRatePct  float64 `json:"accept_rate_pct"`  // accepted / (accepted+declined)
	DeclineRatePct float64 `json:"decline_rate_pct"` // declined / (accepted+declined)
}

// ATSOnboarding is onboarding-document completion + rejection.
type ATSOnboarding struct {
	HiredInRange        int     `json:"hired_in_range"`
	Completed           int     `json:"completed"`
	CompletionRatePct   float64 `json:"completion_rate_pct"`
	DocsReviewed        int     `json:"docs_reviewed"`
	DocsRejected        int     `json:"docs_rejected"`
	DocRejectionRatePct float64 `json:"doc_rejection_rate_pct"`
}

// ATSQuality is interview + approval quality.
type ATSQuality struct {
	InterviewFeedbackCount int     `json:"interview_feedback_count"`
	InterviewPassed        int     `json:"interview_passed"`
	InterviewPassRatePct   float64 `json:"interview_pass_rate_pct"`
	AvgInterviewRating     float64 `json:"avg_interview_rating"` // 1..5
	ApprovalDecided        int     `json:"approval_decided"`
	AvgApprovalCycleDays   float64 `json:"avg_approval_cycle_days"`
	ApprovalSteps          int     `json:"approval_steps"`
	ApprovalBreached       int     `json:"approval_breached"`
	ApprovalSLABreachPct   float64 `json:"approval_sla_breach_pct"`
}

// ATSReport is the composite report for one scope + window.
type ATSReport struct {
	From       time.Time        `json:"from"`
	To         time.Time        `json:"to"`
	Scope      string           `json:"scope"` // human label: Company / Subregion: X / Store <id>
	Funnel     ATSFunnel        `json:"funnel"`
	Timing     ATSTiming        `json:"timing"`
	Offers     ATSOfferOutcomes `json:"offers"`
	Onboarding ATSOnboarding    `json:"onboarding"`
	Quality    ATSQuality       `json:"quality"`
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// pct is part/total as a 1-dp percentage, 0 when total is 0 (no divide-by-zero).
func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return round1(float64(part) / float64(total) * 100)
}

// scopedWhere appends the RBAC applications clause to a base WHERE condition,
// numbering scope placeholders from scopeArgStart, and returns the combined args.
func scopedWhere(base string, scope rbac.Scope, fixed []any, scopeArgStart int) (string, []any) {
	clause, sargs := scope.ApplicationsClause(scopeArgStart)
	if clause != "" {
		base += " AND " + clause
	}
	return base, append(fixed, sargs...)
}

// ATSReport assembles the full report for a scope + window. requiredDocs is the
// configured onboarding required-document set (slice 3.8) used for completion.
func (r *Repo) ATSReport(ctx context.Context, scope rbac.Scope, f ATSFilter, requiredDocs []string) (ATSReport, error) {
	rep := ATSReport{From: f.From, To: f.To}

	funnel, err := r.atsFunnel(ctx, scope, f)
	if err != nil {
		return ATSReport{}, err
	}
	rep.Funnel = funnel

	if err := r.atsTimingAndOffers(ctx, scope, f, &rep); err != nil {
		return ATSReport{}, err
	}
	if err := r.atsOnboarding(ctx, scope, f, requiredDocs, &rep); err != nil {
		return ATSReport{}, err
	}
	if err := r.atsQuality(ctx, scope, f, &rep); err != nil {
		return ATSReport{}, err
	}
	return rep, nil
}

// atsFunnel counts applications (created in range) that reached each milestone via
// event flags: screened (scored), interview (appointment), offer (sent), hired.
func (r *Repo) atsFunnel(ctx context.Context, scope rbac.Scope, f ATSFilter) (ATSFunnel, error) {
	where, args := scopedWhere("a.created_at >= $1 AND a.created_at < $2", scope, []any{f.From, f.To}, 3)
	q := `
		SELECT
			COUNT(*) AS applied,
			COUNT(*) FILTER (WHERE a.ai_score IS NOT NULL OR a.must_have_passed IS NOT NULL) AS screened,
			COUNT(*) FILTER (WHERE EXISTS (SELECT 1 FROM interview_appointments ia WHERE ia.application_id = a.id)) AS interviewed,
			COUNT(*) FILTER (WHERE EXISTS (SELECT 1 FROM offers o WHERE o.application_id = a.id AND o.sent_at IS NOT NULL)) AS offered,
			COUNT(*) FILTER (WHERE a.hired_at IS NOT NULL) AS hired
		FROM applications a
		WHERE ` + where
	var applied, screened, interviewed, offered, hired int
	if err := r.pool.QueryRow(ctx, q, args...).Scan(&applied, &screened, &interviewed, &offered, &hired); err != nil {
		return ATSFunnel{}, fmt.Errorf("reports: ats funnel: %w", err)
	}
	stages := []struct {
		key   string
		count int
	}{
		{"applied", applied},
		{"screened", screened},
		{"interview", interviewed},
		{"offer", offered},
		{"hired", hired},
	}
	out := make([]ATSFunnelStage, 0, len(stages))
	for i, s := range stages {
		conv := 100.0
		if i > 0 {
			conv = pct(s.count, stages[i-1].count)
		}
		out = append(out, ATSFunnelStage{Key: s.key, Count: s.count, ConversionPct: conv})
	}
	return ATSFunnel{Stages: out}, nil
}

// atsTimingAndOffers fills Timing (time-to-hire) + Offers (accept/decline) and the
// offer-derived timing (days-to-offer, offer-response).
func (r *Repo) atsTimingAndOffers(ctx context.Context, scope rbac.Scope, f ATSFilter, rep *ATSReport) error {
	// Time-to-hire over applications hired in range.
	whereHired, hiredArgs := scopedWhere("a.hired_at >= $1 AND a.hired_at < $2", scope, []any{f.From, f.To}, 3)
	qHire := `
		SELECT
			COUNT(*) AS n,
			COALESCE(AVG(EXTRACT(EPOCH FROM (a.hired_at - a.created_at)) / 86400.0), 0) AS avg_days,
			COALESCE(percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (a.hired_at - a.created_at)) / 86400.0), 0) AS median_days
		FROM applications a
		WHERE ` + whereHired
	if err := r.pool.QueryRow(ctx, qHire, hiredArgs...).
		Scan(&rep.Timing.HiredCount, &rep.Timing.AvgDaysToHire, &rep.Timing.MedianDaysToHire); err != nil {
		return fmt.Errorf("reports: ats time-to-hire: %w", err)
	}
	rep.Timing.AvgDaysToHire = round1(rep.Timing.AvgDaysToHire)
	rep.Timing.MedianDaysToHire = round1(rep.Timing.MedianDaysToHire)

	// Offers sent in range (scope via the owning application).
	whereOffer, offerArgs := scopedWhere("o.sent_at >= $1 AND o.sent_at < $2", scope, []any{f.From, f.To}, 3)
	qOffer := `
		SELECT
			COUNT(*) AS sent,
			COUNT(*) FILTER (WHERE o.status = 'accepted') AS accepted,
			COUNT(*) FILTER (WHERE o.status = 'declined') AS declined,
			COALESCE(AVG(EXTRACT(EPOCH FROM (o.responded_at - o.sent_at)) / 86400.0) FILTER (WHERE o.responded_at IS NOT NULL), 0) AS avg_response,
			COALESCE(AVG(EXTRACT(EPOCH FROM (o.sent_at - a.created_at)) / 86400.0), 0) AS avg_to_offer
		FROM offers o
		JOIN applications a ON a.id = o.application_id
		WHERE ` + whereOffer
	if err := r.pool.QueryRow(ctx, qOffer, offerArgs...).Scan(
		&rep.Offers.Sent, &rep.Offers.Accepted, &rep.Offers.Declined,
		&rep.Timing.AvgOfferResponseDays, &rep.Timing.AvgDaysToOffer); err != nil {
		return fmt.Errorf("reports: ats offers: %w", err)
	}
	rep.Timing.AvgOfferResponseDays = round1(rep.Timing.AvgOfferResponseDays)
	rep.Timing.AvgDaysToOffer = round1(rep.Timing.AvgDaysToOffer)
	responded := rep.Offers.Accepted + rep.Offers.Declined
	rep.Offers.AcceptRatePct = pct(rep.Offers.Accepted, responded)
	rep.Offers.DeclineRatePct = pct(rep.Offers.Declined, responded)
	return nil
}

// atsOnboarding fills onboarding completion (hired in range, vs required set) +
// document rejection rate (docs uploaded in range).
func (r *Repo) atsOnboarding(ctx context.Context, scope rbac.Scope, f ATSFilter, requiredDocs []string, rep *ATSReport) error {
	requiredCount := len(requiredDocs)
	// Completion: hired apps in range; an app is complete when it has an approved
	// document for at least every required doc_type.
	whereHired, compArgs := scopedWhere(
		"a.hired_at >= $1 AND a.hired_at < $2",
		scope, []any{f.From, f.To, requiredCount, requiredDocs}, 5)
	qComp := `
		SELECT
			COUNT(*) AS hired,
			COUNT(*) FILTER (WHERE COALESCE(ac.approved_required, 0) >= $3) AS completed
		FROM applications a
		LEFT JOIN (
			SELECT application_id, COUNT(DISTINCT doc_type) AS approved_required
			FROM onboarding_documents
			WHERE status = 'approved' AND doc_type = ANY($4)
			GROUP BY application_id
		) ac ON ac.application_id = a.id
		WHERE ` + whereHired
	if err := r.pool.QueryRow(ctx, qComp, compArgs...).Scan(&rep.Onboarding.HiredInRange, &rep.Onboarding.Completed); err != nil {
		return fmt.Errorf("reports: ats onboarding completion: %w", err)
	}
	// An empty required set can never be "complete" — report 0 (mirrors 3.8).
	if requiredCount == 0 {
		rep.Onboarding.Completed = 0
	}
	rep.Onboarding.CompletionRatePct = pct(rep.Onboarding.Completed, rep.Onboarding.HiredInRange)

	// Document rejection over documents uploaded in range.
	whereDocs, docArgs := scopedWhere("od.uploaded_at >= $1 AND od.uploaded_at < $2", scope, []any{f.From, f.To}, 3)
	qDocs := `
		SELECT
			COUNT(*) FILTER (WHERE od.status IN ('approved','rejected')) AS reviewed,
			COUNT(*) FILTER (WHERE od.status = 'rejected') AS rejected
		FROM onboarding_documents od
		JOIN applications a ON a.id = od.application_id
		WHERE ` + whereDocs
	if err := r.pool.QueryRow(ctx, qDocs, docArgs...).Scan(&rep.Onboarding.DocsReviewed, &rep.Onboarding.DocsRejected); err != nil {
		return fmt.Errorf("reports: ats doc rejection: %w", err)
	}
	rep.Onboarding.DocRejectionRatePct = pct(rep.Onboarding.DocsRejected, rep.Onboarding.DocsReviewed)
	return nil
}

// atsQuality fills interview pass-rate/avg-rating + approval cycle-time/SLA-breach.
func (r *Repo) atsQuality(ctx context.Context, scope rbac.Scope, f ATSFilter, rep *ATSReport) error {
	// Interview feedback recorded in range.
	whereFb, fbArgs := scopedWhere("f.created_at >= $1 AND f.created_at < $2", scope, []any{f.From, f.To}, 3)
	qFb := `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE f.recommendation = 'pass') AS passed,
			COALESCE(AVG(f.overall_rating), 0) AS avg_rating
		FROM interview_feedback f
		JOIN applications a ON a.id = f.application_id
		WHERE ` + whereFb
	if err := r.pool.QueryRow(ctx, qFb, fbArgs...).
		Scan(&rep.Quality.InterviewFeedbackCount, &rep.Quality.InterviewPassed, &rep.Quality.AvgInterviewRating); err != nil {
		return fmt.Errorf("reports: ats interview quality: %w", err)
	}
	rep.Quality.InterviewPassRatePct = pct(rep.Quality.InterviewPassed, rep.Quality.InterviewFeedbackCount)
	rep.Quality.AvgInterviewRating = round1(rep.Quality.AvgInterviewRating)

	// Approval requests created in range (cycle time of decided requests).
	whereReq, reqArgs := scopedWhere("ar.created_at >= $1 AND ar.created_at < $2", scope, []any{f.From, f.To}, 3)
	qReq := `
		SELECT
			COUNT(*) FILTER (WHERE ar.decided_at IS NOT NULL) AS decided,
			COALESCE(AVG(EXTRACT(EPOCH FROM (ar.decided_at - ar.created_at)) / 86400.0) FILTER (WHERE ar.decided_at IS NOT NULL), 0) AS avg_cycle
		FROM approval_requests ar
		JOIN applications a ON a.id = ar.application_id
		WHERE ` + whereReq
	if err := r.pool.QueryRow(ctx, qReq, reqArgs...).Scan(&rep.Quality.ApprovalDecided, &rep.Quality.AvgApprovalCycleDays); err != nil {
		return fmt.Errorf("reports: ats approval cycle: %w", err)
	}
	rep.Quality.AvgApprovalCycleDays = round1(rep.Quality.AvgApprovalCycleDays)

	// Approval steps created in range (SLA-breach = escalated).
	whereStep, stepArgs := scopedWhere("s.created_at >= $1 AND s.created_at < $2", scope, []any{f.From, f.To}, 3)
	qStep := `
		SELECT COUNT(*) AS total, COUNT(*) FILTER (WHERE s.escalated) AS breached
		FROM approval_steps s
		JOIN approval_requests ar ON ar.id = s.request_id
		JOIN applications a ON a.id = ar.application_id
		WHERE ` + whereStep
	if err := r.pool.QueryRow(ctx, qStep, stepArgs...).Scan(&rep.Quality.ApprovalSteps, &rep.Quality.ApprovalBreached); err != nil {
		return fmt.Errorf("reports: ats approval sla: %w", err)
	}
	rep.Quality.ApprovalSLABreachPct = pct(rep.Quality.ApprovalBreached, rep.Quality.ApprovalSteps)
	return nil
}
