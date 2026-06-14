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
  // Human-readable joins from the inbox list endpoint (omitted on single-record
  // responses). The inbox leads with these so a row reads as a person, not a UUID.
  candidate_name?: string;
  candidate_province?: string;
  source_channel?: string;
  position_title?: string;
  store_name?: string;
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
}

// AdminSettings mirrors the Go settings handler dto — runtime, admin-managed flags.
export interface AdminSettings {
  allow_all_tenants: boolean;
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
}

export interface MemberFilter {
  search?: string;
  provider?: string; // line | google | email
  status?: string;
  has_resume?: boolean;
  from?: string;
  to?: string;
  page?: number;
  limit?: number;
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
