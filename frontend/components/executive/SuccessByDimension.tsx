"use client";

// Success by branch / region / position: a board table that decomposes the
// headline hires into the chosen dimension. Σ(hires) reconciles to the ROI
// headline. The top performer (most hires) is highlighted with a brass marker.
import { useTranslations } from "next-intl";
import { Trophy } from "lucide-react";

import { formatNum } from "@/lib/format";
import type { ExecDimension, ExecSuccessRow } from "@/lib/types";
import { EmptyState } from "@/components/executive/ExecutiveSections";

interface Props {
  rows: ExecSuccessRow[];
  dimension: ExecDimension;
}

export function SuccessByDimension({ rows, dimension }: Props) {
  const t = useTranslations("executive");
  const topHires = Math.max(...rows.map((r) => r.hires), 0);

  return (
    <section className="flex flex-col rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex items-baseline justify-between">
        <div>
          <p className="eyebrow brass-underline inline-block">{t("successEyebrow")}</p>
          <h2 className="mt-3 font-heading text-lg font-semibold tracking-tight">
            {t("successTitle", { dim: t(`dim_${dimension}`) })}
          </h2>
        </div>
        <span className="text-xs tabular-nums text-muted-foreground">{t("successCount", { count: rows.length })}</span>
      </header>

      {rows.length === 0 ? (
        <EmptyState icon={<Trophy className="size-4" strokeWidth={1.75} />} title={t("successEmptyTitle")} hint={t("successEmptyHint")} />
      ) : (
        <table className="w-full text-sm" aria-label={t("successTitle", { dim: t(`dim_${dimension}`) })}>
          <thead className="ledger-head">
            <tr>
              <th className="px-3 py-2 text-left">{t(`dim_${dimension}`)}</th>
              <th className="px-3 py-2 text-right">{t("successApplications")}</th>
              <th className="px-3 py-2 text-right">{t("successHires")}</th>
              <th className="px-3 py-2 text-right">{t("successConversion")}</th>
              <th className="px-3 py-2 text-right">{t("successAvgTth")}</th>
              <th className="px-3 py-2 text-left">{t("successTopSource")}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => {
              const isTop = topHires > 0 && r.hires === topHires;
              return (
                <tr key={r.key} className="ledger-row border-b border-hairline last:border-0">
                  <td className="px-3 py-2.5 font-medium text-foreground">
                    <span className="flex items-center gap-2">
                      {isTop && (
                        <span aria-hidden className="text-[var(--brass,oklch(70%_0.12_85))]">
                          <Trophy className="size-3.5" />
                        </span>
                      )}
                      {r.label}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-muted-foreground">{formatNum(r.applications)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums font-semibold text-foreground">{formatNum(r.hires)}</td>
                  <td className="px-3 py-2.5 text-right tabular-nums">{r.conversion}%</td>
                  <td className="px-3 py-2.5 text-right tabular-nums text-muted-foreground">{r.avg_time_to_hire}</td>
                  <td className="px-3 py-2.5 text-xs text-muted-foreground">{r.top_source || "-"}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      {dimension === "region" && rows.length > 0 && (
        <p className="mt-4 text-xs text-muted-foreground">{t("successRegionNote")}</p>
      )}
    </section>
  );
}
