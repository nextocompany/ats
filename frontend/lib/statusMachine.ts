// UI mirror of the backend candidate state machine
// (backend/internal/applications/transitions.go). The backend is the real gate;
// this only decides which action buttons to show. Keep the two in sync.

export type Action =
  | "send_ai_interview"
  | "shortlist"
  | "interview"
  | "mark_interviewed"
  | "submit_approval"
  | "reject";

// allowedActions returns the HR actions available from a given application status.
export function allowedActions(status: string): Action[] {
  switch (status) {
    case "scored": // "screened"
      return ["send_ai_interview"];
    case "ai_interview":
      return []; // AI interview in progress — wait for completion
    case "ai_interviewed":
      return ["shortlist", "interview", "reject"];
    case "shortlisted":
      return ["interview", "reject"];
    case "interview":
      // "interview" here re-opens the scheduler for an additional round.
      return ["mark_interviewed", "interview", "reject"];
    case "interviewed":
      // Allow another round after a completed one, or submit for approval.
      return ["submit_approval", "interview", "reject"];
    case "pending_approval":
      return []; // the ApprovalPanel drives approve/reject for the active level
    case "offer":
      return ["reject"];
    default:
      return []; // pending/parsed/failed/rejected — no manual HR actions
  }
}
