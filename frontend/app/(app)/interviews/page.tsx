"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight, Clock, MapPin, Video } from "lucide-react";

import { InitialChip } from "@/components/people/PeopleBits";
import { PageHeader } from "@/components/shell/PageHeader";
import { Skeleton } from "@/components/ui/skeleton";
import { useMe, useUpcomingInterviews } from "@/lib/queries";
import { canViewInterviews } from "@/lib/roles";
import type { UpcomingInterview } from "@/lib/types";

// Group interviews into day buckets, preserving the server's ascending order.
function groupByDay(items: UpcomingInterview[]): { day: string; rows: UpcomingInterview[] }[] {
  const groups: { day: string; rows: UpcomingInterview[] }[] = [];
  for (const it of items) {
    const day = new Date(it.scheduled_at).toLocaleDateString(undefined, {
      weekday: "long",
      day: "numeric",
      month: "long",
      year: "numeric",
    });
    const last = groups[groups.length - 1];
    if (last && last.day === day) last.rows.push(it);
    else groups.push({ day, rows: [it] });
  }
  return groups;
}

export default function InterviewsPage() {
  const t = useTranslations("interviews");
  const { data: me } = useMe();
  const allowed = canViewInterviews(me);
  const [mine, setMine] = useState(false);
  const { data } = useUpcomingInterviews({ mine }, me ? allowed : false);

  const groups = useMemo(() => (data ? groupByDay(data) : []), [data]);

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

      {/* mine / all segmented toggle */}
      <div className="inline-flex rounded-lg bg-muted/50 p-0.5 ring-1 ring-hairline">
        {([false, true] as const).map((m) => (
          <button
            key={String(m)}
            type="button"
            onClick={() => setMine(m)}
            aria-pressed={mine === m}
            className={
              "rounded-md px-3 py-1.5 text-xs font-medium transition-colors " +
              (mine === m ? "bg-card text-foreground shadow-sm ring-1 ring-hairline" : "text-muted-foreground hover:text-foreground")
            }
          >
            {m ? t("filterMine") : t("filterAll")}
          </button>
        ))}
      </div>

      {!data ? (
        <Skeleton className="h-64 w-full rounded-xl" />
      ) : groups.length === 0 ? (
        <section className="rounded-xl bg-card p-10 text-center ring-1 ring-hairline">
          <p className="text-sm font-semibold text-foreground">{t("emptyTitle")}</p>
          <p className="mx-auto mt-1 max-w-sm text-xs text-muted-foreground">{t("emptyHint")}</p>
        </section>
      ) : (
        <div className="flex flex-col gap-7">
          {groups.map((g) => (
            <section key={g.day} className="space-y-3">
              <h2 className="sticky top-0 z-[1] bg-background/80 py-1 text-xs font-semibold uppercase tracking-wide text-muted-foreground backdrop-blur">
                {g.day}
              </h2>
              <ol className="flex flex-col gap-3">
                {g.rows.map((it) => (
                  <li key={it.id}>
                    <InterviewRow it={it} t={t} />
                  </li>
                ))}
              </ol>
            </section>
          ))}
        </div>
      )}
    </div>
  );
}

function InterviewRow({ it, t }: { it: UpcomingInterview; t: ReturnType<typeof useTranslations> }) {
  const time = new Date(it.scheduled_at).toLocaleTimeString(undefined, { hour: "2-digit", minute: "2-digit" });
  const online = it.mode === "online";
  return (
    <Link
      href={`/applications/${it.application_id}`}
      className="group flex items-center gap-4 rounded-xl bg-card p-4 ring-1 ring-hairline transition-colors hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <span className="flex shrink-0 flex-col items-center justify-center">
        <span className="inline-flex items-center gap-1 text-sm font-semibold tabular-nums text-foreground">
          <Clock className="size-3.5 text-muted-foreground" />
          {time}
        </span>
        {it.round_no > 1 && <span className="mt-0.5 text-[0.625rem] text-muted-foreground">{t("round", { n: it.round_no })}</span>}
      </span>
      <InitialChip name={it.candidate_name || "?"} />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium text-foreground">{it.candidate_name || "-"}</span>
        <span className="block truncate text-xs text-muted-foreground">
          {it.position_title}
          {it.store_name ? ` · ${it.store_name}` : ""}
        </span>
      </span>
      <span className="hidden shrink-0 items-center gap-3 sm:flex">
        <span
          className={
            "inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-[0.6875rem] font-medium " +
            (online ? "bg-brand-soft/60 text-brand" : "bg-muted text-muted-foreground")
          }
        >
          {online ? <Video className="size-3" /> : <MapPin className="size-3" />}
          {online ? t("online") : t("onsite")}
        </span>
        {online && it.online_join_url && (
          <button
            type="button"
            onClick={(e) => {
              e.preventDefault();
              window.open(it.online_join_url, "_blank", "noopener,noreferrer");
            }}
            className="text-xs font-medium text-primary underline-offset-2 hover:underline"
          >
            {t("join")}
          </button>
        )}
      </span>
      <ArrowRight className="size-4 shrink-0 text-muted-foreground/40 transition-transform group-hover:translate-x-0.5 group-hover:text-foreground" />
    </Link>
  );
}
