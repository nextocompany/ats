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

// GET /api/v1/public/positions/:id — public detail projection.
export interface PositionDetail {
  id: string;
  title_th: string;
  title_en: string;
  level: string;
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

// Fields posted as multipart/form-data to /apply (resume is a File).
export interface ApplyInput {
  positionId: string;
  fullName: string;
  phone?: string;
  email?: string;
  idCard?: string;
  province?: string;
  consentVersion: string;
  resume: File;
  lineIdToken: string;
}
