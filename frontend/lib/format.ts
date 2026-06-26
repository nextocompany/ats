// Shared formatting helpers.

// formatDateTime renders an ISO timestamp in the user's locale, or "-" when the
// value is absent or unparseable.
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "-";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "-" : d.toLocaleString();
}

const numFmt = new Intl.NumberFormat("en-US");

// formatNum renders an integer with thousands separators.
export function formatNum(n: number): string {
  return numFmt.format(Math.round(n));
}

// formatMoney renders a whole-currency amount with its currency code (compact,
// no decimals — these are leadership figures, not invoices).
export function formatMoney(n: number, currency: string): string {
  const code = currency || "THB";
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: code,
      maximumFractionDigits: 0,
    }).format(n);
  } catch {
    return `${numFmt.format(Math.round(n))} ${code}`;
  }
}
