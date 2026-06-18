"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight, Clock } from "lucide-react";

import { ScoreBadge } from "@/components/inbox/ScoreBadge";
import { InitialChip } from "@/components/people/PeopleBits";
import { PageHeader } from "@/components/shell/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { useApprovalQueue, useMe } from "@/lib/queries";
import { canAccessApprovals } from "@/lib/roles";

// Typed level → i18n-key map (no unsound cast; falls back to the raw number).
const LEVEL_KEYS: Record<number, "level1" | "level2" | "level3" | "level4"> = {
  1: "level1",
  2: "level2",
  3: "level3",
  4: "level4",
};

export default function ApprovalsPage() {
  const t = useTranslations("approvals");
  const { data: me } = useMe();
  const allowed = canAccessApprovals(me?.role);
  // Gate the fetch too: a non-approver role would only get an empty list.
  const { data } = useApprovalQueue(me ? allowed : false);

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
          {data.map((it) => (
            <li key={it.request_id}>
              <Link
                href={`/applications/${it.application_id}`}
                className="group flex items-center gap-4 rounded-xl bg-card p-4 ring-1 ring-hairline transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                <InitialChip name={it.candidate_name || "?"} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-sm font-medium text-foreground">
                    {it.candidate_name || t("unknownCandidate")}
                  </span>
                  <span className="block truncate text-xs text-muted-foreground">{it.position_title}</span>
                </span>
                <span className="hidden shrink-0 items-center gap-3 sm:flex">
                  <span className="rounded-full bg-brand-soft/60 px-2.5 py-1 text-[0.6875rem] font-medium text-brand">
                    {LEVEL_KEYS[it.active_level] ? t(LEVEL_KEYS[it.active_level]) : String(it.active_level)}
                  </span>
                  {it.due_at && (
                    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
                      <Clock className="size-3.5" />
                      {new Date(it.due_at).toLocaleDateString()}
                    </span>
                  )}
                  <ScoreBadge score={it.ai_score} />
                </span>
                <ArrowRight className="size-4 shrink-0 text-muted-foreground/40 transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
              </Link>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
