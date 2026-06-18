"use client";

// ATS Reports panels (Module-3 3.9). Pure presentational sections over the
// RBAC-scoped AtsReport. Custom CSS panels following the dashboard convention
// (rounded-xl bg-card p-6 ring-1 ring-hairline + eyebrow + tabular-nums), brand
// CSS-var colors (no emerald, no chart lib).
import { useTranslations } from "next-intl";

import type { AtsReport } from "@/lib/types";

type T = ReturnType<typeof useTranslations>;

function Panel({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="rounded-xl bg-card p-6 ring-1 ring-hairline">
      <p className="eyebrow">{title}</p>
      <div className="mt-4">{children}</div>
    </section>
  );
}

// Stat is a labelled big number with an optional unit/suffix.
function Stat({ label, value, suffix }: { label: string; value: string | number; suffix?: string }) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="font-heading text-2xl font-semibold tabular-nums text-foreground">
        {value}
        {suffix ? <span className="ml-1 text-sm font-medium text-muted-foreground">{suffix}</span> : null}
      </span>
    </div>
  );
}

const FUNNEL_KEY: Record<string, string> = {
  applied: "stage_applied",
  screened: "stage_screened",
  interview: "stage_interview",
  offer: "stage_offer",
  hired: "stage_hired",
};

export function FunnelPanel({ funnel, t }: { funnel: AtsReport["funnel"]; t: T }) {
  const max = Math.max(1, ...funnel.stages.map((s) => s.count));
  return (
    <Panel title={t("funnelTitle")}>
      <ul className="flex flex-col gap-3">
        {funnel.stages.map((s, i) => (
          <li key={s.key} className="flex items-center gap-3">
            <span className="w-24 shrink-0 text-xs text-muted-foreground">{t(FUNNEL_KEY[s.key] ?? s.key)}</span>
            <div className="relative h-7 flex-1 overflow-hidden rounded-md bg-muted/50">
              <div
                className="h-full rounded-md bg-brand/80"
                style={{ width: `${Math.max(2, (s.count / max) * 100)}%` }}
              />
              <span className="absolute inset-y-0 left-2 flex items-center text-xs font-semibold tabular-nums text-foreground">
                {s.count}
              </span>
            </div>
            <span className="w-14 shrink-0 text-right text-xs tabular-nums text-muted-foreground">
              {i === 0 ? "—" : `${s.conversion_pct}%`}
            </span>
          </li>
        ))}
      </ul>
    </Panel>
  );
}

export function TimingPanel({ timing, t }: { timing: AtsReport["timing"]; t: T }) {
  const d = t("days");
  return (
    <Panel title={t("timingTitle")}>
      <div className="grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-3">
        <Stat label={t("avgToHire")} value={timing.avg_days_to_hire} suffix={d} />
        <Stat label={t("medianToHire")} value={timing.median_days_to_hire} suffix={d} />
        <Stat label={t("hiredCount")} value={timing.hired_count} />
        <Stat label={t("toOffer")} value={timing.avg_days_to_offer} suffix={d} />
        <Stat label={t("offerResponse")} value={timing.avg_offer_response_days} suffix={d} />
      </div>
    </Panel>
  );
}

export function OutcomesPanel({
  offers,
  onboarding,
  t,
}: {
  offers: AtsReport["offers"];
  onboarding: AtsReport["onboarding"];
  t: T;
}) {
  return (
    <Panel title={t("outcomesTitle")}>
      <div className="grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-3">
        <Stat label={t("offersSent")} value={offers.sent} />
        <Stat label={t("acceptRate")} value={offers.accept_rate_pct} suffix="%" />
        <Stat label={t("declineRate")} value={offers.decline_rate_pct} suffix="%" />
        <Stat
          label={t("onboardingComplete")}
          value={`${onboarding.completed}/${onboarding.hired_in_range}`}
          suffix={`(${onboarding.completion_rate_pct}%)`}
        />
        <Stat label={t("docRejection")} value={onboarding.doc_rejection_rate_pct} suffix="%" />
      </div>
    </Panel>
  );
}

export function QualityPanel({ quality, t }: { quality: AtsReport["quality"]; t: T }) {
  return (
    <Panel title={t("qualityTitle")}>
      <div className="grid grid-cols-2 gap-x-6 gap-y-5 sm:grid-cols-3">
        <Stat label={t("interviewPass")} value={quality.interview_pass_rate_pct} suffix="%" />
        <Stat label={t("avgRating")} value={quality.avg_interview_rating} suffix="/ 5" />
        <Stat label={t("approvalCycle")} value={quality.avg_approval_cycle_days} suffix={t("days")} />
        <Stat label={t("slaBreach")} value={quality.approval_sla_breach_pct} suffix="%" />
      </div>
    </Panel>
  );
}
