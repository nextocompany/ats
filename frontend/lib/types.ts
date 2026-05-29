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
