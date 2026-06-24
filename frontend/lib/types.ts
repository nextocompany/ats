// API types mirroring the Go backend JSON (Sprint 4a contract).

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

// ScoreBreakdown is the per-dimension AI score (mirrors the Go
// applications.ScoreBreakdown). Max points: experience 30, skills 20 (LLM),
// education 10, language 10, location 20 → 100 total.
export interface ScoreBreakdown {
  experience: number;
  skills: number;
  education: number;
  language: number;
  location: number;
}

export interface Application {
  id: string;
  candidate_id: string;
  position_id: string;
  status: string;
  ai_score: number | null;
  must_have_passed: boolean | null;
  assigned_store_id: number | null;
  talent_pool: boolean;
  dedup_state: string;
  needs_manual_review: boolean;
  ocr_confidence: number | null;
  raw_file_blob_url: string;
  raw_file_type: string;
  parsed_profile_blob_url: string;
  parsed_at: string | null;
  created_at: string;
  // Score explainability (single-record detail responses only). Present once the
  // application is scored; the detail panel renders the per-dimension breakdown
  // plus the LLM's strengths/red flags.
  ai_score_breakdown?: ScoreBreakdown | null;
  ai_summary?: string;
  ai_red_flags?: string;
  ai_suggested_positions?: string[] | null;
  // Internal rejection reason (single-record detail responses). Never shown to the
  // candidate; surfaced to HR on a rejected application.
  rejection_reason?: string;
  // Human-readable joins from the inbox list endpoint (omitted on single-record
  // responses). The inbox leads with these so a row reads as a person, not a UUID.
  candidate_name?: string;
  candidate_province?: string;
  source_channel?: string;
  position_title?: string;
  store_name?: string;
}

// InterviewAppointment is a scheduled human interview (mirrors applications.Appointment).
// StoreOption is the slim store projection for the placement/reassign picker.
export interface StoreOption {
  store_no: number;
  store_name: string;
  subregion: string;
  province: string;
}

// Area mirrors internal/areas.Area — a dynamic grouping of stores backing the
// area visibility scope. store_nos / member_ids are populated on the detail GET.
export interface Area {
  id: string;
  name: string;
  active: boolean;
  store_count: number;
  store_nos?: number[];
  member_ids?: string[];
}

export interface InterviewAppointment {
  id: string;
  application_id: string;
  round_no: number;
  scheduled_at: string;
  duration_min: number;
  mode: "onsite" | "online";
  location_text?: string;
  online_join_url?: string;
  created_at: string;
  // Set by the schedule endpoint when an online interview was booked but the Teams
  // link could not be created — HR should send a link manually.
  warning?: string;
}

// UpcomingInterview is a scheduled interview joined with candidate/position/store
// for the HR calendar (GET /api/v1/interviews/upcoming, role-scoped).
export interface UpcomingInterview {
  id: string;
  application_id: string;
  round_no: number;
  scheduled_at: string;
  duration_min: number;
  mode: "onsite" | "online";
  location_text?: string;
  online_join_url?: string;
  candidate_name: string;
  position_title: string;
  store_name?: string;
  assigned_store_id?: number | null;
}

// InterviewCompetencies mirrors applications.InterviewCompetencies — per-dimension
// 0..5 ratings (0 = not rated).
// Superset of competency dimensions across perspectives (0 = not rated). TA rates
// communication/technical/experience/attitude; the Line Manager rates
// culture_fit/growth_potential/leadership. Each perspective sends only its subset.
export interface InterviewCompetencies {
  communication: number;
  technical: number;
  experience: number;
  attitude: number;
  culture_fit: number;
  growth_potential: number;
  leadership: number;
}

export type ScorecardPerspective = "ta" | "line_manager";

export type InterviewRecommendation = "pass" | "hold" | "fail";

// InterviewFeedback mirrors applications.InterviewFeedback — a structured outcome a
// hiring panel records during the interview stage (many per application).
export interface InterviewFeedback {
  id: string;
  application_id: string;
  appointment_id?: string;
  interviewer_name?: string;
  perspective: ScorecardPerspective;
  overall_rating: number;
  recommendation: InterviewRecommendation;
  competencies: InterviewCompetencies;
  strengths?: string;
  concerns?: string;
  notes?: string;
  created_at: string;
}

// InterviewFeedbackInput is the create payload (POST .../interview-feedback).
export interface InterviewFeedbackInput {
  perspective: ScorecardPerspective;
  overall_rating: number;
  recommendation: InterviewRecommendation;
  competencies: InterviewCompetencies;
  strengths?: string;
  concerns?: string;
  notes?: string;
}

// PerspectiveAgg mirrors applications.PerspectiveAgg (averaged scorecard).
export interface PerspectiveAgg {
  count: number;
  avg_overall: number;
  avg_competencies: Record<string, number>;
  recommendations: Record<string, number>;
}

// ScorecardSummary mirrors applications.ScorecardSummary (TA + LM aggregate).
export interface ScorecardSummary {
  ta: PerspectiveAgg | null;
  line_manager: PerspectiveAgg | null;
  composite_score: number | null;
}

// --- Approval workflow (Module-3 3.5) ---------------------------------------
// Multi-level hiring approval chain. Mirrors internal/applications/approval.go.

export type ApprovalDecision = "approve" | "reject";
export type ApprovalEntityStatus = "pending" | "approved" | "rejected";

// ApprovalStep mirrors applications.ApprovalStep (one level in the chain).
export interface ApprovalStep {
  id: string;
  level: number;
  role: string;
  status: ApprovalEntityStatus;
  approver_name?: string;
  comment?: string;
  due_at: string | null;
  escalated: boolean;
  decided_at: string | null;
}

// ApprovalRequest mirrors applications.ApprovalRequest (a hire decision + steps).
export interface ApprovalRequest {
  id: string;
  application_id: string;
  status: ApprovalEntityStatus;
  current_level: number;
  created_at: string;
  decided_at: string | null;
  decision_reason?: string;
  steps: ApprovalStep[];
}

// ApprovalQueueItem mirrors applications.ApprovalQueueItem (an "awaiting me" row).
export interface ApprovalQueueItem {
  request_id: string;
  application_id: string;
  candidate_name?: string;
  position_title?: string;
  store_id: number | null;
  active_level: number;
  active_role: string;
  ai_score: number | null;
  due_at: string | null;
  waiting_since: string;
}

// ApprovalDecisionInput is the decide payload (POST .../decide).
export interface ApprovalDecisionInput {
  decision: ApprovalDecision;
  comment?: string;
  reason?: string;
}

// --- Offer management (Module-3 3.6) ----------------------------------------
// Mirrors internal/applications/offer.go.

export type OfferStatus = "draft" | "sent" | "accepted" | "declined" | "expired";

// Offer mirrors applications.Offer (one per application).
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
}

// OfferInput is the HR compose/edit payload.
export interface OfferInput {
  salary?: number | null;
  start_date?: string | null;
  terms?: string;
  expires_at?: string | null;
}

// --- Letters (Module-3 3.3) -------------------------------------------------
// Mirrors applications.LetterView.
export type LetterType = "interview" | "offer";

export interface Letter {
  id: string;
  type: LetterType;
  created_at: string;
  url: string;
}

// --- Onboarding documents (Module-3 3.8) ------------------------------------
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

// OnboardingStatus mirrors applications.OnboardingStatus (the checklist + progress).
export interface OnboardingStatus {
  application_id: string;
  required: string[];
  documents: OnboardingDoc[];
  approved_count: number;
  required_count: number;
  complete: boolean;
}

// ShortlistItem mirrors applications.ShortlistItem (LM Top-5 row).
export interface ShortlistItem {
  application_id: string;
  candidate_name: string;
  position_id: string;
  position_title: string;
  assigned_store_id: number | null;
  ai_score: number | null;
  ta_avg_overall: number | null;
  composite: number;
}

// Position is the slim picker projection (mirrors positions.ListItem).
export interface Position {
  id: string;
  title_th: string;
  title_en: string;
}

// ScoreWeights are the per-position screening-dimension importance percentages
// (mirrors scoring.Weights). They sum to 100.
export interface ScoreWeights {
  experience: number;
  skills: number;
  education: number;
  language: number;
  location: number;
}

// PositionDetail is the full position projection (mirrors positions.DetailItem),
// used to prefill the requisition form with the position's Master JD text and to
// edit per-position screening weights. score_weights is always the EFFECTIVE set
// (stored config, or the backend default when unset).
export interface PositionDetail {
  id: string;
  title_th: string;
  title_en: string;
  level: string;
  responsibilities: string;
  qualifications: string;
  benefits: string;
  score_weights: ScoreWeights;
}

// Requisition is a managed position opening (a row in the vacancies table,
// source='manual'). Mirrors internal/requisitions.Requisition.
export interface Requisition {
  id: string;
  position_id: string | null;
  position_title: string;
  store_id: number | null;
  store_name: string;
  subregion: string;
  headcount: number;
  status: string; // pending_approval | open | closed | cancelled
  source: string; // manual | peoplesoft
  created_by: string | null;
  approved_by: string | null;
  approved_at: string | null;
  created_at: string;
  updated_at: string;
  // Detailed JD + opening metadata (manual requisitions).
  responsibilities: string;
  qualifications: string;
  benefits: string;
  other_details: string;
  employment_type: string; // "" | full_time | part_time | contract | seasonal
  salary_min: number | null;
  salary_max: number | null;
  priority: string; // normal | urgent
  open_reason: string; // "" | new_headcount | replacement
}

export interface RequisitionInput {
  position_id?: string;
  store_id?: number;
  headcount?: number;
  responsibilities?: string;
  qualifications?: string;
  benefits?: string;
  other_details?: string;
  employment_type?: string;
  salary_min?: number;
  salary_max?: number;
  priority?: string;
  open_reason?: string;
}

export interface RequisitionFilter {
  status?: string;
  store_id?: number;
  position_id?: string;
  page?: number;
  limit?: number;
}

// BulkIntakeResult mirrors applications.bulkResult (per-file outcome of a bulk upload).
export interface BulkIntakeResult {
  total: number;
  succeeded: number;
  failed_count: number;
  created: { filename: string; application_id: string }[];
  failed: { filename: string; error: string }[];
}

export interface Candidate {
  id: string;
  full_name: string;
  phone: string;
  email: string;
  id_card: string;
  province: string;
  subregion: string;
  source_channel: string;
  status: string;
  created_at: string;
}

export interface TimelineEntry {
  action: string;
  entity_type: string;
  entity_id: string;
  // Action-specific detail (e.g. { from, to } for a status_change). Shape varies by
  // action, so it's loosely typed; consumers read known keys defensively.
  new_value?: { from?: string; to?: string; status?: string } | null;
  created_at: string;
}

// --- AI pre-interview (slice 2.5) ---

export interface InterviewTurn {
  role: "assistant" | "user";
  content: string;
  ts?: string;
}

// InterviewSession mirrors the backend interview.Session JSON. Evaluation fields
// are null/empty until status === "completed".
export interface InterviewSession {
  id: string;
  application_id: string;
  access_token: string;
  status: string;
  conversation: InterviewTurn[];
  turn_count: number;
  interview_score: number | null;
  recommendation: string;
  strengths: string[] | null;
  concerns: string[] | null;
  summary: string;
  invited_at: string;
  started_at: string | null;
  completed_at: string | null;
  expires_at: string;
  created_at: string;
}

// GET /api/v1/applications/:id/interview
export interface InterviewView {
  session: InterviewSession;
  interview_url: string;
}

// POST /api/v1/applications/:id/interview
export interface InterviewInviteResult {
  id: string;
  status: string;
  access_token: string;
  interview_url: string;
}

// RecommendedPosition is one Master JD role the AI judges the candidate a fit for.
export interface RecommendedPosition {
  position_id: string;
  title: string;
  fit_score: number;
  reasons: string[];
}

// FitAnalysis mirrors the backend fit.Analysis JSON — the cross-position verdict
// combining screening + AI interview against the whole Master JD catalogue.
export interface FitAnalysis {
  application_id: string;
  overall_fit: "strong" | "moderate" | "weak" | "none";
  summary: string;
  strengths: string[];
  concerns: string[];
  recommended: RecommendedPosition[];
  no_match_reason?: string;
  generated_at: string;
}

export interface Funnel {
  applied: number;
  passed_ai: number;
  reviewed: number;
  hired: number;
}

export interface KPI {
  applied: number;
  passed: number;
  onboarded: number;
  waiting: number;
}

export interface Source {
  channel: string;
  applied: number;
  hired: number;
  conversion: number;
}

// --- ATS Reports (Module-3 3.9) ---------------------------------------------
// Mirrors internal/reports/ats_report.go (RBAC-scoped, date-ranged metrics).
export interface AtsFunnelStage {
  key: string;
  count: number;
  conversion_pct: number;
}

export interface AtsReport {
  from: string;
  to: string;
  scope: string;
  funnel: { stages: AtsFunnelStage[] };
  timing: {
    hired_count: number;
    avg_days_to_hire: number;
    median_days_to_hire: number;
    avg_days_to_offer: number;
    avg_offer_response_days: number;
  };
  offers: {
    sent: number;
    accepted: number;
    declined: number;
    accept_rate_pct: number;
    decline_rate_pct: number;
  };
  onboarding: {
    hired_in_range: number;
    completed: number;
    completion_rate_pct: number;
    docs_reviewed: number;
    docs_rejected: number;
    doc_rejection_rate_pct: number;
  };
  quality: {
    interview_feedback_count: number;
    interview_passed: number;
    interview_pass_rate_pct: number;
    avg_interview_rating: number;
    approval_decided: number;
    avg_approval_cycle_days: number;
    approval_steps: number;
    approval_breached: number;
    approval_sla_breach_pct: number;
  };
}

// StoreLoad: review backlog at one store (reports/by-store).
export interface StoreLoad {
  store_id: number | null;
  store_name: string;
  waiting: number;
}

// OpenRole: open hiring need by position (reports/open-roles).
export interface OpenRole {
  position_id: string;
  title: string;
  openings: number;
  stores: number;
}

export interface Me {
  id: string;
  email: string;
  role: string;
  store_id: number | null;
  subregion: string;
  // Dynamic RBAC: the caller's resolved permission keys + data scope, sent by
  // GET /users/me. The UI gates on these instead of hardcoded role lists.
  permissions?: string[];
  scope?: string;
  // Local users.id (uniform across SSO + password); used to tell whether a
  // candidate lock is held by the current user. id is the Entra OID for SSO.
  local_id?: string;
}

// CandidateLock mirrors internal/candidatelock.Lock — the short-lived processing
// lock on a candidate (keyed by canonical candidates.id).
export interface CandidateLock {
  candidate_id: string;
  locked_by: string;
  locked_by_name?: string;
  locked_at: string;
  expires_at: string;
}

// --- Dynamic RBAC (admin role/permission management) ---
export interface RbacPermission {
  key: string;
  label_en: string;
  label_th: string;
  category: string;
  sort: number;
}

export interface RbacRole {
  key: string;
  label_en: string;
  label_th: string;
  scope_kind: string;
  is_builtin: boolean;
  permissions: string[];
  created_at: string;
}

export interface RbacRoleInput {
  key?: string;
  label_en?: string;
  label_th?: string;
  scope_kind?: string;
  permissions?: string[];
}

// --- Executive Overview (company-wide leadership dashboard) ---
// Mirrors the Go internal/executive payload. data_source is "mock" (demo) or
// "live" (ATS-derived; budget pending PeopleSoft → budget_available=false).

export interface ExecutiveCompany {
  budget_headcount: number;
  actual_headcount: number;
  vacancy: number;
  fill_rate_pct: number;
  budget_available: boolean;
}

// ExecutiveStoreFill: one branch's staffing for the "most short-staffed" ranking.
export interface ExecutiveStoreFill {
  store_no: number;
  store_name: string;
  subregion: string;
  budget_headcount: number;
  actual_headcount: number;
  heads_short: number;
  fill_rate_pct: number;
}

// ExecutivePipelinePosition: recruitment funnel for one position company-wide.
export interface ExecutivePipelinePosition {
  position_id: string;
  title: string;
  applied: number;
  screening: number;
  interview: number;
  offer: number;
  hired: number;
  openings: number;
}

export interface ExecutiveOverview {
  data_source: "mock" | "live";
  generated_at: string;
  company: ExecutiveCompany;
  stores: ExecutiveStoreFill[]; // sorted asc by fill_rate (most short-staffed first)
  pipeline: ExecutivePipelinePosition[];
  sourcing: Source[];
}

// AdminSettings mirrors the Go settings handler dto — runtime, admin-managed flags.
export interface AdminSettings {
  allow_all_tenants: boolean;
}

// HRUser mirrors internal/hrauth.User — a local password account managed by a
// super_admin (alongside Entra SSO users).
export interface HRUser {
  id: string;
  email: string;
  full_name: string;
  role: string;
  store_id?: number | null;
  subregion?: string;
  is_active: boolean;
  has_password: boolean;
  source: "local" | "sso";
  phone?: string;
  is_dpo: boolean;
  is_primary_dpo: boolean;
  last_login_at?: string | null;
  created_at: string;
}

// CreateHRUserInput is the super_admin payload to provision a local account.
export interface CreateHRUserInput {
  email: string;
  full_name: string;
  role: string;
  store_id?: number | null;
  subregion?: string;
  password: string;
}

// UpdateHRUserInput patches an account (sparse: omitted fields are unchanged).
export interface UpdateHRUserInput {
  full_name?: string;
  role?: string;
  store_id?: number | null;
  subregion?: string;
  is_active?: boolean;
  phone?: string;
  is_dpo?: boolean;
  is_primary_dpo?: boolean;
  password?: string;
}

export interface ApplicationFilter {
  status?: string;
  min_score?: number;
  store_id?: number;
  source_channel?: string;
  page?: number;
  limit?: number;
}

// Member mirrors the Go members.Member admin projection (career-portal accounts).
export interface Member {
  id: string;
  full_name: string;
  email: string;
  phone: string;
  province: string;
  email_verified: boolean;
  line_linked: boolean;
  google_linked: boolean;
  email_linked: boolean;
  has_resume: boolean;
  resume_file_type: string;
  status: "active" | "suspended" | "anonymized";
  pdpa_consent: boolean;
  pdpa_version: string;
  applications_count: number;
  active_sessions: number;
  last_login_at: string | null;
  created_at: string;
  // Populated only on the person detail (the unified Candidates detail): one row
  // per position the person applied to, across all linked candidate rows.
  applications?: AccountApplication[];
  // Canonical (non-duplicate) candidate id for this account — the key for the
  // candidate processing lock. Absent for an account with no candidate yet.
  candidate_id?: string;
}

// AccountApplication mirrors the Go members.AccountApplication — a per-position
// funnel row shown in the unified person detail, linking to /applications/[id].
export interface AccountApplication {
  id: string;
  position_id: string;
  position_title: string;
  status: string;
  ai_score: number | null;
  created_at: string;
}

export interface MemberFilter {
  search?: string;
  provider?: string; // line | google | email
  status?: string;
  tag?: string;
  has_resume?: boolean;
  from?: string;
  to?: string;
  page?: number;
  limit?: number;
}

// MemberNote mirrors the Go members.Note (HR-only timeline note).
export interface MemberNote {
  id: string;
  author_email: string;
  body: string;
  created_at: string;
}

// MemberStats mirrors the Go members.Stats summary strip.
export interface MemberStats {
  total: number;
  active: number;
  suspended: number;
  with_applications: number;
  new_this_week: number;
  by_provider: Record<string, number>;
}

// ReportExport mirrors reports.exportView (Sprint 5b): a stored export plus
// short-lived signed download links.
export interface ReportExport {
  id: string;
  kind: string;
  period: string;
  csv_blob: string;
  json_blob: string;
  delivered: boolean;
  created_at: string;
  csv_url?: string;
  json_url?: string;
}

// SearchHit mirrors search.Hit (Sprint 5c): a candidate plus their best
// application's status/score.
export interface SearchHit {
  candidate_id: string;
  full_name: string;
  province: string;
  status: string;
  ai_score: number | null;
}

export interface SearchFilter {
  q: string;
  status?: string;
  page?: number;
  limit?: number;
}

// --- PDPA / DPO console (Phase 5.4) ---

// DpoOfficer is one published Data Protection Officer (s.41).
export interface DpoOfficer {
  name: string;
  email: string;
  phone: string;
  is_primary: boolean;
}

// DpoDirectory mirrors pdpa.DPODirectory: the controller (company) plus every
// active DPO-flagged account. officers is never null (empty ⇒ none designated yet).
export interface DpoDirectory {
  company: string;
  officers: DpoOfficer[];
}

// PdpaOverview mirrors pdpaadmin.Overview: the console summary card data.
export interface PdpaOverview {
  dsar_pending: number;
  breaches_open: number;
  breaches_overdue: number;
  current_consent_version: string;
  retention_days: number;
  retention_sweep_enabled: boolean;
  dpo: DpoDirectory;
}

// DsarRequest mirrors pdpaadmin.DSARRequest: one queued data-subject request
// joined with the account identity.
export interface DsarRequest {
  id: string;
  account_id: string;
  account_name: string;
  account_email: string;
  request_type: string;
  status: string;
  reason: string;
  requested_at: string;
  resolved_at: string | null;
  resolved_by: string | null;
}

export interface DsarFilter {
  status?: string;
  page?: number;
  limit?: number;
}

// ConsentRecord mirrors pdpaadmin.ConsentRecord: one row of a subject's consent
// history (the unified ledger).
export interface ConsentRecord {
  id: string;
  candidate_id: string | null;
  account_id: string | null;
  consent_given: boolean;
  version: string;
  source_channel: string;
  created_at: string;
}

// PolicyDoc mirrors pdpa.ConsentDocument: the current published privacy/consent
// notice, shown read-only on the dashboard /privacy reference page.
export interface PolicyDoc {
  version: string;
  locale: string;
  title: string;
  body: string;
  effective_at: string;
}
