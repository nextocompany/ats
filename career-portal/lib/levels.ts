// Candidate level taxonomy — the only job facet the public API exposes
// (entry / experienced / senior / management). Centralized so the jobs filter,
// job card, and detail page share one source of truth for labels + ordering.

export const LEVELS = ["entry", "experienced", "senior", "management"] as const;

export type Level = (typeof LEVELS)[number];

export const LEVEL_LABELS: Record<string, string> = {
  entry: "ระดับเริ่มต้น",
  experienced: "มีประสบการณ์",
  senior: "ระดับอาวุโส",
  management: "ระดับบริหาร",
};

export function levelLabel(level: string): string {
  return LEVEL_LABELS[level.toLowerCase()] ?? level;
}
