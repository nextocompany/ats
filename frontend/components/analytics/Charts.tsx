"use client";

import { ArrowUpRight, ArrowDownRight } from "lucide-react";

import type { Funnel, KPI, Source } from "@/lib/types";

const fmt = new Intl.NumberFormat("en-US");

/* ──────────────────────────────────────────────────────────────────────
   KPI band — editorial hierarchy: one dominant primary metric (emerald),
   three supporting metrics demoted to a hairline-divided strip.
   ────────────────────────────────────────────────────────────────────── */

interface Metric {
  label: string;
  value: number;
  hint: string;
  delta?: number;
}

export function KpiCards({
  kpi,
  variant = "hero",
}: {
  kpi: KPI;
  /** "hero" = bold filled-emerald band (Overview). "reporting" = compact outline strip (Analytics). */
  variant?: "hero" | "reporting";
}) {
  // Derived, presentational deltas (share of pipeline) — no data changes.
  const passRate = kpi.applied > 0 ? Math.round((kpi.passed / kpi.applied) * 100) : 0;
  const onboardRate = kpi.passed > 0 ? Math.round((kpi.onboarded / kpi.passed) * 100) : 0;

  const supporting: Metric[] = [
    { label: "Passed AI gate", value: kpi.passed, hint: `${passRate}% of applied`, delta: passRate },
    { label: "Onboarded", value: kpi.onboarded, hint: `${onboardRate}% of passed`, delta: onboardRate },
    { label: "Awaiting review", value: kpi.waiting, hint: "needs an operator" },
  ];

  if (variant === "reporting") {
    return <KpiStrip kpi={kpi} supporting={supporting} />;
  }

  return (
    <section
      aria-label="Key metrics"
      className="grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline lg:grid-cols-[1.35fr_2fr]"
    >
      {/* Primary — dominant, emerald, with a brass keyline as the premium accent.
          Padding + corner tick + left keyline share one optical inset scale so the
          brass marks line up consistently from 320 → 1440 (no cramped wrap at 320). */}
      <div className="relative flex flex-col justify-between bg-brand px-5 py-6 text-brand-foreground sm:px-7">
        {/* Brass left keyline — the single confident gold emphasis on the hero number */}
        <span
          aria-hidden
          className="absolute inset-y-6 left-0 w-[3px] rounded-full"
          style={{ background: "var(--brass)" }}
        />
        {/* Disciplined flat keyline mark — a quiet brass corner tick, no soft glow.
            Inset matches the panel padding so the tick sits a consistent gutter in. */}
        <span
          aria-hidden
          className="pointer-events-none absolute right-4 top-6 size-3 border-r border-t opacity-50 sm:right-5"
          style={{ borderColor: "var(--brass)" }}
        />
        <p className="pl-3.5 text-[0.6875rem] font-semibold uppercase tracking-[0.16em] text-brand-foreground/70">
          Total applications
        </p>
        <div className="mt-3 pl-3.5">
          <span className="block font-semibold tabular-nums leading-none [font-size:var(--text-stat)] tracking-tight">
            {fmt.format(kpi.applied)}
          </span>
          <p className="mt-3 text-sm text-brand-foreground/80">
            Across all stores · current intake cycle
          </p>
        </div>
      </div>

      {/* Supporting strip */}
      <div className="grid grid-cols-1 gap-px bg-hairline sm:grid-cols-3">
        {supporting.map((m) => (
          <div key={m.label} className="flex flex-col justify-between bg-card px-5 py-5">
            <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
              {m.label}
            </p>
            <div className="mt-2">
              <span className="block text-3xl font-semibold tabular-nums leading-none tracking-tight text-foreground">
                {fmt.format(m.value)}
              </span>
              <div className="mt-2 flex items-center gap-1 text-xs text-muted-foreground">
                {typeof m.delta === "number" && (
                  <span
                    className={`inline-flex items-center gap-0.5 font-medium ${
                      m.delta >= 50 ? "text-brand" : "text-muted-foreground"
                    }`}
                  >
                    {m.delta >= 50 ? (
                      <ArrowUpRight className="size-3.5" />
                    ) : (
                      <ArrowDownRight className="size-3.5" />
                    )}
                    {m.delta}%
                  </span>
                )}
                <span>{typeof m.delta === "number" ? "" : m.hint}</span>
                {typeof m.delta === "number" && <span className="text-muted-foreground/70">conversion</span>}
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ──────────────────────────────────────────────────────────────────────
   Reporting stat strip (Analytics) — a compact, hairline, outline variant
   of the KPI band. No big emerald fill; reads as a report header with a
   cycle/period label and "vs previous cycle" framing, so Analytics never
   shares Overview's bold hero treatment.
   ────────────────────────────────────────────────────────────────────── */

function KpiStrip({ kpi, supporting }: { kpi: KPI; supporting: Metric[] }) {
  return (
    <section
      aria-label="Key metrics"
      className="rounded-xl bg-card ring-1 ring-hairline"
    >
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1 border-b border-hairline px-5 py-3">
        <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
          Current intake cycle
        </p>
        <p className="text-xs text-muted-foreground">
          Figures shown vs previous cycle
        </p>
      </div>
      <dl className="grid grid-cols-2 divide-hairline sm:grid-cols-4 sm:divide-x">
        {/* Lead metric — emphasized by scale + a brass keyline, not by fill */}
        <div className="relative border-b border-hairline px-5 py-4 sm:border-b-0">
          <span
            aria-hidden
            className="absolute inset-y-4 left-0 w-[2px] rounded-full"
            style={{ background: "var(--brass)" }}
          />
          <dt className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
            Total applications
          </dt>
          <dd className="mt-1.5 text-2xl font-semibold tabular-nums leading-none tracking-tight text-foreground">
            {fmt.format(kpi.applied)}
          </dd>
        </div>
        {supporting.map((m, i) => (
          <div
            key={m.label}
            className={`px-5 py-4 ${i < supporting.length - 1 ? "border-b border-hairline sm:border-b-0" : ""}`}
          >
            <dt className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
              {m.label}
            </dt>
            <dd className="mt-1.5 flex items-baseline gap-2">
              <span className="text-2xl font-semibold tabular-nums leading-none tracking-tight text-foreground">
                {fmt.format(m.value)}
              </span>
              <span className="text-xs text-muted-foreground">{m.hint}</span>
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

/* ──────────────────────────────────────────────────────────────────────
   Recruitment funnel — custom, brand-colored, honestly proportional.
   Each band's width maps to its real value relative to Applied (with a MIN
   floor so small stages stay labelable). Stage labels ride ABOVE each band
   so every stage is self-describing at any width — including 320 — and never
   collapses to a bare number. Trapezoid connectors carry the step-conversion
   delta; Hired terminates in a brass accent. Pure CSS widths/clip-path.
   ────────────────────────────────────────────────────────────────────── */

const FUNNEL_STAGES = [
  { key: "applied", label: "Applied" },
  { key: "passed_ai", label: "Passed AI" },
  { key: "reviewed", label: "Reviewed" },
  { key: "hired", label: "Hired" },
] as const;

// Floor so even a 2-of-23 stage keeps a legible, brass-terminated band.
// Honest proportion drives the rest: width = (value / appliedMax).
const FUNNEL_MIN_WIDTH = 16;

export function FunnelChart({ funnel }: { funnel: Funnel }) {
  // Applied is the top of funnel and the proportional reference.
  const max = Math.max(funnel.applied, 1);
  const endToEnd = max > 0 ? Math.round((funnel.hired / max) * 100) : 0;

  const widthFor = (value: number) =>
    Math.min(100, Math.max((value / max) * 100, FUNNEL_MIN_WIDTH));

  return (
    <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-6 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">Pipeline</p>
          <h2 className="mt-3 font-heading text-lg font-semibold tracking-tight">Recruitment Funnel</h2>
        </div>
        <span className="text-xs text-muted-foreground tabular-nums">
          {endToEnd}% end-to-end
        </span>
      </header>

      {/* Centered, converging tapered bands. Width is honestly proportional to
          each stage's value vs. Applied, so Hired (small) is clearly narrower
          than Reviewed, which is clearly narrower than Applied. The label sits
          above the band — always legible, never truncated, even at 320. */}
      <ol className="flex flex-col">
        {FUNNEL_STAGES.map((stage, i) => {
          const value = funnel[stage.key];
          const widthPct = widthFor(value);
          const prev = i > 0 ? funnel[FUNNEL_STAGES[i - 1].key] : value;
          const prevWidthPct = i > 0 ? widthFor(prev) : widthPct;
          const step = prev > 0 ? Math.round((value / prev) * 100) : 100;
          const isHired = i === FUNNEL_STAGES.length - 1;
          // Emerald tonal ramp deepening toward the point of the funnel.
          const shade = `oklch(${42 - i * 2.4}% ${0.088 - i * 0.006} 162)`;

          return (
            <li key={stage.key}>
              {/* Converging connector — a clipped trapezoid that visually steps
                  from the previous (wider) band down to this (narrower) one. */}
              {i > 0 && (
                <div className="relative h-5" aria-hidden>
                  <div
                    className="mx-auto h-full transition-[width] duration-500"
                    style={{
                      width: `${prevWidthPct}%`,
                      transitionTimingFunction: "var(--ease-out)",
                    }}
                  >
                    <div
                      className="h-full"
                      style={{
                        background: shade,
                        opacity: 0.16,
                        clipPath: `polygon(0 0, 100% 0, ${
                          50 + (widthPct / prevWidthPct) * 50
                        }% 100%, ${50 - (widthPct / prevWidthPct) * 50}% 100%)`,
                      }}
                    />
                  </div>
                  {/* Step-conversion delta riding the neck of the funnel */}
                  <span className="absolute inset-0 grid place-items-center text-[0.6875rem] font-medium tabular-nums text-muted-foreground">
                    {step}% pass
                  </span>
                </div>
              )}

              {/* Stage label ABOVE the band — width-independent, so the stage is
                  always self-describing (e.g. "Reviewed 5"), never a bare number. */}
              <div
                className="mx-auto mb-1 flex items-baseline justify-between gap-2 px-1"
                style={{ width: `${widthPct}%`, minWidth: "8.5rem" }}
              >
                <span className="text-[0.6875rem] font-semibold uppercase tracking-[0.1em] text-muted-foreground">
                  {stage.label}
                </span>
                <span className="flex items-baseline gap-1 tabular-nums">
                  <span className="text-sm font-semibold leading-none text-foreground">
                    {fmt.format(value)}
                  </span>
                  {isHired && value > 0 && (
                    <span
                      aria-hidden
                      className="ml-0.5 inline-block size-1.5 shrink-0 rounded-full"
                      style={{ background: "var(--brass)" }}
                    />
                  )}
                </span>
              </div>

              {/* The stage band itself — centered, value-proportional width.
                  Count is mirrored inside the band for the wider stages; the
                  label above guarantees legibility regardless of band width. */}
              <div
                className="relative mx-auto flex h-11 items-center justify-end rounded-md px-4 text-brand-foreground transition-[width] duration-500"
                style={{
                  width: `${widthPct}%`,
                  minWidth: "8.5rem",
                  background: shade,
                  transitionTimingFunction: "var(--ease-out)",
                }}
              >
                {isHired && value > 0 && (
                  <span
                    aria-hidden
                    className="absolute inset-y-2 left-0 w-[3px] rounded-full"
                    style={{ background: "var(--brass)" }}
                  />
                )}
                <span className="text-base font-semibold leading-none tabular-nums">
                  {fmt.format(value)}
                </span>
              </div>

              {isHired && (
                <p
                  className="mx-auto mt-1.5 text-[0.6875rem] uppercase tracking-[0.12em] text-brass"
                  style={{ width: `${widthPct}%`, minWidth: "8.5rem" }}
                >
                  {endToEnd}% end-to-end
                </p>
              )}
            </li>
          );
        })}
      </ol>
    </section>
  );
}

/* ──────────────────────────────────────────────────────────────────────
   Sourcing efficiency — channel rows ranked by volume, with a conversion
   read. Brass marks the best converter (the one premium accent moment).
   ────────────────────────────────────────────────────────────────────── */

export function SourcesChart({ sources }: { sources: Source[] }) {
  const ranked = [...sources].sort((a, b) => b.applied - a.applied);
  const maxApplied = Math.max(...ranked.map((s) => s.applied), 1);
  const bestConv = Math.max(...ranked.map((s) => s.conversion), 0);

  return (
    <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">Channels</p>
          <h2 className="mt-3 font-heading text-lg font-semibold tracking-tight">Sourcing Efficiency</h2>
        </div>
        <span className="text-xs text-muted-foreground">volume · conversion</span>
      </header>

      {ranked.length === 0 ? (
        <p className="py-8 text-center text-sm text-muted-foreground">No sourcing data yet.</p>
      ) : (
        // Substantial emerald-filled bars on the same shaded track as the funnel —
        // so funnel + sources read as ONE data-viz system. The channel label and
        // count ride inside/atop the bar; conversion is the deliberate secondary metric.
        <ol className="flex flex-col gap-4">
          {ranked.map((s, i) => {
            const widthPct = Math.max((s.applied / maxApplied) * 100, 4);
            const conv = Math.round(s.conversion * 100);
            const isBest = s.conversion === bestConv && bestConv > 0;
            // Match the funnel's tonal step so the two charts share a ramp.
            const shade = `oklch(${40.5 - i * 1.4}% ${0.085 - i * 0.004} 162)`;
            return (
              <li key={s.channel}>
                <div className="mb-1.5 flex items-baseline justify-between text-sm">
                  <span className="font-medium capitalize text-foreground">{s.channel}</span>
                  <span className="flex items-baseline gap-2 tabular-nums">
                    <span className="font-semibold text-foreground">{fmt.format(s.applied)}</span>
                    <span
                      className={`text-xs ${
                        isBest ? "font-semibold text-brass" : "text-muted-foreground"
                      }`}
                    >
                      {conv}% conv{isBest ? " · best" : ""}
                    </span>
                  </span>
                </div>
                <div className="h-7 w-full overflow-hidden rounded-md bg-muted">
                  <div
                    className="flex h-full items-center justify-end rounded-md pr-2 transition-[width] duration-500"
                    style={{
                      width: `${widthPct}%`,
                      backgroundColor: shade,
                      transitionTimingFunction: "var(--ease-out)",
                    }}
                  >
                    {isBest && (
                      <span
                        aria-hidden
                        className="inline-block size-1.5 rounded-full"
                        style={{ background: "var(--brass)" }}
                      />
                    )}
                  </div>
                </div>
              </li>
            );
          })}
        </ol>
      )}
    </section>
  );
}
