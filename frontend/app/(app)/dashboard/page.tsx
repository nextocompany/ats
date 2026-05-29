"use client";

import Link from "next/link";

import { FunnelChart, KpiCards } from "@/components/analytics/Charts";
import { buttonVariants } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useFunnel, useKpi } from "@/lib/queries";

export default function DashboardPage() {
  const { data: kpi } = useKpi();
  const { data: funnel } = useFunnel();

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold tracking-tight">Overview</h1>
        <Link href="/applications" className={buttonVariants({ size: "sm" })}>
          Open inbox →
        </Link>
      </div>
      {kpi ? <KpiCards kpi={kpi} /> : <Skeleton className="h-24 w-full" />}
      <div className="max-w-2xl">
        {funnel ? <FunnelChart funnel={funnel} /> : <Skeleton className="h-72 w-full" />}
      </div>
    </div>
  );
}
