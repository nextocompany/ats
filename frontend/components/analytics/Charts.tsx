"use client";

import { useTranslations } from "next-intl";
import { ArrowUpRight } from "lucide-react";

import type { Funnel, KPI, Source } from "@/lib/types";

const fmt = new Intl.NumberFormat("en-US");

/* ──────────────────────────────────────────────────────────────────────
   KPI band — editorial hierarchy: one dominant primary metric (CP Axtra blue),
   three supporting metrics demoted to a hairline-divided strip.
   ────────────────────────────────────────────────────────────────────── */

interface Metric {
  label: string;
  value: number;
  hint: string;
}

export function KpiCards({
  kpi,
  variant = "hero",
}: {
  kpi: KPI;
  /** "hero" = bold filled CP Axtra blue band (Overview). "reporting" = compact outline strip (Analytics). */
  variant?: "hero" | "reporting";
}) {
  const t = useTranslations("analytics");
  // Derived, presentational deltas (share of pipeline) — no data changes.
  const passRate = kpi.applied > 0 ? Math.round((kpi.passed / kpi.applied) * 100) : 0;
  const onboardRate = kpi.passed > 0 ? Math.round((kpi.onboarded / kpi.passed) * 100) : 0;

  const supporting: Metric[] = [
    { label: t("kpiPassed"), value: kpi.passed, hint: t("kpiPassedHint", { rate: passRate }) },
    { label: t("kpiOnboarded"), value: kpi.onboarded, hint: t("kpiOnboardedHint", { rate: onboardRate }) },
    { label: t("kpiWaiting"), value: kpi.waiting, hint: t("kpiWaitingHint") },
  ];

  if (variant === "reporting") {
    return <KpiStrip kpi={kpi} supporting={supporting} />;
  }

  return (
    <section
      aria-label="Key metrics"
      className="grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline lg:grid-cols-[1.35fr_2fr]"
    >
      {/* Primary — dominant, CP Axtra blue, with a keyline as the premium accent.
          Padding + corner tick + left keyline share one optical inset scale so the
          brass marks line up consistently from 320 → 1440 (no cramped wrap at 320). */}
      <div className="relative flex flex-col justify-between bg-brand px-5 py-6 text-brand-foreground sm:px-7">
        {/* Brass left keyline — the single confident gold emphasis on the hero number */}
        <span
          aria-hidden
          className="absolute inset-y-6 left-0 w-[3px] rounded-full"
          style={{ background: "var(--brass)" }}
        />
        <p className="pl-3.5 text-[0.6875rem] font-semibold uppercase tracking-[0.16em] text-brand-foreground/70">
          {t("kpiTotal")}
        </p>
        <div className="mt-3 pl-3.5">
          <span className="num block font-semibold tabular-nums leading-none [font-size:var(--text-stat)] tracking-tight">
            {fmt.format(kpi.applied)}
          </span>
          <p className="mt-3 text-sm text-brand-foreground/80">{t("kpiTotalHeroSub")}</p>
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
              <span className="num block text-3xl font-semibold tabular-nums leading-none tracking-tight text-foreground">
                {fmt.format(m.value)}
              </span>
              <p className="mt-2 text-xs text-muted-foreground">{m.hint}</p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ──────────────────────────────────────────────────────────────────────
   Reporting stat strip (Analytics) — a compact, hairline, outline variant
   of the KPI band. No big color fill; reads as a report header with a
   cycle/period label and "vs previous cycle" framing, so Analytics never
   shares Overview's bold hero treatment.
   ────────────────────────────────────────────────────────────────────── */

function KpiStrip({ kpi, supporting }: { kpi: KPI; supporting: Metric[] }) {
  const t = useTranslations("analytics");
  const passRate = kpi.applied > 0 ? Math.round((kpi.passed / kpi.applied) * 100) : 0;
  return (
    // Bento, not a flat 4-up: one dominant lead panel (Total applications) with a
    // satellite cluster of three quieter figures. Scale contrast + asymmetry carry
    // the hierarchy; the lead reads first, the rest support it.
    <section aria-label="Key metrics" className="grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline lg:grid-cols-[1.1fr_2fr]">
      {/* Lead panel — large figure, brass keyline, a slim pass-through bar so the
          headline number carries a glanceable read instead of sitting alone. */}
      <div className="relative flex flex-col justify-between bg-card px-6 py-6">
        <span
          aria-hidden
          className="absolute inset-y-6 left-0 w-[3px] rounded-full"
          style={{ background: "var(--brass)" }}
        />
        <div className="flex items-baseline justify-between pl-3.5">
          <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
            {t("kpiTotal")}
          </p>
          <p className="text-[0.6875rem] uppercase tracking-[0.12em] text-muted-foreground/70">
            {t("kpiThisCycle")}
          </p>
        </div>
        <div className="mt-4 pl-3.5">
          <span className="num block font-semibold tabular-nums leading-none [font-size:var(--text-stat)] tracking-tight text-foreground">
            {fmt.format(kpi.applied)}
          </span>
          <div className="mt-4 flex items-center gap-2.5">
            <span aria-hidden className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted">
              <span
                className="block h-full origin-left rounded-full bg-brand transition-transform duration-700"
                style={{ transform: `scaleX(${passRate / 100})`, transitionTimingFunction: "var(--ease-out)" }}
              />
            </span>
            <span className="text-xs font-medium tabular-nums text-muted-foreground">
              {t("kpiClearScreening", { rate: passRate })}
            </span>
          </div>
        </div>
      </div>

      {/* Satellite cluster — three supporting figures, equal weight to each other
          but all clearly subordinate to the lead. Hairline-gridded. */}
      <dl className="grid grid-cols-1 gap-px bg-hairline sm:grid-cols-3">
        {supporting.map((m) => (
          <div key={m.label} className="flex flex-col justify-between bg-card px-5 py-5">
            <dt className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
              {m.label}
            </dt>
            <dd className="mt-2">
              <span className="num block text-3xl font-semibold tabular-nums leading-none tracking-tight text-foreground">
                {fmt.format(m.value)}
              </span>
              <span className="mt-2 block text-xs text-muted-foreground">{m.hint}</span>
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
  { key: "applied", labelKey: "stageApplied" },
  { key: "passed_ai", labelKey: "stagePassed" },
  { key: "reviewed", labelKey: "stageReviewed" },
  { key: "hired", labelKey: "stageHired" },
] as const;

// Floor so even a 2-of-23 stage keeps a legible, brass-terminated band.
// Honest proportion drives the rest: width = (value / appliedMax).
const FUNNEL_MIN_WIDTH = 16;

// Per-stage tonal ramp: a deliberate blue → brass descent down the funnel so
// the four stages never read as near-identical blue fills. Deep CP Axtra blue at
// the mouth, warming through to a brass-leaning tone at Hired (the conversion
// win). Hue sweeps 264 (blue) → 86 (brass), lightness lifts, chroma holds.
const FUNNEL_SHADES = [
  "oklch(46% 0.17 264)", // Applied — deep blue
  "oklch(53% 0.15 240)", // Passed AI — cooler mid-blue
  "oklch(62% 0.13 196)", // Reviewed — teal-blue, clearly lighter
  "oklch(72% 0.155 86)", // Hired — brass
] as const;

export function FunnelChart({ funnel }: { funnel: Funnel }) {
  const t = useTranslations("analytics");
  // Applied is the top of funnel and the proportional reference.
  const max = Math.max(funnel.applied, 1);
  const endToEnd = max > 0 ? Math.round((funnel.hired / max) * 100) : 0;

  // Honest widths: each band is its real share of Applied (floored so a tiny
  // stage stays labelable), then clamped to never exceed the stage above it —
  // so the silhouette narrows monotonically AND the width means something.
  const widths = FUNNEL_STAGES.reduce<number[]>((acc, stage, i) => {
    const honest = Math.min(100, Math.max((funnel[stage.key] / max) * 100, FUNNEL_MIN_WIDTH));
    acc.push(i === 0 ? honest : Math.min(honest, acc[i - 1]));
    return acc;
  }, []);

  return (
    <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-6 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">{t("funnelEyebrow")}</p>
          <h2 className="mt-3 font-heading text-lg font-semibold tracking-tight">{t("funnelTitle")}</h2>
        </div>
        <span className="text-xs text-muted-foreground tabular-nums">
          {t("funnelEndToEnd", { rate: endToEnd })}
        </span>
      </header>

      {/* Centered, converging tapered bands. Width is honestly proportional to
          each stage's value vs. Applied, so Hired (small) is clearly narrower
          than Reviewed, which is clearly narrower than Applied. The label sits
          above the band — always legible, never truncated, even at 320. */}
      <ol className="flex flex-col">
        {FUNNEL_STAGES.map((stage, i) => {
          const value = funnel[stage.key];
          const widthPct = widths[i];
          const prev = i > 0 ? funnel[FUNNEL_STAGES[i - 1].key] : value;
          const prevWidthPct = i > 0 ? widths[i - 1] : widthPct;
          const step = prev > 0 ? Math.round((value / prev) * 100) : 100;
          const isHired = i === FUNNEL_STAGES.length - 1;
          // Deliberate blue → brass ramp down the stages (see FUNNEL_SHADES).
          const shade = FUNNEL_SHADES[i];
          // The lighter, warmer lower bands need navy ink for AA contrast; the
          // deep upper blues carry white. Index 2+ flips to dark text.
          const bandInk = i >= 2 ? "oklch(22% 0.05 264)" : "var(--brand-foreground)";

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
                    {t("funnelStepPass", { rate: step })}
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
                  {t(stage.labelKey)}
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
                className="relative mx-auto flex h-11 items-center justify-end rounded-md px-4 transition-[width] duration-500"
                style={{
                  width: `${widthPct}%`,
                  minWidth: "8.5rem",
                  background: shade,
                  color: bandInk,
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
                  {t("funnelEndToEnd", { rate: endToEnd })}
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

// Tonal ramp from CP Axtra blue → brass across the ranked channels, so the
// panel reads as a deliberate scale (top channel deep blue, the tail warming
// toward brass) rather than a stack of identical bars. Clamped to one row too.
function channelShade(index: number, count: number): string {
  if (count <= 1) return "oklch(46% 0.18 264)";
  const t = index / (count - 1); // 0 → top, 1 → tail
  const l = 46 + t * 30; // lightness drifts up
  const c = 0.18 - t * 0.02;
  const h = 264 - t * 179; // blue → brass-ish hue path
  return `oklch(${l}% ${c} ${h})`;
}

export function SourcesChart({ sources }: { sources: Source[] }) {
  const t = useTranslations("analytics");
  const ranked = [...sources].sort((a, b) => b.applied - a.applied);
  const maxApplied = Math.max(...ranked.map((s) => s.applied), 1);
  const bestConv = Math.max(...ranked.map((s) => s.conversion), 0);
  const totalApplied = ranked.reduce((sum, s) => sum + s.applied, 0);
  const totalHired = ranked.reduce((sum, s) => sum + s.hired, 0);
  // Quarter ticks across the volume scale, so a single bar reads against an axis.
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((t) => Math.round(maxApplied * t));

  return (
    <section className="flex flex-col self-start rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">{t("sourcesEyebrow")}</p>
          <h2 className="mt-3 font-heading text-lg font-semibold tracking-tight">{t("sourcesTitle")}</h2>
        </div>
        <span className="hidden text-xs text-muted-foreground sm:inline">{t("sourcesAxis")}</span>
      </header>

      {ranked.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-14 text-center">
          <span
            aria-hidden
            className="mb-4 grid size-11 place-items-center rounded-2xl bg-brand-soft text-brand"
          >
            <ArrowUpRight className="size-5" />
          </span>
          <p className="text-sm font-semibold text-foreground">{t("sourcesEmptyTitle")}</p>
          <p className="mx-auto mt-1 max-w-xs text-xs text-muted-foreground">{t("sourcesEmptyHint")}</p>
          <span className="mx-auto mt-5 block h-px w-10 bg-hairline" aria-hidden />
        </div>
      ) : (
        <div className="flex flex-col">
          {/* Volume axis ticks — the bars now sit against a labeled scale, so
              even a single channel reads as a chart, not a floating bar. */}
          <div className="relative mb-2 h-4">
            {ticks.map((t, i) => (
              <span
                key={i}
                className="absolute -translate-x-1/2 text-[0.625rem] tabular-nums text-muted-foreground/70"
                style={{ left: `${(i / (ticks.length - 1)) * 100}%` }}
              >
                {fmt.format(t)}
              </span>
            ))}
          </div>

          <ol className="flex flex-col gap-3.5">
            {ranked.map((s, i) => {
              const widthPct = Math.max((s.applied / maxApplied) * 100, 6);
              const conv = Math.round(s.conversion * 100);
              const isBest = s.conversion === bestConv && bestConv > 0;
              const shade = channelShade(i, ranked.length);
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
                        {t("sourcesConv", { rate: conv })}
                        {isBest ? t("sourcesBestSuffix") : ""}
                      </span>
                    </span>
                  </div>
                  {/* Track carries faint gridlines aligned to the axis ticks. */}
                  <div className="relative h-7 w-full overflow-hidden rounded-md bg-muted">
                    {ticks.slice(1, -1).map((_, ti) => (
                      <span
                        key={ti}
                        aria-hidden
                        className="absolute inset-y-0 w-px bg-hairline/70"
                        style={{ left: `${((ti + 1) / (ticks.length - 1)) * 100}%` }}
                      />
                    ))}
                    <div
                      className="relative flex h-full items-center justify-end rounded-md pr-2 transition-[width] duration-500"
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

          {/* Footer read — total volume + blended conversion, plus the brass-best
              legend so the panel never bottoms out in empty space. */}
          <footer className="mt-5 flex items-center justify-between border-t border-hairline pt-4 text-xs">
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <span aria-hidden className="size-1.5 rounded-full" style={{ background: "var(--brass)" }} />
              {t("sourcesBestLegend")}
            </span>
            <span className="tabular-nums text-muted-foreground">
              {t.rich("sourcesFooter", {
                applied: fmt.format(totalApplied),
                hired: fmt.format(totalHired),
                b: (chunks) => <span className="font-semibold text-foreground">{chunks}</span>,
              })}
            </span>
          </footer>
        </div>
      )}
    </section>
  );
}
