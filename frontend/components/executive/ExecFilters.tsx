"use client";

// Global filter bar for the Recruitment ROI dashboard: a rolling period window
// and the success-table dimension. Both are persisted in the URL so a board view
// is shareable. Rendered as two segmented controls (no library dropdown — the
// option set is tiny and a segmented control reads as a deliberate control).
import { useTranslations } from "next-intl";

import type { ExecDimension, ExecPeriod } from "@/lib/types";

const PERIODS: ExecPeriod[] = ["month", "quarter", "year"];
const DIMENSIONS: ExecDimension[] = ["branch", "region", "position"];

interface Props {
  period: ExecPeriod;
  dimension: ExecDimension;
  onPeriod: (p: ExecPeriod) => void;
  onDimension: (d: ExecDimension) => void;
}

export function ExecFilters({ period, dimension, onPeriod, onDimension }: Props) {
  const t = useTranslations("executive");
  return (
    <section className="flex flex-wrap items-center justify-between gap-4 rounded-xl bg-card p-4 ring-1 ring-hairline print:hidden">
      <Segmented
        ariaLabel={t("periodAria")}
        legend={t("periodLabel")}
        options={PERIODS.map((p) => ({ value: p, label: t(`period_${p}`) }))}
        active={period}
        onChange={(v) => onPeriod(v as ExecPeriod)}
      />
      <Segmented
        ariaLabel={t("dimensionAria")}
        legend={t("dimensionLabel")}
        options={DIMENSIONS.map((d) => ({ value: d, label: t(`dim_${d}`) }))}
        active={dimension}
        onChange={(v) => onDimension(v as ExecDimension)}
      />
    </section>
  );
}

function Segmented({
  ariaLabel,
  legend,
  options,
  active,
  onChange,
}: {
  ariaLabel: string;
  legend: string;
  options: { value: string; label: string }[];
  active: string;
  onChange: (v: string) => void;
}) {
  return (
    <div className="flex items-center gap-3">
      <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{legend}</span>
      <div role="group" aria-label={ariaLabel} className="inline-flex rounded-lg bg-muted p-0.5">
        {options.map((o) => {
          const isActive = o.value === active;
          return (
            <button
              key={o.value}
              type="button"
              aria-pressed={isActive}
              onClick={() => onChange(o.value)}
              className={
                "rounded-md px-3 py-1.5 text-sm font-medium transition-colors " +
                (isActive
                  ? "bg-card text-foreground shadow-sm ring-1 ring-hairline"
                  : "text-muted-foreground hover:text-foreground")
              }
            >
              {o.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
