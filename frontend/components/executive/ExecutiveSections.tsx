"use client";

import { Store, Briefcase } from "lucide-react";

import { Pill } from "@/components/people/PeopleBits";
import type { ExecutiveCompany, ExecutivePipelinePosition, ExecutiveStoreFill } from "@/lib/types";

const fmt = new Intl.NumberFormat("en-US");

/* ──────────────────────────────────────────────────────────────────────────
   Executive Overview sections — company headcount band, "most short-staffed
   branches" ranking, and pipeline-by-position. Same ledger language as the
   operational dashboard (CP Axtra blue surfaces, hairline rings, tabular nums);
   fill-rate bars warm from blue → amber → clay as a branch falls behind budget.
   ────────────────────────────────────────────────────────────────────────── */

// Fill-rate → bar colour. Healthy branches read CP Axtra blue; under-staffed
// branches warm to amber then clay so the eye lands on the worst first.
function fillShade(pct: number): string {
  if (pct < 70) return "var(--score-low)";
  if (pct < 85) return "var(--score-mid)";
  return "var(--brand)";
}

const dash = "-";

/* ── Company headcount band — one dominant figure + three supporting metrics,
   mirroring the KPI hero band. Falls back to em-dashes when budget is pending
   (live mode before PeopleSoft is wired). ───────────────────────────────── */
export function HeadcountBand({ company }: { company: ExecutiveCompany }) {
  const hasBudget = company.budget_available;
  const supporting: { label: string; value: string; hint: string }[] = [
    {
      label: "Budgeted",
      value: hasBudget ? fmt.format(company.budget_headcount) : dash,
      hint: hasBudget ? "approved headcount" : "pending HRIS",
    },
    {
      label: "Vacancy",
      value: fmt.format(company.vacancy),
      hint: hasBudget ? "open positions" : "open vacancies",
    },
    {
      label: "Fill rate",
      value: hasBudget ? `${company.fill_rate_pct}%` : dash,
      hint: hasBudget ? "of budget filled" : "pending HRIS",
    },
  ];

  return (
    <section
      aria-label="Company headcount"
      className="grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline lg:grid-cols-[1.35fr_2fr]"
    >
      <div className="relative flex flex-col justify-between bg-brand px-5 py-6 text-brand-foreground sm:px-7">
        <span
          aria-hidden
          className="absolute inset-y-6 left-0 w-[3px] rounded-full"
          style={{ background: "var(--brass)" }}
        />
        <p className="pl-3.5 text-[0.6875rem] font-semibold uppercase tracking-[0.16em] text-brand-foreground/70">
          Actual headcount
        </p>
        <div className="mt-3 pl-3.5">
          <span className="num block font-semibold tabular-nums leading-none [font-size:var(--text-stat)] tracking-tight">
            {fmt.format(company.actual_headcount)}
          </span>
          <p className="mt-3 text-sm text-brand-foreground/80">
            {hasBudget
              ? `of ${fmt.format(company.budget_headcount)} budgeted · ${company.fill_rate_pct}% filled`
              : "across all stores · budget pending HRIS"}
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-px bg-hairline sm:grid-cols-3">
        {supporting.map((m) => (
          <div key={m.label} className="flex flex-col justify-between bg-card px-5 py-5">
            <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
              {m.label}
            </p>
            <div className="mt-2">
              <span className="num block text-3xl font-semibold tabular-nums leading-none tracking-tight text-foreground">
                {m.value}
              </span>
              <p className="mt-2 text-xs text-muted-foreground">{m.hint}</p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ── Most short-staffed branches — ranked ascending by fill-rate (worst first),
   exactly the question "which branch needs people now?". ─────────────────── */
export function ShortStaffedPanel({ stores }: { stores: ExecutiveStoreFill[] }) {
  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">Staffing</p>
          <h2 className="mt-3 flex items-center gap-2 font-heading text-lg font-semibold tracking-tight">
            <span aria-hidden className="text-muted-foreground">
              <Store className="size-4" strokeWidth={1.75} />
            </span>
            Most short-staffed branches
          </h2>
        </div>
        <span className="text-xs tabular-nums text-muted-foreground">{stores.length} stores</span>
      </header>

      {stores.length === 0 ? (
        <EmptyState
          icon={<Store className="size-4" strokeWidth={1.75} />}
          title="No staffing data"
          hint="Branch fill-rate appears here once headcount targets and hires are available."
        />
      ) : (
        <ol className="flex flex-col gap-3">
          {stores.map((s) => (
            <li key={s.store_no} className="-mx-2 px-2 py-1">
              <div className="mb-1.5 flex items-baseline justify-between gap-2 text-sm">
                <span className="flex min-w-0 items-baseline gap-2">
                  <span className="truncate font-medium text-foreground">{s.store_name}</span>
                  {s.subregion && (
                    <span className="shrink-0 text-xs text-muted-foreground">{s.subregion}</span>
                  )}
                </span>
                <span className="flex shrink-0 items-baseline gap-2 tabular-nums">
                  <span className="font-semibold text-foreground">{s.fill_rate_pct}%</span>
                  {s.heads_short > 0 && (
                    <span className="text-xs text-[var(--score-low)]">-{fmt.format(s.heads_short)} heads</span>
                  )}
                </span>
              </div>
              <div className="h-2.5 w-full overflow-hidden rounded-full bg-muted">
                <div
                  className="h-full rounded-full transition-[width] duration-500"
                  style={{
                    width: `${Math.max(Math.min(s.fill_rate_pct, 100), 4)}%`,
                    backgroundColor: fillShade(s.fill_rate_pct),
                    transitionTimingFunction: "var(--ease-out)",
                  }}
                />
              </div>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

/* ── Pipeline by position — compact funnel (applied → screening → interview →
   offer → hired) per role, with open headcount. ─────────────────────────── */
export function PipelinePanel({ rows }: { rows: ExecutivePipelinePosition[] }) {
  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">Hiring</p>
          <h2 className="mt-3 flex items-center gap-2 font-heading text-lg font-semibold tracking-tight">
            <span aria-hidden className="text-muted-foreground">
              <Briefcase className="size-4" strokeWidth={1.75} />
            </span>
            Pipeline by position
          </h2>
        </div>
        <span className="text-xs tabular-nums text-muted-foreground">applied ▸ hired</span>
      </header>

      {rows.length === 0 ? (
        <EmptyState
          icon={<Briefcase className="size-4" strokeWidth={1.75} />}
          title="No pipeline yet"
          hint="Applications by position appear here as candidates move through screening to hire."
        />
      ) : (
        <ol className="flex flex-col divide-y divide-hairline">
          {rows.map((p) => (
            <li key={p.position_id} className="flex items-center justify-between gap-3 py-3">
              <span className="flex min-w-0 items-baseline gap-2">
                <span className="truncate text-sm font-medium text-foreground">{p.title}</span>
                <span className="shrink-0 text-xs text-muted-foreground">{p.openings} open</span>
              </span>
              <span className="flex shrink-0 items-center gap-1 text-xs tabular-nums text-muted-foreground">
                <Stage value={p.applied} />
                <Sep />
                <Stage value={p.screening} />
                <Sep />
                <Stage value={p.interview} />
                <Sep />
                <Stage value={p.offer} />
                <Sep />
                <Stage value={p.hired} strong />
              </span>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}

function Stage({ value, strong = false }: { value: number; strong?: boolean }) {
  return (
    <span className={strong ? "font-semibold text-brand" : "text-foreground"}>{fmt.format(value)}</span>
  );
}

function Sep() {
  return <span aria-hidden className="text-muted-foreground/40">▸</span>;
}

function EmptyState({ icon, title, hint }: { icon: React.ReactNode; title: string; hint: string }) {
  return (
    <div className="flex flex-col items-center justify-center py-12 text-center">
      <span aria-hidden className="mb-4 grid size-11 place-items-center rounded-2xl bg-brand-soft text-brand">
        {icon}
      </span>
      <p className="text-sm font-semibold text-foreground">{title}</p>
      <p className="mx-auto mt-1 max-w-xs text-xs text-muted-foreground">{hint}</p>
    </div>
  );
}

// DemoBadge keeps the executive honest about synthetic figures (mock mode) or a
// budget that is still pending the PeopleSoft/HRIS integration (live mode).
export function DemoBadge({
  source,
  budgetAvailable,
  demoLabel,
  budgetPendingLabel,
}: {
  source?: "mock" | "live";
  budgetAvailable?: boolean;
  demoLabel: string;
  budgetPendingLabel: string;
}) {
  if (source === "mock") return <Pill tone="pending">{demoLabel}</Pill>;
  if (source === "live" && budgetAvailable === false) return <Pill tone="neutral">{budgetPendingLabel}</Pill>;
  return null;
}
