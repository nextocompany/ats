"use client";

import { useTranslations } from "next-intl";

import { FunnelChart, KpiCards, SourcesChart } from "@/components/analytics/Charts";
import { ScheduledExports } from "@/components/analytics/ScheduledExports";
import { PageHeader } from "@/components/shell/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { useFunnel, useKpi, useSources } from "@/lib/queries";

export default function AnalyticsPage() {
  const t = useTranslations("analytics");
  const { data: kpi } = useKpi();
  const { data: funnel } = useFunnel();
  const { data: sources } = useSources();

  return (
    <div className="settle space-y-8">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("meta")} />

      {kpi ? <KpiCards kpi={kpi} variant="reporting" /> : <Skeleton className="h-24 w-full rounded-xl" />}

      <div className="grid gap-6 lg:grid-cols-2">
        {funnel ? <FunnelChart funnel={funnel} /> : <Skeleton className="h-80 w-full rounded-xl" />}
        {sources ? <SourcesChart sources={sources} /> : <Skeleton className="h-80 w-full rounded-xl" />}
      </div>

      <ScheduledExports />
    </div>
  );
}
