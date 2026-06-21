// Shared formatting helpers.

// formatDateTime renders an ISO timestamp in the user's locale, or "-" when the
// value is absent or unparseable.
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "-" : d.toLocaleString();
}
