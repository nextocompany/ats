import { cn } from "@/lib/utils";

import { Eyebrow } from "@/components/ds/Eyebrow";

export interface AccountSummaryFigure {
  // The prominent value, e.g. "80%", "3 / 5", "เชื่อมแล้ว", "มิ.ย. 2026".
  value: string;
  // Whether the value is a tabular figure (gets the numeral treatment) or text.
  numeric?: boolean;
  // The quiet label below, e.g. "ความสมบูรณ์ของโปรไฟล์".
  label: string;
}

interface AccountIdentityHeaderProps {
  name: string;
  // Primary contact line under the name (email or LINE display id).
  contact: string;
  // Optional secondary contact note (e.g. LINE id when email is primary).
  secondary?: string;
  // At-a-glance summary figures shown as a divided figure row (StatBand idiom).
  summary: AccountSummaryFigure[];
  className?: string;
}

// AccountIdentityHeader is the page's identity unit: an avatar-initial mark, the
// member's name as the page title (h1), contact lines, and a divided figure row of
// at-a-glance facts. It reuses the AccountNav avatar-initial pattern and the
// StatBand figure idiom (tabular numerals, hairline dividers) without forcing
// non-numeric facts into the oversized stat numeral slot.
export function AccountIdentityHeader({
  name,
  contact,
  secondary,
  summary,
  className,
}: AccountIdentityHeaderProps) {
  const initial = name.trim().charAt(0).toUpperCase() || "?";

  return (
    <header className={cn("flex flex-col gap-8", className)}>
      <div className="flex flex-col gap-5 sm:flex-row sm:items-center sm:gap-6">
        <span
          aria-hidden="true"
          className="grid size-16 shrink-0 place-content-center rounded-full border border-line bg-secondary text-2xl font-semibold text-foreground sm:size-20"
        >
          {initial}
        </span>
        <div className="flex min-w-0 flex-col gap-2">
          <Eyebrow>บัญชีของฉัน</Eyebrow>
          <h1 className="[font-size:var(--text-h2)] font-semibold leading-[1.1] text-foreground">
            {name}
          </h1>
          <div className="flex flex-col gap-0.5">
            <p className="truncate text-sm text-muted-foreground">{contact}</p>
            {secondary ? (
              <p className="truncate text-sm text-muted-foreground">{secondary}</p>
            ) : null}
          </div>
        </div>
      </div>

      <AccountSummaryRow summary={summary} />
    </header>
  );
}

// AccountSummaryRow renders the at-a-glance facts as a hairline-divided row of
// figures — the StatBand idiom (tabular nums, quiet labels) but sized for mixed
// numeric/text facts so "LINE: เชื่อมแล้ว" does not get the giant numeral.
function AccountSummaryRow({ summary }: { summary: AccountSummaryFigure[] }) {
  return (
    <dl className="grid grid-cols-2 gap-px overflow-hidden rounded-xl border border-line bg-line xl:grid-cols-4">
      {summary.map((item) => (
        <div key={item.label} className="flex flex-col gap-1.5 bg-card px-5 py-5">
          <dd
            className={cn(
              "font-semibold leading-none tracking-tight text-foreground",
              item.numeric
                ? "num [font-size:var(--text-h3)] tabular-nums"
                : "[font-size:var(--text-h3)]",
            )}
          >
            {item.value}
          </dd>
          <dt className="text-xs font-medium leading-snug text-muted-foreground">
            {item.label}
          </dt>
        </div>
      ))}
    </dl>
  );
}
