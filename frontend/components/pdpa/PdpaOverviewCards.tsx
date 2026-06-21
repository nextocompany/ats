"use client";

// Compliance snapshot for the PDPA console: a stat strip (DSAR queue depth,
// breach status with the overdue count escalated, consent version, retention)
// plus the published DPO contact card.
import { useTranslations } from "next-intl";
import { AlertTriangle } from "lucide-react";

import type { PdpaOverview } from "@/lib/types";

function Stat({ label, value, tone }: { label: string; value: string; tone?: "warn" }) {
  return (
    <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
      <p className="text-xs uppercase tracking-wide text-muted-foreground">{label}</p>
      <p
        className={`mt-2 text-3xl font-semibold tabular-nums ${
          tone === "warn" ? "text-destructive" : "text-foreground"
        }`}
      >
        {value}
      </p>
    </div>
  );
}

export function PdpaOverviewCards({ overview }: { overview: PdpaOverview }) {
  const t = useTranslations("pdpa");
  const dpo = overview.dpo;
  const placeholder = (v: string | null | undefined) => (!v || v.trim() === "" ? t("notSet") : v);

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <Stat label={t("statDsarPending")} value={String(overview.dsar_pending)} />
        <Stat label={t("statBreachesOpen")} value={String(overview.breaches_open)} />
        <Stat
          label={t("statBreachesOverdue")}
          value={String(overview.breaches_overdue)}
          tone={overview.breaches_overdue > 0 ? "warn" : undefined}
        />
        <Stat label={t("statConsentVersion")} value={overview.current_consent_version || "-"} />
      </div>

      {overview.breaches_overdue > 0 && (
        <div className="flex items-start gap-3 rounded-xl bg-destructive/10 p-4 text-destructive ring-1 ring-destructive/20">
          <AlertTriangle aria-hidden="true" className="mt-0.5 size-5 shrink-0" />
          <p className="text-sm">{t("overdueWarning", { count: overview.breaches_overdue })}</p>
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">{t("retentionTitle")}</p>
          <p className="mt-2 text-sm text-foreground">
            {t("retentionBody", { days: overview.retention_days })}{" "}
            <span className="text-muted-foreground">
              {overview.retention_sweep_enabled ? t("retentionOn") : t("retentionOff")}
            </span>
          </p>
        </div>
        <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
          <p className="text-xs uppercase tracking-wide text-muted-foreground">{t("dpoTitle")}</p>
          <div className="mt-2 space-y-2 text-sm">
            <div className="flex gap-2">
              <span className="w-20 shrink-0 text-muted-foreground">{t("dpoCompany")}</span>
              <span className="text-foreground">{placeholder(dpo.company)}</span>
            </div>
            {dpo.officers.length > 0 ? (
              dpo.officers.map((o, i) => (
                <dl key={i} className="space-y-1 border-t border-hairline pt-2">
                  <div className="flex gap-2">
                    <dt className="w-20 shrink-0 text-muted-foreground">{t("dpoName")}</dt>
                    <dd className="text-foreground">{placeholder(o.name)}</dd>
                  </div>
                  <div className="flex gap-2">
                    <dt className="w-20 shrink-0 text-muted-foreground">{t("dpoEmail")}</dt>
                    <dd className="text-foreground">{placeholder(o.email)}</dd>
                  </div>
                  <div className="flex gap-2">
                    <dt className="w-20 shrink-0 text-muted-foreground">{t("dpoPhone")}</dt>
                    <dd className="text-foreground">{placeholder(o.phone)}</dd>
                  </div>
                </dl>
              ))
            ) : (
              <p className="text-muted-foreground">{t("dpoUnset")}</p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
