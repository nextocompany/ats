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

export interface Me {
  id: string;
  email: string;
  role: string;
  store_id: number | null;
  subregion: string;
}

export interface ApplicationFilter {
  status?: string;
  min_score?: number;
  store_id?: number;
  source_channel?: string;
  page?: number;
  limit?: number;
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
