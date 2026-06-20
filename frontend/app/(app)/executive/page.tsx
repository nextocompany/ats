"use client";

// Executive Overview - a board-report / print-style leadership pack. A persistent
// company summary band sits above four tabbed board views (shortage / headcount /
// pipeline / sourcing). Budget-derived metrics show a dignified pending-HRIS state
// until PeopleSoft is wired. RBAC-gated to leadership roles.
import { Suspense } from "react";
import { useRouter, useSearchParams, usePathname } from "next/navigation";
import { useTranslations, useLocale } from "next-intl";
import { Store, Building2, Briefcase, Share2, Printer } from "lucide-react";

import { SourcesChart } from "@/components/analytics/Charts";
import { CompanySummaryBand } from "@/components/executive/CompanySummaryBand";
import { HeadcountVacancyPanel } from "@/components/executive/HeadcountVacancyPanel";
import { ExecutiveTabs, type TabDef } from "@/components/executive/ExecutiveTabs";
import {
  DataSourceBadge,
  PipelinePanel,
  ShortStaffedPanel,
} from "@/components/executive/ExecutiveSections";
import { PageHeader } from "@/components/shell/PageHeader";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useExecutiveOverview, useMe } from "@/lib/queries";
import { canViewExecutive } from "@/lib/roles";

const TAB_KEYS = ["shortage", "headcount", "pipeline", "sourcing"] as const;

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
  const { data, isLoading } = useExecutiveOverview(me ? allowed : false);

  const tabParam = params.get("tab");
  const activeTab = (TAB_KEYS as readonly string[]).includes(tabParam ?? "") ? (tabParam as string) : "shortage";

  function setTab(key: string) {
    const sp = new URLSearchParams(params.toString());
    sp.set("tab", key);
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

  const budgetAvailable = data?.company.budget_available ?? false;
  const tabs: TabDef[] = [
    { key: "shortage", label: t("tabShortage"), icon: Store },
    { key: "headcount", label: t("tabHeadcount"), icon: Building2 },
    { key: "pipeline", label: t("tabPipeline"), icon: Briefcase },
    { key: "sourcing", label: t("tabSourcing"), icon: Share2 },
  ];

  return (
    <div className="settle space-y-8">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        meta={t("meta")}
        actions={
          <div className="flex items-center gap-3 print:hidden">
            <DataSourceBadge
              source={data?.data_source}
              budgetAvailable={data?.company.budget_available}
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

      {data ? (
        <CompanySummaryBand company={data.company} />
      ) : (
        <Skeleton className="h-28 w-full rounded-xl" />
      )}

      {data ? (
        <ExecutiveTabs
          tabs={tabs}
          active={activeTab}
          onChange={setTab}
          ariaLabel={t("tabsAria")}
          panels={{
            shortage: <ShortStaffedPanel stores={data.stores} budgetAvailable={budgetAvailable} />,
            headcount: <HeadcountVacancyPanel stores={data.stores} budgetAvailable={budgetAvailable} />,
            pipeline: <PipelinePanel rows={data.pipeline} />,
            sourcing: <SourcesChart sources={data.sourcing} />,
          }}
        />
      ) : isLoading ? (
        <Skeleton className="h-72 w-full rounded-xl" />
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
          {data.data_source === "mock" ? t("demoData") : budgetAvailable ? t("dataSourceLive") : t("pendingHrisShort")}
        </footer>
      )}
    </div>
  );
}
