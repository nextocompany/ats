"use client";

// Executive — Recruitment ROI & Performance. A leadership board pack: the ROI band
// (cost vs hires, from admin-configured assumptions), the volume/response funnel,
// time-to-hire, and success broken down by branch / region / position. Period and
// dimension live in the URL so a view is shareable; RBAC-gated to leadership roles.
//
// The older budget/headcount tabs are intentionally retired here (their components
// remain on disk, just no longer imported) because the headcount-budget data they
// depend on does not exist without the pending HRIS integration.
import { Suspense, useState } from "react";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { Printer } from "lucide-react";

import { CostConfigDialog } from "@/components/executive/CostConfigDialog";
import { DataSourceBadge } from "@/components/executive/ExecutiveSections";
import { ExecFilters } from "@/components/executive/ExecFilters";
import { FunnelVolume } from "@/components/executive/FunnelVolume";
import { RoiBand } from "@/components/executive/RoiBand";
import { SuccessByDimension } from "@/components/executive/SuccessByDimension";
import { TimeToHirePanel } from "@/components/executive/TimeToHirePanel";
import { PageHeader } from "@/components/shell/PageHeader";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useExecutiveROI, useMe } from "@/lib/queries";
import { canEditExecCost, canViewExecutive } from "@/lib/roles";
import type { ExecDimension, ExecPeriod } from "@/lib/types";

const PERIODS: readonly string[] = ["month", "quarter", "year"];
const DIMENSIONS: readonly string[] = ["branch", "region", "position"];

export default function ExecutivePage() {
  return (
    <Suspense fallback={<Skeleton className="h-96 w-full rounded-xl" />}>
      <ExecutiveInner />
    </Suspense>
  );
}

function ExecutiveInner() {
  const t = useTranslations("executive");
  const locale = useLocale();
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const { data: me } = useMe();
  const allowed = canViewExecutive(me);
  const canEdit = canEditExecCost(me);
  const [costOpen, setCostOpen] = useState(false);

  const period = (PERIODS.includes(params.get("period") ?? "") ? params.get("period") : "quarter") as ExecPeriod;
  const dimension = (DIMENSIONS.includes(params.get("dim") ?? "") ? params.get("dim") : "branch") as ExecDimension;

  const { data, isLoading } = useExecutiveROI({ period, dimension }, me ? allowed : false);

  function setParam(key: string, value: string) {
    const sp = new URLSearchParams(params.toString());
    sp.set(key, value);
    router.replace(`${pathname}?${sp.toString()}`, { scroll: false });
  }

  // Wait for identity before deciding; only block once we know the role.
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

  return (
    <div className="settle space-y-6">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("roiPageTitle")}
        meta={t("roiPageMeta")}
        actions={
          <div className="flex items-center gap-3 print:hidden">
            <DataSourceBadge
              source={data?.data_source}
              budgetAvailable={true}
              demoLabel={t("demoData")}
              pendingLabel={t("pendingHrisShort")}
              liveLabel={t("dataSourceLive")}
            />
            <Button variant="outline" size="sm" className="gap-2" onClick={() => window.print()}>
              <Printer className="size-4" strokeWidth={1.75} />
              {t("printReport")}
            </Button>
          </div>
        }
      />

      <ExecFilters
        period={period}
        dimension={dimension}
        onPeriod={(p) => setParam("period", p)}
        onDimension={(d) => setParam("dim", d)}
      />

      {data ? (
        <>
          <RoiBand data={data} canEdit={canEdit} onConfigure={() => setCostOpen(true)} />
          <div className="grid gap-6 lg:grid-cols-2">
            <FunnelVolume funnel={data.funnel} />
            <TimeToHirePanel tth={data.time_to_hire} />
          </div>
          <SuccessByDimension rows={data.success} dimension={data.dimension} />
        </>
      ) : isLoading ? (
        <div className="space-y-6">
          <Skeleton className="h-40 w-full rounded-xl" />
          <Skeleton className="h-72 w-full rounded-xl" />
        </div>
      ) : (
        <section className="rounded-xl bg-card p-10 text-center ring-1 ring-hairline">
          <p className="text-sm font-semibold text-foreground">{t("loadError")}</p>
        </section>
      )}

      {/* Print-only footer: data provenance for the board pack. */}
      {data && (
        <footer className="hidden text-xs text-muted-foreground print:block">
          {t("asOf", { date: new Date(data.generated_at).toLocaleString(locale) })}
          {" · "}
          {data.data_source === "mock" ? t("demoData") : t("dataSourceLive")}
          {" · "}
          {t("roiDisclaimer")}
        </footer>
      )}

      {canEdit && <CostConfigDialog open={costOpen} onClose={() => setCostOpen(false)} />}
    </div>
  );
}
