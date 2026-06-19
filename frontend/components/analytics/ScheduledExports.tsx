"use client";

import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useReportExports, useTriggerExport } from "@/lib/queries";

function formatDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString();
}

// ScheduledExports lists recent report exports with download links and an
// on-demand "Export now" trigger (Sprint 5b).
export function ScheduledExports() {
  const t = useTranslations("analytics");
  const { data: exports, isLoading } = useReportExports();
  const trigger = useTriggerExport();

  return (
    <section className="rounded-xl bg-card ring-1 ring-hairline">
      <header className="flex items-center justify-between gap-4 border-b border-hairline px-6 py-4">
        <div>
          <p className="eyebrow">{t("exportsEyebrow")}</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">{t("exportsTitle")}</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">{t("exportsDesc")}</p>
        </div>
        <Button onClick={() => trigger.mutate()} disabled={trigger.isPending}>
          {trigger.isPending ? t("exporting") : t("exportNow")}
        </Button>
      </header>

      <div className="p-6">
        {isLoading ? (
          <Skeleton className="h-20 w-full" />
        ) : !exports || exports.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">{t("exportsEmpty")}</p>
        ) : (
          <ul className="divide-y divide-border">
            {exports.map((e) => (
              <li key={e.id} className="flex items-center justify-between gap-4 py-2.5 text-sm">
                <div className="min-w-0">
                  <span className="font-medium">{e.period}</span>{" "}
                  <span className="text-muted-foreground">· {e.kind}</span>{" "}
                  <span className="text-muted-foreground">· {formatDate(e.created_at)}</span>
                  {!e.delivered ? (
                    <span className="ml-2 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">{t("notDelivered")}</span>
                  ) : null}
                </div>
                <div className="flex shrink-0 gap-3">
                  {e.csv_url ? (
                    <a href={e.csv_url} className="text-primary underline-offset-4 hover:underline" target="_blank" rel="noreferrer">
                      CSV
                    </a>
                  ) : null}
                  {e.json_url ? (
                    <a href={e.json_url} className="text-primary underline-offset-4 hover:underline" target="_blank" rel="noreferrer">
                      JSON
                    </a>
                  ) : null}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </section>
  );
}
