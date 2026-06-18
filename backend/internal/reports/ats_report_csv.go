package reports

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
)

func csvInt(n int) string    { return strconv.Itoa(n) }
func csvF1(f float64) string { return strconv.FormatFloat(f, 'f', 1, 64) }

// EncodeATSCSV flattens an ATS report into section,metric,value rows (same shape
// family as the snapshot EncodeCSV) for a synchronous CSV download.
func EncodeATSCSV(rep ATSReport) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	rows := [][]string{
		{"section", "metric", "value"},
		{"meta", "scope", rep.Scope},
		{"meta", "from", rep.From.UTC().Format("2006-01-02")},
		{"meta", "to", rep.To.UTC().Format("2006-01-02")},
	}
	for _, s := range rep.Funnel.Stages {
		rows = append(rows,
			[]string{"funnel:" + s.Key, "count", csvInt(s.Count)},
			[]string{"funnel:" + s.Key, "conversion_pct", csvF1(s.ConversionPct)},
		)
	}
	rows = append(rows,
		[]string{"timing", "hired_count", csvInt(rep.Timing.HiredCount)},
		[]string{"timing", "avg_days_to_hire", csvF1(rep.Timing.AvgDaysToHire)},
		[]string{"timing", "median_days_to_hire", csvF1(rep.Timing.MedianDaysToHire)},
		[]string{"timing", "avg_days_to_offer", csvF1(rep.Timing.AvgDaysToOffer)},
		[]string{"timing", "avg_offer_response_days", csvF1(rep.Timing.AvgOfferResponseDays)},
		[]string{"offers", "sent", csvInt(rep.Offers.Sent)},
		[]string{"offers", "accepted", csvInt(rep.Offers.Accepted)},
		[]string{"offers", "declined", csvInt(rep.Offers.Declined)},
		[]string{"offers", "accept_rate_pct", csvF1(rep.Offers.AcceptRatePct)},
		[]string{"offers", "decline_rate_pct", csvF1(rep.Offers.DeclineRatePct)},
		[]string{"onboarding", "hired_in_range", csvInt(rep.Onboarding.HiredInRange)},
		[]string{"onboarding", "completed", csvInt(rep.Onboarding.Completed)},
		[]string{"onboarding", "completion_rate_pct", csvF1(rep.Onboarding.CompletionRatePct)},
		[]string{"onboarding", "docs_reviewed", csvInt(rep.Onboarding.DocsReviewed)},
		[]string{"onboarding", "docs_rejected", csvInt(rep.Onboarding.DocsRejected)},
		[]string{"onboarding", "doc_rejection_rate_pct", csvF1(rep.Onboarding.DocRejectionRatePct)},
		[]string{"quality", "interview_feedback_count", csvInt(rep.Quality.InterviewFeedbackCount)},
		[]string{"quality", "interview_passed", csvInt(rep.Quality.InterviewPassed)},
		[]string{"quality", "interview_pass_rate_pct", csvF1(rep.Quality.InterviewPassRatePct)},
		[]string{"quality", "avg_interview_rating", csvF1(rep.Quality.AvgInterviewRating)},
		[]string{"quality", "approval_decided", csvInt(rep.Quality.ApprovalDecided)},
		[]string{"quality", "avg_approval_cycle_days", csvF1(rep.Quality.AvgApprovalCycleDays)},
		[]string{"quality", "approval_steps", csvInt(rep.Quality.ApprovalSteps)},
		[]string{"quality", "approval_breached", csvInt(rep.Quality.ApprovalBreached)},
		[]string{"quality", "approval_sla_breach_pct", csvF1(rep.Quality.ApprovalSLABreachPct)},
	)

	if err := w.WriteAll(rows); err != nil {
		return nil, fmt.Errorf("reports: encode ats csv: %w", err)
	}
	return buf.Bytes(), nil
}
