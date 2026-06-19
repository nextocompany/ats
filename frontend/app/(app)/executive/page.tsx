"use client";

import { useTranslations } from "next-intl";

import { SourcesChart } from "@/components/analytics/Charts";
import {
  DemoBadge,
  HeadcountBand,
  PipelinePanel,
  ShortStaffedPanel,
} from "@/components/executive/ExecutiveSections";
import { PageHeader } from "@/components/shell/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { useExecutiveOverview, useMe } from "@/lib/queries";
import { canViewExecutive } from "@/lib/roles";

export default function ExecutivePage() {
  const t = useTranslations("executive");
  const { data: me } = useMe();
  // Gate the fetch too: a non-leadership role would only get a 403.
  const allowed = canViewExecutive(me);
  const { data, isLoading } = useExecutiveOverview(me ? allowed : false);

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
    <div className="settle space-y-8">
      <PageHeader
        eyebrow={t("eyebrow")}
        title={t("title")}
        meta={t("meta")}
        actions={
          <DemoBadge
            source={data?.data_source}
            budgetAvailable={data?.company.budget_available}
            demoLabel={t("demoData")}
            budgetPendingLabel={t("budgetPending")}
          />
        }
      />

      {data ? (
        <HeadcountBand company={data.company} />
      ) : (
        <Skeleton className="h-36 w-full rounded-xl" />
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        {data ? (
          <ShortStaffedPanel stores={data.stores} />
        ) : (
          <Skeleton className="h-72 w-full rounded-xl" />
        )}
        {data ? (
          <PipelinePanel rows={data.pipeline} />
        ) : (
          <Skeleton className="h-72 w-full rounded-xl" />
        )}
      </div>

      {data ? (
        <SourcesChart sources={data.sourcing} />
      ) : isLoading ? (
        <Skeleton className="h-64 w-full rounded-xl" />
      ) : null}
    </div>
  );
}
