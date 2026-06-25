// Package apptimeline turns the raw application status history into a curated,
// candidate-facing milestone timeline.
//
// It is the privacy boundary for the status page: internal churn (parsed,
// scored, name_mismatch, …) collapses into a handful of friendly milestones,
// and nothing here ever carries a rejection reason, an AI score, or actor
// identity. Keep this package pure (stdlib only) so the curation logic stays
// unit-testable without a database.
package apptimeline

import "time"

// Milestone keys — the only status vocabulary the candidate ever sees.
const (
	KeyApplied      = "applied"       // application received
	KeyScreening    = "screening"     // qualifications under review (parsed/scored/shortlisted)
	KeyAIInterview  = "ai_interview"  // pre-interview assessment (ai_interview/ai_interviewed)
	KeyInterview    = "interview"     // human interview (interview/interviewed)
	KeyDecision     = "decision"      // hiring approval in progress (pending_approval)
	KeyOffer        = "offer"         // job offer extended
	KeyHired        = "hired"         // selected (terminal, positive)
	KeyNotSelected  = "not_selected"  // rejected (terminal, negative branch)
	KeyActionNeeded = "action_needed" // recoverable upload problem (invalid_resume/name_mismatch/failed)
)

// Milestone states.
const (
	StateDone     = "done"
	StateCurrent  = "current"
	StateUpcoming = "upcoming"
)

// happyPath is the ordered positive journey shown as a stepper.
var happyPath = []string{
	KeyApplied, KeyScreening, KeyAIInterview, KeyInterview, KeyDecision, KeyOffer, KeyHired,
}

// rawToMilestone maps every raw application status to its curated milestone.
// Statuses absent from this map fall back to KeyApplied (defensive; the status
// set is fixed in applications.model).
var rawToMilestone = map[string]string{
	"pending":          KeyApplied,
	"parsed":           KeyScreening,
	"scored":           KeyScreening,
	"shortlisted":      KeyScreening,
	"ai_interview":     KeyAIInterview,
	"ai_interviewed":   KeyAIInterview,
	"interview":        KeyInterview,
	"interviewed":      KeyInterview,
	"pending_approval": KeyDecision,
	"offer":            KeyOffer,
	"hired":            KeyHired,
	"rejected":         KeyNotSelected,
	"invalid_resume":   KeyActionNeeded,
	"name_mismatch":    KeyActionNeeded,
	"failed":           KeyActionNeeded,
}

// labels are the candidate-facing Thai copy per milestone.
var labels = map[string]string{
	KeyApplied:      "ได้รับใบสมัครแล้ว",
	KeyScreening:    "กำลังพิจารณาคุณสมบัติ",
	KeyAIInterview:  "แบบสัมภาษณ์เบื้องต้น",
	KeyInterview:    "นัดสัมภาษณ์",
	KeyDecision:     "อยู่ระหว่างพิจารณาอนุมัติ",
	KeyOffer:        "ข้อเสนอการจ้างงาน",
	KeyHired:        "ยินดีด้วย! คุณได้รับการคัดเลือก",
	KeyNotSelected:  "ยังไม่ผ่านการพิจารณาในรอบนี้",
	KeyActionNeeded: "ต้องดำเนินการเพิ่มเติม",
}

// details are the longer per-milestone explanation shown under the active or
// branch step. They carry the actionable guidance (especially the re-apply
// instruction for KeyActionNeeded) that the candidate must see; the timeline
// renders the detail directly rather than gating it behind a reached date.
var details = map[string]string{
	KeyApplied:      "เราได้รับใบสมัครของคุณแล้ว",
	KeyScreening:    "กำลังพิจารณาคุณสมบัติของคุณ",
	KeyAIInterview:  "ขั้นตอนแบบสัมภาษณ์เบื้องต้นกับผู้ช่วย AI",
	KeyInterview:    "ทีมงานจะติดต่อเพื่อนัดหมายสัมภาษณ์",
	KeyDecision:     "ใบสมัครของคุณอยู่ระหว่างการอนุมัติภายใน",
	KeyOffer:        "เข้าสู่ระบบเพื่อดูรายละเอียดและตอบรับข้อเสนอ",
	KeyHired:        "ทีม HR จะติดต่อเรื่องการเริ่มงาน",
	KeyNotSelected:  "ขอบคุณที่สนใจร่วมงานกับเรา เราจะเก็บข้อมูลไว้พิจารณาในโอกาสหน้า",
	KeyActionNeeded: "ไฟล์ที่อัปโหลดอาจไม่ใช่เรซูเม่ หรือชื่อไม่ตรงกับบัญชีของคุณ กรุณาสมัครใหม่อีกครั้งพร้อมแนบเรซูเม่ของคุณ",
}

// Event is one recorded status transition (the curation never sees from_status,
// rejection_reason, or who made the change).
type Event struct {
	To string
	At time.Time
}

// Milestone is one curated step in the candidate timeline.
type Milestone struct {
	Key       string     `json:"key"`
	Label     string     `json:"label"`
	Detail    string     `json:"detail"`
	ReachedAt *time.Time `json:"reached_at"`
	State     string     `json:"state"`
}

// Build curates the raw history into the candidate-facing timeline.
//
//   - events:     recorded status transitions (chronological order not required)
//   - createdAt:  the application's created_at, the synthetic "applied" timestamp
//     (the initial "pending" is an INSERT default, so the trigger records no
//     event for it)
//   - current:    the application's current status
//
// Happy-path journeys return the full ladder (done/current/upcoming). A terminal
// or recoverable branch (rejected / upload problem) returns only the steps
// actually reached plus the branch milestone, never dangling future steps.
func Build(events []Event, createdAt time.Time, current string) []Milestone {
	reached := reachedTimes(events, createdAt)

	cur, ok := rawToMilestone[current]
	if !ok {
		cur = KeyApplied
	}

	switch cur {
	case KeyNotSelected, KeyActionNeeded:
		return branchTimeline(reached, cur)
	default:
		return happyTimeline(reached, cur)
	}
}

// reachedTimes computes the earliest time each milestone was entered. "applied"
// is always present, dated from created_at.
func reachedTimes(events []Event, createdAt time.Time) map[string]time.Time {
	reached := map[string]time.Time{KeyApplied: createdAt}
	for _, e := range events {
		mk, ok := rawToMilestone[e.To]
		if !ok {
			continue
		}
		if existing, seen := reached[mk]; !seen || e.At.Before(existing) {
			reached[mk] = e.At
		}
	}
	return reached
}

// happyTimeline returns the full ordered ladder with done/current/upcoming.
func happyTimeline(reached map[string]time.Time, current string) []Milestone {
	curIdx := indexOf(happyPath, current)
	out := make([]Milestone, 0, len(happyPath))
	for i, key := range happyPath {
		state := StateUpcoming
		switch {
		case i < curIdx:
			state = StateDone
		case i == curIdx:
			state = StateCurrent
		}
		out = append(out, milestone(key, state, reached))
	}
	return out
}

// branchTimeline returns the steps actually reached on the happy path (done),
// then the terminal/recoverable branch milestone (current). No future steps.
func branchTimeline(reached map[string]time.Time, branch string) []Milestone {
	out := make([]Milestone, 0, 4)
	for _, key := range happyPath {
		if _, entered := reached[key]; entered {
			out = append(out, milestone(key, StateDone, reached))
		}
	}
	out = append(out, milestone(branch, StateCurrent, reached))
	return out
}

func milestone(key, state string, reached map[string]time.Time) Milestone {
	var at *time.Time
	if t, ok := reached[key]; ok {
		tt := t
		at = &tt
	}
	return Milestone{Key: key, Label: labels[key], Detail: details[key], ReachedAt: at, State: state}
}

func indexOf(xs []string, v string) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return 0 // unknown current → treat as the first step
}
