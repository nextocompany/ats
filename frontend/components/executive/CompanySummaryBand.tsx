"use client";

// Persistent company scoreboard above the executive tabs. Flat 4-up ledger strip
// (board-report refinement of the old HeadcountBand hero). Budget-derived cells
// (Budgeted, Fill rate) carry a dignified pending-HRIS state when the company
// budget is not yet wired (PeopleSoft); Actual headcount + Vacancy are always real.
import { useTranslations } from "next-intl";

import { Pill } from "@/components/people/PeopleBits";
import type { ExecutiveCompany } from "@/lib/types";

const fmt = new Intl.NumberFormat("en-US");

export function CompanySummaryBand({ company }: { company: ExecutiveCompany }) {
  const t = useTranslations("executive");
  const hasBudget = company.budget_available;

  return (
    <section
      aria-label={t("companyHeadcountAria")}
      className="grid gap-px overflow-hidden rounded-xl bg-hairline ring-1 ring-hairline grid-cols-2 sm:grid-cols-4"
    >
      {/* Lead cell - Actual headcount */}
      <div className="relative col-span-2 flex flex-col justify-between bg-brand px-5 py-6 text-brand-foreground sm:col-span-1">
        <span
          aria-hidden
          className="absolute inset-y-6 left-0 w-[3px] rounded-full"
          style={{ background: "var(--brass)" }}
        />
        <p className="pl-3.5 text-[0.6875rem] font-semibold uppercase tracking-[0.16em] text-brand-foreground/70">
          {t("actualHeadcount")}
        </p>
        <div className="mt-3 pl-3.5">
          <span className="num block font-semibold tabular-nums leading-none [font-size:var(--text-stat)] tracking-tight">
            {fmt.format(company.actual_headcount)}
          </span>
          <p className="mt-3 text-sm text-brand-foreground/80">
            {hasBudget
              ? t("headcountBudgetLine", {
                  budget: fmt.format(company.budget_headcount),
                  pct: company.fill_rate_pct,
                })
              : t("headcountNoBudgetLine")}
          </p>
        </div>
      </div>

      {/* Budgeted - pending-HRIS aware */}
      <SummaryCell
        label={t("budgeted")}
        value={hasBudget ? fmt.format(company.budget_headcount) : null}
        hint={hasBudget ? t("approvedHeadcount") : t("pendingHris")}
        pendingShort={t("pendingHrisShort")}
        pending={!hasBudget}
      />

      {/* Vacancy - always real */}
      <SummaryCell
        label={t("vacancy")}
        value={fmt.format(company.vacancy)}
        hint={hasBudget ? t("openPositions") : t("openVacancies")}
      />

      {/* Fill rate - pending-HRIS aware */}
      <SummaryCell
        label={t("fillRate")}
        value={hasBudget ? `${company.fill_rate_pct}%` : null}
        hint={hasBudget ? t("ofBudgetFilled") : t("pendingHris")}
        pendingShort={t("pendingHrisShort")}
        pending={!hasBudget}
      />
    </section>
  );
}

// A supporting scoreboard cell. When `pending`, the figure renders as a quiet "-"
// with a neutral pill instead of the bold foreground number. Never red/amber.
function SummaryCell({
  label,
  value,
  hint,
  pending = false,
  pendingShort,
}: {
  label: string;
  value: string | null;
  hint: string;
  pending?: boolean;
  pendingShort?: string;
}) {
  return (
    <div className="flex flex-col justify-between bg-card px-5 py-5">
      <div className="flex items-start justify-between gap-2">
        <p className="text-[0.6875rem] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
          {label}
        </p>
        {pending && pendingShort && <Pill tone="neutral">{pendingShort}</Pill>}
      </div>
      <div className="mt-2">
        {pending || value === null ? (
          <span className="num block text-3xl font-semibold leading-none tracking-tight text-muted-foreground">
            -
          </span>
        ) : (
          <span className="num block text-3xl font-semibold tabular-nums leading-none tracking-tight text-foreground">
            {value}
          </span>
        )}
        <p className="mt-2 text-xs text-muted-foreground">{hint}</p>
      </div>
    </div>
  );
}
