"use client";

// Scheduled human-interview rounds for an application. Surfaces persisted
// appointments (date/time, mode, Teams join link) per round — supports multi-round
// interviews. Hidden until at least one round exists; scheduling itself lives in
// the status actions (AiSummaryPanel → ScheduleInterviewDialog).
import { useTranslations, useLocale } from "next-intl";

import { useInterviewAppointments } from "@/lib/queries";
import { Badge } from "@/components/ui/badge";

export function InterviewRoundsPanel({ applicationId }: { applicationId: string }) {
  const t = useTranslations("resume");
  const locale = useLocale();
  const { data, isLoading } = useInterviewAppointments(applicationId);

  if (isLoading) return null;
  if (!data || data.length === 0) return null; // no rounds scheduled yet

  const formatWhen = (iso: string) =>
    new Date(iso).toLocaleString(locale === "th" ? "th-TH" : "en-GB", {
      dateStyle: "medium",
      timeStyle: "short",
    });

  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <p className="eyebrow">{t("roundsTitle", { count: data.length })}</p>
      <ul className="mt-3 flex flex-col gap-2">
        {data.map((a) => (
          <li key={a.id} className="rounded-lg bg-muted/40 px-3 py-2">
            <div className="flex items-center justify-between gap-3">
              <span className="text-sm font-medium text-foreground">{t("roundNo", { n: a.round_no })}</span>
              <Badge variant="secondary" className="capitalize">
                {t.has(`mode_${a.mode}`) ? t(`mode_${a.mode}`) : a.mode}
              </Badge>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              {formatWhen(a.scheduled_at)} · {t("minutes", { n: a.duration_min })}
            </p>
            {a.location_text && <p className="mt-0.5 text-xs text-muted-foreground">{a.location_text}</p>}
            {a.online_join_url && (
              <a
                href={a.online_join_url}
                target="_blank"
                rel="noopener noreferrer"
                className="mt-1 inline-block text-xs font-medium text-brand underline-offset-2 hover:underline"
              >
                {t("joinMeeting")}
              </a>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
