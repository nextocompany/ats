"use client";

// Headcount & vacancy by branch - board table for the executive "Headcount" tab.
// Actual headcount is always real; Budget / Vacancy (gap) / Fill are budget-derived
// and render a pending-HRIS state when the company budget is not yet connected.
import { useTranslations } from "next-intl";
import { Building2 } from "lucide-react";

import { EmptyState, fillShade } from "@/components/executive/ExecutiveSections";
import { Pill } from "@/components/people/PeopleBits";
import type { ExecutiveStoreFill } from "@/lib/types";

const fmt = new Intl.NumberFormat("en-US");

export function HeadcountVacancyPanel({
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
          <p className="eyebrow brass-underline inline-block">{t("tabHeadcount")}</p>
          <h2 className="mt-3 flex items-center gap-2 font-heading text-lg font-semibold tracking-tight">
            <span aria-hidden className="text-muted-foreground">
              <Building2 className="size-4" strokeWidth={1.75} />
            </span>
            {t("headcountVacancyTitle")}
          </h2>
        </div>
        {!budgetAvailable && <Pill tone="neutral">{t("pendingHrisShort")}</Pill>}
      </header>

      {stores.length === 0 ? (
        <EmptyState
          icon={<Building2 className="size-4" strokeWidth={1.75} />}
          title={t("noStaffingData")}
          hint={t("noStaffingHint")}
        />
      ) : (
        <table className="w-full text-sm" aria-label={t("headcountVacancyTitle")}>
          <thead className="ledger-head">
            <tr>
              <th className="px-3 py-2 text-left">{t("colBranch")}</th>
              <th className="px-3 py-2 text-left">{t("colSubregion")}</th>
              <th className="px-3 py-2 text-right">{t("colActual")}</th>
              <th className="px-3 py-2 text-right">{t("colBudget")}</th>
              <th className="px-3 py-2 text-right">{t("colVacancy")}</th>
              <th className="px-3 py-2 text-right">{t("colFill")}</th>
            </tr>
          </thead>
          <tbody>
            {stores.map((s) => (
              <tr key={s.store_no} className="ledger-row border-b border-hairline last:border-0">
                <td className="px-3 py-2.5 font-medium text-foreground">{s.store_name}</td>
                <td className="px-3 py-2.5 text-xs text-muted-foreground">{s.subregion || "-"}</td>
                <td className="px-3 py-2.5 text-right tabular-nums text-foreground">
                  {fmt.format(s.actual_headcount)}
                </td>
                <td className="px-3 py-2.5 text-right tabular-nums text-muted-foreground">
                  {budgetAvailable ? fmt.format(s.budget_headcount) : "-"}
                </td>
                <td className="px-3 py-2.5 text-right tabular-nums text-muted-foreground">
                  {budgetAvailable ? fmt.format(s.heads_short) : "-"}
                </td>
                <td className="px-3 py-2.5 text-right">
                  {budgetAvailable ? <FillBar pct={s.fill_rate_pct} /> : <span className="text-muted-foreground">-</span>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

// Inline fill-rate bar with the numeric value, mirroring ShortStaffedPanel.
function FillBar({ pct }: { pct: number }) {
  return (
    <span className="inline-flex items-center justify-end gap-2 tabular-nums">
      <span className="font-semibold text-foreground">{pct}%</span>
      <span className="h-2 w-16 overflow-hidden rounded-full bg-muted" aria-hidden>
        <span
          className="block h-full rounded-full"
          style={{ width: `${Math.max(Math.min(pct, 100), 4)}%`, backgroundColor: fillShade(pct) }}
        />
      </span>
    </span>
  );
}
