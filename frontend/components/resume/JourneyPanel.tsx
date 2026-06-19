"use client";

// Candidate journey timeline on the application detail. Reuses the existing
// candidate-scoped timeline endpoint (via useTimeline on the application's
// candidate). Surfaces status changes, resume views, re-engagement, and consent as
// a compact reverse-chronological list. Hidden until at least one event exists.
import { useTranslations, useLocale } from "next-intl";

import { useTimeline } from "@/lib/queries";
import type { TimelineEntry } from "@/lib/types";

type T = ReturnType<typeof useTranslations>;

function statusText(s: string | undefined, t: T): string {
  if (!s) return "";
  return t.has(`jstatus_${s}`) ? t(`jstatus_${s}`) : s;
}

function detail(e: TimelineEntry, t: T): string | null {
  if ((e.action === "status_change" || e.action === "bulk_action") && e.new_value) {
    const to = statusText(e.new_value.to ?? e.new_value.status, t);
    const from = statusText(e.new_value.from, t);
    if (from && to) return `${from} → ${to}`;
    if (to) return to;
  }
  return null;
}

export function JourneyPanel({ candidateId }: { candidateId: string }) {
  const t = useTranslations("resume");
  const locale = useLocale();
  const { data, isLoading } = useTimeline(candidateId);

  if (isLoading) return null;
  if (!data || data.length === 0) return null;

  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <p className="eyebrow">{t("journeyTitle")}</p>
      <ol className="mt-3 flex flex-col gap-3">
        {data.map((e, i) => {
          const d = detail(e, t);
          return (
            <li key={`${e.created_at}-${i}`} className="flex gap-3">
              <span aria-hidden="true" className="mt-1.5 size-2 shrink-0 rounded-full bg-brand/60" />
              <div className="min-w-0">
                <p className="text-sm text-foreground">
                  {t.has(`jact_${e.action}`) ? t(`jact_${e.action}`) : e.action}
                  {d && <span className="text-muted-foreground"> · {d}</span>}
                </p>
                <p className="text-xs text-muted-foreground">
                  {new Date(e.created_at).toLocaleString(locale === "th" ? "th-TH" : "en-GB")}
                </p>
              </div>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
