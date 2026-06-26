"use client";

// ROI headline band: the cost-vs-hires business case. Every figure here is driven
// by admin-configured assumptions, never by real finance data, so the band carries
// a prominent disclaimer and — when assumptions are unset — an empty-state that
// routes a settings.admin to the cost editor instead of showing a fabricated zero.
import { useTranslations } from "next-intl";
import { Coins, TrendingUp, Wallet, Timer, SlidersHorizontal } from "lucide-react";

import { Button } from "@/components/ui/button";
import { formatMoney, formatNum } from "@/lib/format";
import type { ExecutiveROI } from "@/lib/types";

interface Props {
  data: ExecutiveROI;
  canEdit: boolean;
  onConfigure: () => void;
}

export function RoiBand({ data, canEdit, onConfigure }: Props) {
  const t = useTranslations("executive");
  const currency = data.cost.currency || "THB";

  return (
    <section aria-labelledby="roi-heading" className="rounded-xl bg-card p-6 ring-1 ring-hairline">
      <header className="mb-5 flex flex-wrap items-baseline justify-between gap-3">
        <div>
          <p className="eyebrow brass-underline inline-block">{t("roiEyebrow")}</p>
          <h2 id="roi-heading" className="mt-3 font-heading text-lg font-semibold tracking-tight">
            {t("roiTitle")}
          </h2>
        </div>
        <p className="max-w-xs text-right text-xs text-muted-foreground">{t("roiDisclaimer")}</p>
      </header>

      {data.cost_configured ? (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
          <Stat icon={<Coins className="size-4" />} label={t("roiHires")} value={formatNum(data.hires)} />
          <Stat
            icon={<Wallet className="size-4" />}
            label={t("roiCostPerHire")}
            value={formatMoney(data.cost_per_hire, currency)}
          />
          <Stat
            icon={<TrendingUp className="size-4" />}
            label={t("roiSavings")}
            value={formatMoney(data.savings, currency)}
            tone={data.savings >= 0 ? "good" : "bad"}
          />
          <Stat
            icon={<TrendingUp className="size-4" />}
            label={t("roiPct")}
            value={`${data.roi_pct}%`}
            tone={data.roi_pct >= 0 ? "good" : "bad"}
            accent
          />
          <Stat
            icon={<Timer className="size-4" />}
            label={t("roiVacancyAvoided")}
            value={formatMoney(data.vacancy_cost_avoided, currency)}
          />
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center gap-3 py-10 text-center">
          <span aria-hidden className="grid size-11 place-items-center rounded-2xl bg-brand-soft text-brand">
            <SlidersHorizontal className="size-5" />
          </span>
          <p className="text-sm font-semibold text-foreground">{t("roiUnsetTitle")}</p>
          <p className="mx-auto max-w-sm text-xs text-muted-foreground">{t("roiUnsetHint")}</p>
          {canEdit && (
            <Button size="sm" className="mt-1 gap-2" onClick={onConfigure}>
              <SlidersHorizontal className="size-4" strokeWidth={1.75} />
              {t("roiSetAssumptions")}
            </Button>
          )}
        </div>
      )}

      {data.cost_configured && canEdit && (
        <div className="mt-4 flex justify-end print:hidden">
          <Button variant="ghost" size="sm" className="gap-2 text-muted-foreground" onClick={onConfigure}>
            <SlidersHorizontal className="size-4" strokeWidth={1.75} />
            {t("roiEditAssumptions")}
          </Button>
        </div>
      )}
    </section>
  );
}

function Stat({
  icon,
  label,
  value,
  tone,
  accent = false,
}: {
  icon: React.ReactNode;
  label: string;
  value: string;
  tone?: "good" | "bad";
  accent?: boolean;
}) {
  const valueColor =
    tone === "bad" ? "text-[var(--score-low)]" : tone === "good" ? "text-foreground" : "text-foreground";
  return (
    <div className={"rounded-lg p-4 ring-1 ring-hairline " + (accent ? "bg-brand-soft" : "bg-muted/40")}>
      <div className="mb-2 flex items-center gap-2 text-muted-foreground">
        <span aria-hidden>{icon}</span>
        <span className="text-xs font-medium">{label}</span>
      </div>
      <p className={"font-heading text-2xl font-semibold tabular-nums tracking-tight " + (accent ? "text-brand" : valueColor)}>
        {value}
      </p>
    </div>
  );
}
