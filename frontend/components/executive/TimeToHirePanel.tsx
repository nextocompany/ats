"use client";

// Time-to-hire: median and average days from apply to offer-accept (created_at →
// hired_at) over the period's hires. Median leads (resistant to outliers); the
// hire count anchors the sample size so a small-n median reads honestly.
import { useTranslations } from "next-intl";
import { Clock } from "lucide-react";

import { formatNum } from "@/lib/format";
import type { ExecTimeToHire } from "@/lib/types";
import { EmptyState } from "@/components/executive/ExecutiveSections";

export function TimeToHirePanel({ tth }: { tth: ExecTimeToHire }) {
  const t = useTranslations("executive");
  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5">
        <p className="eyebrow brass-underline inline-block">{t("tthEyebrow")}</p>
        <h2 className="mt-3 font-heading text-lg font-semibold tracking-tight">{t("tthTitle")}</h2>
      </header>

      {tth.hires === 0 ? (
        <EmptyState icon={<Clock className="size-4" strokeWidth={1.75} />} title={t("tthEmptyTitle")} hint={t("tthEmptyHint")} />
      ) : (
        <div className="grid grid-cols-3 gap-4">
          <Metric label={t("tthMedian")} value={tth.median_days} unit={t("tthDays")} accent />
          <Metric label={t("tthAvg")} value={tth.avg_days} unit={t("tthDays")} />
          <Metric label={t("tthSample")} value={tth.hires} unit={t("tthHiresUnit")} integer />
        </div>
      )}
    </section>
  );
}

function Metric({
  label,
  value,
  unit,
  accent = false,
  integer = false,
}: {
  label: string;
  value: number;
  unit: string;
  accent?: boolean;
  integer?: boolean;
}) {
  return (
    <div className={"rounded-lg p-4 ring-1 ring-hairline " + (accent ? "bg-brand-soft" : "bg-muted/40")}>
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className={"mt-1 font-heading text-3xl font-semibold tabular-nums tracking-tight " + (accent ? "text-brand" : "text-foreground")}>
        {integer ? formatNum(value) : value}
      </p>
      <p className="mt-0.5 text-xs text-muted-foreground">{unit}</p>
    </div>
  );
}
