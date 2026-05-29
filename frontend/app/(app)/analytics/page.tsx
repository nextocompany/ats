"use client";

import { FunnelChart, KpiCards, SourcesChart } from "@/components/analytics/Charts";
import { ScheduledExports } from "@/components/analytics/ScheduledExports";
import { Skeleton } from "@/components/ui/skeleton";
import { useFunnel, useKpi, useSources } from "@/lib/queries";

export default function AnalyticsPage() {
  const { data: kpi } = useKpi();
  const { data: funnel } = useFunnel();
  const { data: sources } = useSources();

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold tracking-tight">Analytics</h1>
      {kpi ? <KpiCards kpi={kpi} /> : <Skeleton className="h-24 w-full" />}
      <div className="grid gap-4 lg:grid-cols-2">
        {funnel ? <FunnelChart funnel={funnel} /> : <Skeleton className="h-72 w-full" />}
        {sources ? <SourcesChart sources={sources} /> : <Skeleton className="h-72 w-full" />}
      </div>
      <ScheduledExports />
    </div>
  );
}
