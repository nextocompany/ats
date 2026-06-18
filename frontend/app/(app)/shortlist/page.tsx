"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight } from "lucide-react";

import { ScoreBadge, FitLabel } from "@/components/inbox/ScoreBadge";
import { InitialChip } from "@/components/people/PeopleBits";
import { PageHeader } from "@/components/shell/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { useMe, useShortlist } from "@/lib/queries";
import { isLineManager } from "@/lib/roles";

export default function ShortlistPage() {
  const t = useTranslations("shortlist");
  const { data: me } = useMe();
  const allowed = isLineManager(me?.role) || me?.role === "super_admin";
  const { data, isLoading } = useShortlist(5);

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
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("meta")} />

      {!data ? (
        <Skeleton className="h-64 w-full rounded-xl" />
      ) : data.length === 0 ? (
        <section className="rounded-xl bg-card p-10 text-center ring-1 ring-hairline">
          <p className="text-sm font-semibold text-foreground">{t("emptyTitle")}</p>
          <p className="mx-auto mt-1 max-w-sm text-xs text-muted-foreground">{t("emptyHint")}</p>
        </section>
      ) : (
        <ol className="flex flex-col gap-3">
          {data.map((it, i) => (
            <li key={it.application_id}>
              <Link
                href={`/applications/${it.application_id}`}
                className="group flex items-center gap-4 rounded-xl bg-card p-4 ring-1 ring-hairline transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <span className="num w-6 shrink-0 text-center text-sm font-semibold tabular-nums text-muted-foreground">
                  {i + 1}
                </span>
                <InitialChip name={it.candidate_name || "?"} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-foreground">
                    {it.candidate_name || "ผู้สมัคร"}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">{it.position_title}</span>
                </span>
                <span className="hidden shrink-0 items-center gap-3 sm:flex">
                  <span className="text-right">
                    <span className="block text-[0.6875rem] uppercase tracking-[0.12em] text-muted-foreground">
                      {t("composite")}
                    </span>
                    <span className="num text-base font-semibold tabular-nums text-brand">{it.composite}</span>
                  </span>
                  <span className="flex items-center gap-1.5">
                    <ScoreBadge score={it.ai_score} />
                    <FitLabel score={it.ai_score} />
                  </span>
                  {it.ta_avg_overall != null && (
                    <span className="text-xs text-muted-foreground">
                      TA <span className="font-medium tabular-nums text-foreground">{it.ta_avg_overall.toFixed(1)}/5</span>
                    </span>
                  )}
                </span>
                <ArrowRight className="size-4 shrink-0 text-muted-foreground/40 transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
              </Link>
            </li>
          ))}
        </ol>
      )}

      {isLoading && !data && <Skeleton className="h-64 w-full rounded-xl" />}
    </div>
  );
}
