"use client";

// Volume & response funnel: how many resumes came in over the period, the share HR
// picked up (response rate), and the conversion to hire — plus a hand-rolled CSS
// funnel bar (applied → screened → interviewed → offered → hired). No chart lib;
// bars are <div> widths against the applied total (mirrors analytics/Charts).
import { useTranslations } from "next-intl";
import { Inbox } from "lucide-react";

import { formatNum } from "@/lib/format";
import type { ExecFunnelStat } from "@/lib/types";
import { EmptyState } from "@/components/executive/ExecutiveSections";

const STAGES = ["applied", "screened", "interviewed", "offered", "hired"] as const;

export function FunnelVolume({ funnel }: { funnel: ExecFunnelStat }) {
  const t = useTranslations("executive");
  const max = Math.max(funnel.applied, 1);
  const values: Record<(typeof STAGES)[number], number> = {
    applied: funnel.applied,
    screened: funnel.screened,
    interviewed: funnel.interviewed,
    offered: funnel.offered,
    hired: funnel.hired,
  };

  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5">
        <p className="eyebrow brass-underline inline-block">{t("funnelEyebrow")}</p>
        <h2 className="mt-3 font-heading text-lg font-semibold tracking-tight">{t("funnelTitle")}</h2>
      </header>

      {funnel.applied === 0 ? (
        <EmptyState icon={<Inbox className="size-4" strokeWidth={1.75} />} title={t("funnelEmptyTitle")} hint={t("funnelEmptyHint")} />
      ) : (
        <>
          <div className="mb-6 grid grid-cols-3 gap-4">
            <Headline label={t("funnelResumesIn")} value={formatNum(funnel.applied)} />
            <Headline label={t("funnelResponseRate")} value={`${funnel.response_rate}%`} />
            <Headline label={t("funnelConversion")} value={`${funnel.conversion_to_hire}%`} accent />
          </div>

          <ol className="flex flex-col gap-3">
            {STAGES.map((stage, i) => {
              const v = values[stage];
              const pct = Math.max((v / max) * 100, v > 0 ? 3 : 0);
              return (
                <li key={stage} className="flex items-center gap-3">
                  <span className="w-24 shrink-0 text-xs font-medium text-muted-foreground">{t(`funnelStage_${stage}`)}</span>
                  <span className="relative h-7 flex-1 overflow-hidden rounded-md bg-muted">
                    <span
                      className="absolute inset-y-0 left-0 rounded-md"
                      style={{ width: `${pct}%`, backgroundColor: stageShade(i) }}
                      aria-hidden
                    />
                  </span>
                  <span className="w-16 shrink-0 text-right text-sm font-semibold tabular-nums text-foreground">
                    {formatNum(v)}
                  </span>
                </li>
              );
            })}
          </ol>
        </>
      )}
    </section>
  );
}

function Headline({ label, value, accent = false }: { label: string; value: string; accent?: boolean }) {
  return (
    <div className={"rounded-lg p-4 ring-1 ring-hairline " + (accent ? "bg-brand-soft" : "bg-muted/40")}>
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className={"mt-1 font-heading text-2xl font-semibold tabular-nums tracking-tight " + (accent ? "text-brand" : "text-foreground")}>
        {value}
      </p>
    </div>
  );
}

// Blue → brass tonal ramp down the funnel, so the narrowing reads as a deliberate
// scale rather than five identical bars.
function stageShade(index: number): string {
  const l = 46 + index * 6;
  const h = 264 - index * 40;
  return `oklch(${l}% 0.16 ${h})`;
}
