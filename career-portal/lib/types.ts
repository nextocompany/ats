// Public Career API types mirroring the Go backend (internal/public/handler.go).
// The portal is unauthenticated except for the LINE id-token sent on apply.

export interface Meta {
  total: number;
  page: number;
  limit: number;
}

export interface Envelope<T> {
  success: boolean;
  data: T;
  error?: string;
  meta?: Meta;
}

// GET /api/v1/public/positions — open positions only (positions.PublicPosition).
export interface PublicPosition {
  id: string;
  title_th: string;
  title_en: string;
  level: string;
  open_count: number;
}

// Offer management (Module-3 3.6) — mirrors applications.OfferView / OfferResponseInput.
export type OfferStatus = "draft" | "sent" | "accepted" | "declined" | "expired";

export interface Offer {
  id: string;
  application_id: string;
  status: OfferStatus;
  salary: number | null;
  start_date: string | null;
  terms?: string;
  sent_at: string | null;
  responded_at: string | null;
  expires_at: string | null;
  decline_reason?: string;
  created_at: string;
  position_title?: string;
  store_id: number | null;
}

export interface OfferResponseInput {
  decision: "accept" | "decline";
  reason?: string;
}

// Letters (Module-3 3.3) — mirrors applications.LetterView.
export type LetterType = "interview" | "offer";

export interface Letter {
  id: string;
  type: LetterType;
  created_at: string;
  url: string;
}

// --- Onboarding documents (Module-3 3.8) ---
// Mirrors internal/applications/onboarding.go.
export type DocStatus = "pending" | "approved" | "rejected";

export interface OnboardingDoc {
  id: string;
  doc_type: string;
  status: DocStatus;
  file_name?: string;
  file_type?: string;
  review_reason?: string;
  uploaded_at: string;
  reviewed_at?: string | null;
  url?: string;
}

// GET /api/v1/public/auth/onboarding — the member's checklist + progress.
export interface OnboardingStatus {
  application_id: string;
  required: string[];
  documents: OnboardingDoc[];
  approved_count: number;
  required_count: number;
  complete: boolean;
}

// GET /api/v1/public/positions/:id — public detail projection. The JD text is the
// position catalog's role-generic Master JD (shown when present, see JobDetailClient).
export interface PositionDetail {
  id: string;
  title_th: string;
  title_en: string;
  level: string;
  responsibilities: string;
  qualifications: string;
  benefits: string;
}

// POST /api/v1/public/apply — returns the opaque status token (201).
export interface ApplyResult {
  status_token: string;
}

// GET /api/v1/public/status/:token — minimal candidate-facing projection.
export interface ApplicationStatus {
  status: string;
  applied_at: string;
  position?: string;
}

// GET /api/v1/public/me/applications — the logged-in member's application history
// (one row per position applied to). status_token deep-links to /status.
export interface PortalApplication {
  status_token: string;
  position_title: string;
  status: string;
  applied_at: string;
}

// --- AI pre-interview (slice 2.5) ---

// One message in the interview chat. role mirrors the chat-completion roles.
export interface InterviewTurn {
  role: "assistant" | "user";
  content: string;
}

// Candidate-facing interview state returned by the public interview endpoints.
export interface InterviewSessionState {
  status: string;
  turns: InterviewTurn[];
  done: boolean;
}

// Fields posted as multipart/form-data to /apply (resume is a File). Identity is
// the candidate session cookie (account-first) — no LINE id-token header.
export interface ApplyInput {
  positionId: string;
  fullName: string;
  phone?: string;
  email?: string;
  idCard?: string;
  province?: string;
  consentVersion: string;
  resume: File;
}

// --- candidate membership (internal/candidateauth) ---

// GET /api/v1/public/auth/me — client-safe account projection.
export interface Account {
  id: string;
  full_name: string;
  email: string;
  phone: string;
  province: string;
  line_display_id: string;
  line_linked: boolean;
  google_linked: boolean;
  has_resume: boolean;
  resume_file_type: string;
  pdpa_consent: boolean;
  // Consent versioning (Phase 2): the version the member accepted, and whether a
  // newer notice version is current (so the UI can prompt for re-consent).
  pdpa_version?: string;
  pdpa_needs_reconsent?: boolean;
}

// One entry in a candidate's CV history (GET /api/v1/public/auth/resumes).
export interface AccountResume {
  id: string;
  original_filename: string;
  file_type: string;
  is_default: boolean;
  created_at: string;
}

// PATCH /api/v1/public/auth/profile body.
export interface ProfileInput {
  full_name?: string;
  phone?: string;
  line_display_id?: string;
  province?: string;
  consent_given?: boolean;
  consent_version?: string;
}

// POST /api/v1/public/apply/quick — quick-apply with the saved resume (201).
export interface QuickApplyResult {
  status_token: string;
}
