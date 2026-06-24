package applications

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/nexto/hr-ats/internal/rbac"
)

// ErrApprovalConflict signals that an approval write lost a race — the request was
// already opened/decided or advanced past the level the caller targeted. Handlers
// map it to HTTP 409 (the SQL guards are the real serialization point; this is the
// clean status for the loser of a concurrent submit/decide).
var ErrApprovalConflict = errors.New("applications: approval request conflict")

// Multi-level hiring approval chain (Module-3 3.5). A request is opened from the
// interviewed stage and signed off by a fixed four-level chain in order:
// Staff (hr_staff) -> HR Manager -> SGM -> Regional Director. The creator is the
// Staff-level sign-off (level 1 is recorded approved at creation); levels 2..4 are
// the remaining approvals. Any reject (with a reason) is terminal. The state lives
// in the approval_requests / approval_steps tables (migration 000022).

// Approval request / step lifecycle states.
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)

// Decision verbs accepted by the decide endpoint.
const (
	DecisionApprove = "approve"
	DecisionReject  = "reject"
)

// approvalMaxLevel is the final (Regional Director) level; approving it completes
// the chain and advances the application to offer.
const approvalMaxLevel = 4

// approvalChain is the ordered level -> role mapping seeded into approval_steps.
// The role is a label used for SLA-reminder targeting + display; decide authZ is
// by permission (canDecideLevel), so both the legacy and new role at a level can
// approve. Labels use the new role model (hr_store/area_hr/hiring_manager_store/ta).
var approvalChain = []struct {
	Level int
	Role  string
}{
	{1, "hr_store"},
	{2, "area_hr"},
	{3, "hiring_manager_store"},
	{4, "ta"},
}

// canDecideLevel reports whether role may decide the given approval level —
// resolved via dynamic RBAC (rbac.PermApprovalDecideL1..L4). super_admin holds all.
func canDecideLevel(role string, level int) bool {
	perm := rbac.ApprovalDecidePermForLevel(level)
	if perm == "" {
		return false
	}
	return rbac.Can(role, perm)
}

// canSubmitApproval reports whether role may open an approval request.
func canSubmitApproval(role string) bool {
	return rbac.Can(role, rbac.PermApprovalSubmit)
}

// validDecision reports whether d is a recognised decision verb.
func validDecision(d string) bool {
	return d == DecisionApprove || d == DecisionReject
}

// roleForLevel returns the responsible role for an approval level (empty if out of
// range).
func roleForLevel(level int) string {
	for _, c := range approvalChain {
		if c.Level == level {
			return c.Role
		}
	}
	return ""
}

// levelLabel returns a human label for an approval level, used in notification copy.
func levelLabel(level int) string {
	switch level {
	case 1:
		return "Staff"
	case 2:
		return "HR Manager"
	case 3:
		return "SGM"
	case 4:
		return "Regional Director"
	default:
		return ""
	}
}

// ApprovalStep is one level in the chain. ApproverID is server-stamped and never
// client-supplied (json:"-"); the joined ApproverName is exposed instead, exactly
// like InterviewFeedback.InterviewerName.
type ApprovalStep struct {
	ID           uuid.UUID  `json:"id"`
	Level        int        `json:"level"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	ApproverID   *uuid.UUID `json:"-"`
	ApproverName string     `json:"approver_name,omitempty"`
	Comment      string     `json:"comment,omitempty"`
	DueAt        *time.Time `json:"due_at"`
	Escalated    bool       `json:"escalated"`
	DecidedAt    *time.Time `json:"decided_at"`
}

// ApprovalRequest is a single hire decision plus its four ordered steps.
type ApprovalRequest struct {
	ID             uuid.UUID      `json:"id"`
	ApplicationID  uuid.UUID      `json:"application_id"`
	Status         string         `json:"status"`
	CurrentLevel   int            `json:"current_level"`
	CreatedBy      *uuid.UUID     `json:"-"`
	CreatedAt      time.Time      `json:"created_at"`
	DecidedAt      *time.Time     `json:"decided_at"`
	DecisionReason string         `json:"decision_reason,omitempty"`
	Steps          []ApprovalStep `json:"steps"`
}

// ApprovalQueueItem is a row in an approver's "awaiting my decision" queue.
type ApprovalQueueItem struct {
	RequestID     uuid.UUID  `json:"request_id"`
	ApplicationID uuid.UUID  `json:"application_id"`
	CandidateName string     `json:"candidate_name,omitempty"`
	PositionTitle string     `json:"position_title,omitempty"`
	StoreID       *int       `json:"store_id"`
	ActiveLevel   int        `json:"active_level"`
	ActiveRole    string     `json:"active_role"`
	AIScore       *float64   `json:"ai_score"`
	DueAt         *time.Time `json:"due_at"`
	WaitingSince  time.Time  `json:"waiting_since"`
}

// OverdueApprovalStep is an active pending step past its SLA, used by the sweep to
// escalate to the responsible approver role.
type OverdueApprovalStep struct {
	StepID        uuid.UUID `json:"step_id"`
	Role          string    `json:"role"`
	StoreID       *int      `json:"store_id"`
	CandidateName string    `json:"candidate_name,omitempty"`
	PositionTitle string    `json:"position_title,omitempty"`
}

// ApprovalDecisionInput is the decide-endpoint request body.
type ApprovalDecisionInput struct {
	Decision string `json:"decision"` // approve | reject
	Comment  string `json:"comment"`
	Reason   string `json:"reason"` // required when Decision == reject
}

// approvalDecideArgs carries a validated decision into the repository transaction.
type approvalDecideArgs struct {
	RequestID  uuid.UUID
	Level      int
	Approve    bool
	ApproverID uuid.UUID
	Comment    string
	Reason     string
	SLAHours   int
}
