"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";

import { Badge } from "@/components/ui/badge";
import { useInterview } from "@/lib/queries";

function recTone(rec: string): string {
  if (rec === "strong_recommend" || rec === "recommend") return "var(--score-high)";
  if (rec === "neutral") return "var(--score-mid)";
  if (rec === "caution") return "var(--score-low)";
  return "var(--muted-foreground)";
}

export function InterviewPanel({ applicationId }: { applicationId: string }) {
  const t = useTranslations("resume");
  const { data, isLoading, isError } = useInterview(applicationId);
  const [showTranscript, setShowTranscript] = useState(false);
  const [copied, setCopied] = useState(false);

  if (isLoading) return null;
  if (isError) {
    return (
      <p className="mt-6 border-t border-hairline pt-6 text-xs text-muted-foreground">
        {t("intvFailed")}
      </p>
    );
  }
  if (!data) return null; // no interview yet (404 → null) → render nothing

  const s = data.session;
  const completed = s.status === "completed";
  const score = s.interview_score;
  const tone =
    score === null
      ? "var(--muted-foreground)"
      : score >= 75
        ? "var(--score-high)"
        : score >= 50
          ? "var(--score-mid)"
          : "var(--score-low)";

  return (
    <div className="mt-6 space-y-5 border-t border-hairline pt-6">
      <div className="flex items-center justify-between">
        <div>
          <p className="eyebrow">{t("intvEyebrow")}</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">{t("intvTitle")}</h2>
        </div>
        <Badge variant="secondary" className="capitalize">
          {t.has(`istatus_${s.status}`) ? t(`istatus_${s.status}`) : s.status.replace("_", " ")}
        </Badge>
      </div>

      {!completed && (
        <div className="rounded-lg bg-brand-soft/60 p-4 text-sm ring-1 ring-brand/10">
          <p className="font-medium text-foreground">
            {s.status === "invited" ? t("intvAwaiting") : t("intvInProgress")}
          </p>
          <button
            type="button"
            onClick={() => {
              void navigator.clipboard?.writeText(data.interview_url);
              setCopied(true);
            }}
            className="mt-2 text-xs font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
          >
            {copied ? t("intvLinkCopied") : t("intvCopyLink")}
          </button>
        </div>
      )}

      {completed && (
        <>
          <div className="flex items-center gap-4 rounded-lg bg-brand-soft/60 p-4 ring-1 ring-brand/10">
            <div
              className="grid size-16 shrink-0 place-items-center rounded-lg text-2xl font-semibold tabular-nums text-white"
              style={{ backgroundColor: tone }}
              aria-label={score === null ? t("intvNotScored") : t("intvScoreAria", { score: Math.round(score) })}
            >
              {score === null ? "—" : Math.round(score)}
            </div>
            <div className="min-w-0">
              {s.recommendation && (
                <span
                  className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold text-white"
                  style={{ backgroundColor: recTone(s.recommendation) }}
                >
                  {t.has(`irec_${s.recommendation}`) ? t(`irec_${s.recommendation}`) : s.recommendation}
                </span>
              )}
              {s.summary && <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">{s.summary}</p>}
            </div>
          </div>

          {s.strengths && s.strengths.length > 0 && (
            <div>
              <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
                {t("strengths")}
              </p>
              <ul className="space-y-1.5 text-sm text-foreground">
                {s.strengths.map((item, i) => (
                  <li key={i} className="flex gap-2">
                    <span className="text-[var(--score-high)]">•</span>
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {s.concerns && s.concerns.length > 0 && (
            <div>
              <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
                {t("concerns")}
              </p>
              <ul className="space-y-1.5 text-sm text-foreground">
                {s.concerns.map((item, i) => (
                  <li key={i} className="flex gap-2">
                    <span className="text-[var(--score-low)]">•</span>
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
        </>
      )}

      {s.conversation.length > 0 && (
        <div>
          <button
            type="button"
            onClick={() => setShowTranscript((v) => !v)}
            className="text-xs font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
          >
            {showTranscript ? t("intvHideTranscript") : t("intvViewTranscript", { count: s.conversation.length })}
          </button>
          {showTranscript && (
            <ul className="mt-3 space-y-3">
              {s.conversation.map((turn, i) => (
                <li key={`${i}-${turn.role}`} className={turn.role === "user" ? "flex justify-end" : "flex justify-start"}>
                  <div
                    className={
                      turn.role === "user"
                        ? "max-w-[85%] rounded-2xl rounded-br-sm bg-primary px-3 py-2 text-xs leading-relaxed text-primary-foreground"
                        : "max-w-[85%] rounded-2xl rounded-bl-sm bg-muted px-3 py-2 text-xs leading-relaxed text-foreground"
                    }
                  >
                    {turn.content}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  );
}
