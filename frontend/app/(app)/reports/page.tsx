"use client";

// ATS Reports (Module-3 3.9). HR-facing recruitment-funnel metrics over a date
// range, RBAC-scoped server-side (store roles see their store). Role-gated; CSV
// export via the synchronous downloadFile pattern (no toaster-free portal here —
// dashboard has a Toaster).
import { useState } from "react";
import { Download } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import {
  FunnelPanel,
  OutcomesPanel,
  QualityPanel,
  TimingPanel,
} from "@/components/reports/ReportSections";
import { PageHeader } from "@/components/shell/PageHeader";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { buildQuery, downloadFile } from "@/lib/api";
import { useAtsReport, useMe } from "@/lib/queries";
import { canViewReports } from "@/lib/roles";

function ymd(d: Date): string {
  return d.toISOString().slice(0, 10);
}

const DEFAULT_WINDOW_DAYS = 90;

export default function ReportsPage() {
  const t = useTranslations("reports");
  const { data: me } = useMe();
  const allowed = canViewReports(me?.role);

  // Lazy initializers (run once at setup) keep the impure Date out of render.
  const [today] = useState(() => ymd(new Date()));
  const [from, setFrom] = useState(() => ymd(new Date(Date.now() - DEFAULT_WINDOW_DAYS * 24 * 60 * 60 * 1000)));
  const [to, setTo] = useState(() => ymd(new Date()));

  const { data, isLoading, isError } = useAtsReport(allowed ? from : "", allowed ? to : "");
  const [exporting, setExporting] = useState(false);

  if (me && !allowed) {
    return (
      <div className="settle">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <section className="mt-8 rounded-xl bg-card p-10 text-center ring-1 ring-hairline">
          <p className="text-sm font-semibold text-foreground">{t("notAvailable")}</p>
          <p className="mx-auto mt-1 max-w-sm text-xs text-muted-foreground">{t("notAvailableHint")}</p>
        </section>
      </div>
    );
  }

  async function exportCsv() {
    setExporting(true);
    try {
      await downloadFile(`/api/v1/reports/ats.csv${buildQuery({ from, to })}`, "ats-report.csv");
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("exportFailed"));
    } finally {
      setExporting(false);
    }
  }

  const dateInput =
    "rounded-lg border border-input bg-transparent px-3 py-2 text-sm tabular-nums outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";

  return (
    <div className="settle space-y-8">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        meta={data ? t("scopeMeta", { scope: data.scope }) : t("meta")}
        actions={
          <div className="flex flex-wrap items-end gap-2">
            <label className="flex flex-col gap-1 text-xs text-muted-foreground">
              {t("from")}
              <input type="date" value={from} max={to} onChange={(e) => setFrom(e.target.value)} className={dateInput} />
            </label>
            <label className="flex flex-col gap-1 text-xs text-muted-foreground">
              {t("to")}
              <input type="date" value={to} min={from} max={today} onChange={(e) => setTo(e.target.value)} className={dateInput} />
            </label>
            <Button variant="outline" size="sm" className="gap-2" onClick={exportCsv} disabled={exporting || !data}>
              <Download className="size-4" />
              {exporting ? t("exporting") : t("exportCsv")}
            </Button>
          </div>
        }
      />

      {isError ? (
        <section role="alert" className="rounded-xl bg-card p-10 text-center text-sm text-destructive ring-1 ring-hairline">
          {t("loadFailed")}
        </section>
      ) : isLoading || !data ? (
        <Skeleton className="h-[60vh] w-full rounded-xl" />
      ) : (
        <div className="grid gap-6 lg:grid-cols-2">
          <FunnelPanel funnel={data.funnel} t={t} />
          <TimingPanel timing={data.timing} t={t} />
          <OutcomesPanel offers={data.offers} onboarding={data.onboarding} t={t} />
          <QualityPanel quality={data.quality} t={t} />
        </div>
      )}
    </div>
  );
}
