package applications

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Interview feedback: the structured outcome a hiring panel records during the
// human-interview stage. Many rows per application (one per interviewer/round),
// recorded independently of the "mark interviewed" transition.

// Recommendation values (the interviewer's verdict).
const (
	RecPass = "pass" // ผ่าน
	RecHold = "hold" // รอพิจารณา
	RecFail = "fail" // ไม่ผ่าน
)

var validRecommendations = map[string]bool{RecPass: true, RecHold: true, RecFail: true}

// Scorecard perspective: who is assessing. TA (recruiter) and the Line Manager
// (sgm) each rate their own competency subset; both share overall_rating +
// recommendation. Legacy rows (pre-000021) default to "ta".
const (
	PerspectiveTA          = "ta"
	PerspectiveLineManager = "line_manager"
)

var validPerspectives = map[string]bool{PerspectiveTA: true, PerspectiveLineManager: true}

// ValidatePerspective reports whether p is a known scorecard perspective.
func ValidatePerspective(p string) bool { return validPerspectives[p] }

const (
	ratingMin = 1 // overall_rating range
	ratingMax = 5
	compMax   = 5 // per-competency range; 0 means "not rated"
)

// InterviewCompetencies are the per-dimension 1..5 ratings (0 = not rated). Stored
// as JSONB so the dimension set can evolve without a schema change (mirrors the way
// applications.ai_score_breakdown is persisted).
// Superset of all competency dimensions across perspectives (0 = not rated).
// TA rates communication/technical/attitude (+experience); the Line Manager rates
// culture_fit/growth_potential/leadership. Each perspective sends only its subset;
// the rest stay 0.
type InterviewCompetencies struct {
	Communication   int `json:"communication"`    // การสื่อสาร (TA)
	Technical       int `json:"technical"`        // ความรู้/ทักษะงาน (TA)
	Experience      int `json:"experience"`       // ประสบการณ์/ความเหมาะสมกับตำแหน่ง (TA)
	Attitude        int `json:"attitude"`         // ทัศนคติ (TA)
	CultureFit      int `json:"culture_fit"`      // วัฒนธรรมองค์กร (LM)
	GrowthPotential int `json:"growth_potential"` // ศักยภาพการเติบโต (LM)
	Leadership      int `json:"leadership"`       // ภาวะผู้นำ (LM)
}

// InterviewFeedback is one recorded interview outcome for an application.
type InterviewFeedback struct {
	ID            uuid.UUID  `json:"id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	AppointmentID *uuid.UUID `json:"appointment_id,omitempty"`
	InterviewerID *uuid.UUID `json:"-"` // FK; the display name is exposed instead
	// InterviewerName is joined from users on read (full_name, else email). It is
	// never accepted from the client — the server stamps the authenticated user.
	InterviewerName string                `json:"interviewer_name,omitempty"`
	Perspective     string                `json:"perspective"` // "ta" | "line_manager"
	OverallRating   int                   `json:"overall_rating"`
	Recommendation  string                `json:"recommendation"`
	Competencies    InterviewCompetencies `json:"competencies"`
	Strengths       string                `json:"strengths,omitempty"`
	Concerns        string                `json:"concerns,omitempty"`
	Notes           string                `json:"notes,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
}

// ValidateFeedback checks a feedback record before persistence. It enforces the
// rating ranges and a known recommendation; the free-text fields are unconstrained.
func ValidateFeedback(f InterviewFeedback) error {
	if f.OverallRating < ratingMin || f.OverallRating > ratingMax {
		return fmt.Errorf("overall_rating must be between %d and %d", ratingMin, ratingMax)
	}
	if !validRecommendations[f.Recommendation] {
		return fmt.Errorf("recommendation must be one of pass, hold, fail")
	}
	if f.Perspective != "" && !validPerspectives[f.Perspective] {
		return fmt.Errorf("perspective must be one of ta, line_manager")
	}
	for name, v := range map[string]int{
		"communication":    f.Competencies.Communication,
		"technical":        f.Competencies.Technical,
		"experience":       f.Competencies.Experience,
		"attitude":         f.Competencies.Attitude,
		"culture_fit":      f.Competencies.CultureFit,
		"growth_potential": f.Competencies.GrowthPotential,
		"leadership":       f.Competencies.Leadership,
	} {
		if v < 0 || v > compMax {
			return fmt.Errorf("competency %s must be between 0 and %d", name, compMax)
		}
	}
	return nil
}

// CanRecordFeedback reports whether feedback may be recorded for an application in
// the given status. Allowed while a human interview is active or just completed.
func CanRecordFeedback(status string) bool {
	return status == StatusInterview || status == StatusInterviewed
}
