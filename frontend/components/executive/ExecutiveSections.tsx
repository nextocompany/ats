"use client";

import { useTranslations } from "next-intl";
import { Store, Briefcase } from "lucide-react";

import { Pill } from "@/components/people/PeopleBits";
import type { ExecutivePipelinePosition, ExecutiveStoreFill } from "@/lib/types";

const fmt = new Intl.NumberFormat("en-US");

/* ──────────────────────────────────────────────────────────────────────────
   Executive Overview board sections - the "most short-staffed branches" ranking
   and pipeline-by-position, plus shared helpers (EmptyState, fillShade) reused
   by the headcount panel. Same ledger language as the operational dashboard
   (CP Axtra blue surfaces, hairline rings, tabular nums); fill-rate bars warm
   from blue → amber → clay as a branch falls behind budget.
   ────────────────────────────────────────────────────────────────────────── */

// Fill-rate → bar colour. Healthy branches read CP Axtra blue; under-staffed
// branches warm to amber then clay so the eye lands on the worst first.
export function fillShade(pct: number): string {
  if (pct < 70) return "var(--score-low)";
  if (pct < 85) return "var(--score-mid)";
  return "var(--brand)";
}

/* ── Most short-staffed branches - a board table ranked ascending by fill-rate
   (worst first), the question "which branch needs people now?". The fill/short
   columns are budget-derived, so they show a pending-HRIS placeholder (branch
   names stay real) until PeopleSoft is wired. ────────────────────────────── */
export function ShortStaffedPanel({
  stores,
  budgetAvailable,
}: {
  stores: ExecutiveStoreFill[];
  budgetAvailable: boolean;
}) {
  const t = useTranslations("executive");
  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">{t("staffing")}</p>
          <h2 className="mt-3 flex items-center gap-2 font-heading text-lg font-semibold tracking-tight">
            <span aria-hidden className="text-muted-foreground">
              <Store className="size-4" strokeWidth={1.75} />
            </span>
            {t("mostShortStaffed")}
          </h2>
        </div>
        {budgetAvailable ? (
          <span className="text-xs tabular-nums text-muted-foreground">{t("storesCount", { count: stores.length })}</span>
        ) : (
          <Pill tone="neutral">{t("pendingHrisShort")}</Pill>
        )}
      </header>

      {stores.length === 0 ? (
        <EmptyState
          icon={<Store className="size-4" strokeWidth={1.75} />}
          title={t("noStaffingData")}
          hint={t("noStaffingHint")}
        />
      ) : (
        <table className="w-full text-sm" aria-label={t("mostShortStaffed")}>
          <thead className="ledger-head">
            <tr>
              <th className="px-3 py-2 text-right">{t("rank")}</th>
              <th className="px-3 py-2 text-left">{t("colBranch")}</th>
              <th className="px-3 py-2 text-left">{t("colSubregion")}</th>
              <th className="px-3 py-2 text-right">{t("colShort")}</th>
              <th className="px-3 py-2 text-right">{t("colFill")}</th>
            </tr>
          </thead>
          <tbody>
            {stores.map((s, i) => (
              <tr key={s.store_no} className="ledger-row border-b border-hairline last:border-0">
                <td className="px-3 py-2.5 text-right tabular-nums text-muted-foreground">{i + 1}</td>
                <td className="px-3 py-2.5 font-medium text-foreground">{s.store_name}</td>
                <td className="px-3 py-2.5 text-xs text-muted-foreground">{s.subregion || "-"}</td>
                <td className="px-3 py-2.5 text-right tabular-nums">
                  {budgetAvailable && s.heads_short > 0 ? (
                    <span className="text-[var(--score-low)]">{t("headsShort", { n: fmt.format(s.heads_short) })}</span>
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </td>
                <td className="px-3 py-2.5 text-right">
                  {budgetAvailable ? (
                    <span className="inline-flex items-center justify-end gap-2 tabular-nums">
                      <span className="font-semibold text-foreground">{s.fill_rate_pct}%</span>
                      <span className="h-2 w-16 overflow-hidden rounded-full bg-muted" aria-hidden>
                        <span
                          className="block h-full rounded-full"
                          style={{
                            width: `${Math.max(Math.min(s.fill_rate_pct, 100), 4)}%`,
                            backgroundColor: fillShade(s.fill_rate_pct),
                          }}
                        />
                      </span>
                    </span>
                  ) : (
                    <span className="text-muted-foreground">-</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

/* ── Pipeline by position - compact funnel (applied → screening → interview →
   offer → hired) per role, with open headcount. ─────────────────────────── */
export function PipelinePanel({ rows }: { rows: ExecutivePipelinePosition[] }) {
  const t = useTranslations("executive");
  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">{t("hiring")}</p>
          <h2 className="mt-3 flex items-center gap-2 font-heading text-lg font-semibold tracking-tight">
            <span aria-hidden className="text-muted-foreground">
              <Briefcase className="size-4" strokeWidth={1.75} />
            </span>
            {t("pipelineByPosition")}
          </h2>
        </div>
        <span className="text-xs tabular-nums text-muted-foreground">{t("appliedToHired")}</span>
      </header>

      {rows.length === 0 ? (
        <EmptyState
          icon={<Briefcase className="size-4" strokeWidth={1.75} />}
          title={t("noPipeline")}
          hint={t("noPipelineHint")}
        />
      ) : (
        <ol className="flex flex-col divide-y divide-hairline">
          {rows.map((p) => (
            <li key={p.position_id} className="flex items-center justify-between gap-3 py-3">
              <span className="flex min-w-0 items-baseline gap-2">
                <span className="truncate text-sm font-medium text-foreground">{p.title}</span>
                <span className="shrink-0 text-xs text-muted-foreground">{t("openCount", { n: p.openings })}</span>
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

export function EmptyState({ icon, title, hint }: { icon: React.ReactNode; title: string; hint: string }) {
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

// DataSourceBadge keeps the executive honest about the data behind the board:
// synthetic figures (mock mode), a budget still pending PeopleSoft/HRIS (live but
// no budget), or fully-live data. Pending is neutral, never an error colour.
export function DataSourceBadge({
  source,
  budgetAvailable,
  demoLabel,
  pendingLabel,
  liveLabel,
}: {
  source?: "mock" | "live";
  budgetAvailable?: boolean;
  demoLabel: string;
  pendingLabel: string;
  liveLabel: string;
}) {
  if (source === "mock") return <Pill tone="pending">{demoLabel}</Pill>;
  if (source === "live" && budgetAvailable === false) return <Pill tone="neutral">{pendingLabel}</Pill>;
  if (source === "live") return <Pill tone="neutral">{liveLabel}</Pill>;
  return null;
}
