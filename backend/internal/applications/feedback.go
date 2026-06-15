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

const (
	ratingMin = 1 // overall_rating range
	ratingMax = 5
	compMax   = 5 // per-competency range; 0 means "not rated"
)

// InterviewCompetencies are the per-dimension 1..5 ratings (0 = not rated). Stored
// as JSONB so the dimension set can evolve without a schema change (mirrors the way
// applications.ai_score_breakdown is persisted).
type InterviewCompetencies struct {
	Communication int `json:"communication"` // การสื่อสาร
	Technical     int `json:"technical"`     // ความรู้/ทักษะงาน
	Experience    int `json:"experience"`    // ประสบการณ์/ความเหมาะสมกับตำแหน่ง
	CultureFit    int `json:"culture_fit"`   // ทัศนคติ/วัฒนธรรมองค์กร
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
	for name, v := range map[string]int{
		"communication": f.Competencies.Communication,
		"technical":     f.Competencies.Technical,
		"experience":    f.Competencies.Experience,
		"culture_fit":   f.Competencies.CultureFit,
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
