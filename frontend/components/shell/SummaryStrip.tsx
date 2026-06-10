import type { ReactNode } from "react";

export interface SummaryStat {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  /** Visually promote the first stat as the strip's lead figure. */
  lead?: boolean;
  /** brass keyline emphasis on the lead stat */
  accent?: boolean;
}

/* Header summary strip — a hairline-divided band of figures that sits under a
   PageHeader so a one-row table never reads as "broken/empty". The lead stat is
   scaled up with a brass keyline; the rest are quieter satellites. Shared across
   Inbox + Candidates so list surfaces carry the same ledger language. */
export function SummaryStrip({ stats }: { stats: SummaryStat[] }) {
  return (
    // Hairline-gridded via a 1px gap over a hairline background — no per-cell
    // border math, so the strip stays clean at every breakpoint.
    <section
      aria-label="Summary"
      className="grid grid-cols-2 gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline sm:grid-cols-4"
    >
      {stats.map((s) => (
        <div key={s.label} className="relative bg-card px-5 py-4">
          {s.accent && (
            <span
              aria-hidden
              className="absolute inset-y-4 left-0 w-[2px] rounded-full"
              style={{ background: "var(--brass)" }}
            />
          )}
          <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.13em] text-muted-foreground">
            {s.label}
          </p>
          <p
            className={`mt-1.5 font-semibold tabular-nums leading-none tracking-tight text-foreground ${
              s.lead ? "text-[2rem]" : "text-2xl"
            }`}
          >
            {s.value}
          </p>
          {s.hint && <p className="mt-1.5 text-xs text-muted-foreground">{s.hint}</p>}
        </div>
      ))}
    </section>
  );
}
