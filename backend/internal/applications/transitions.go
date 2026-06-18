package applications

// Candidate status state machine. This is the single source of truth for which
// manual HR transitions are legal; the dashboard handlers (status PATCH, bulk,
// and the interview-schedule endpoint) all gate against it, and the frontend
// mirrors it in lib/statusMachine.ts. Keep the two in sync.
//
// Progression (see the plan / docs):
//
//	scored        --Send AI Interview-->  ai_interview      (interview pkg, guarded there)
//	ai_interview  --(session completed)-> ai_interviewed    (system, interview pkg)
//	ai_interviewed--Shortlist/Interview/Reject
//	shortlisted   --Interview/Reject
//	interview     --Mark done(interviewed)/Reject
//	interviewed   --Submit for approval(pending_approval)/Reject
//	pending_approval --(4-level chain decides)--> offer | rejected
//	offer         --Reject
//
// The "interview" target requires a schedule payload, so it is reachable only via
// the interview-schedule endpoint, never via a plain status PATCH (RequiresSchedule).
// The "offer" target is reachable only by completing the approval chain (the
// approval decide endpoint owns pending_approval's exits), never via a plain PATCH —
// pending_approval therefore has NO generic transitions. "rejected" is reachable from
// every funnel state and requires a reason.

// allowedTransitions[current] is the set of manual target statuses permitted from
// that state. Transitions driven by the system (Send AI Interview → ai_interview,
// session-completed → ai_interviewed) live in the interview package and are not
// listed here.
var allowedTransitions = map[string]map[string]bool{
	StatusAIInterviewed: {StatusShortlisted: true, StatusInterview: true, StatusRejected: true},
	StatusShortlisted:   {StatusInterview: true, StatusRejected: true},
	StatusInterview:     {StatusInterviewed: true, StatusRejected: true},
	StatusInterviewed:   {StatusRejected: true}, // hire now routes through the approval chain, not a direct PATCH
	StatusOffer:         {StatusRejected: true},
	// StatusPendingApproval is intentionally absent: its only legal exits (offer,
	// rejected) are driven by the approval decide endpoint inside a transaction.
}

// CanTransition reports whether moving from → to is a legal manual HR transition.
func CanTransition(from, to string) bool {
	return allowedTransitions[from][to]
}

// RequiresSchedule reports whether a target status may only be reached via the
// interview-schedule endpoint (which collects a date/time + mode), not a plain
// status PATCH.
func RequiresSchedule(to string) bool {
	return to == StatusInterview
}

// RequiresReason reports whether a target status must carry a reason.
func RequiresReason(to string) bool {
	return to == StatusRejected
}

// CanRequestApproval reports whether a hiring approval request may be opened from
// the given status. Only an interviewed candidate can enter the approval chain.
func CanRequestApproval(status string) bool {
	return status == StatusInterviewed
}
