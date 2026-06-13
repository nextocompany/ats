"use client";

import { useState } from "react";

import { Badge } from "@/components/ui/badge";
import { useInterview } from "@/lib/queries";

const REC_LABEL: Record<string, string> = {
  strong_recommend: "แนะนำอย่างยิ่ง",
  recommend: "แนะนำ",
  neutral: "เป็นกลาง",
  caution: "ควรพิจารณา",
};

function recTone(rec: string): string {
  if (rec === "strong_recommend" || rec === "recommend") return "var(--score-high)";
  if (rec === "neutral") return "var(--score-mid)";
  if (rec === "caution") return "var(--score-low)";
  return "var(--muted-foreground)";
}

export function InterviewPanel({ applicationId }: { applicationId: string }) {
  const { data, isLoading, isError } = useInterview(applicationId);
  const [showTranscript, setShowTranscript] = useState(false);
  const [copied, setCopied] = useState(false);

  if (isLoading) return null;
  if (isError) {
    return (
      <p className="mt-6 border-t border-hairline pt-6 text-xs text-muted-foreground">
        Could not load the AI interview.
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
          <p className="eyebrow">AI interview</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">Pre-screening chat</h2>
        </div>
        <Badge variant="secondary" className="capitalize">
          {s.status.replace("_", " ")}
        </Badge>
      </div>

      {!completed && (
        <div className="rounded-lg bg-brand-soft/60 p-4 text-sm ring-1 ring-brand/10">
          <p className="font-medium text-foreground">
            {s.status === "invited" ? "Invitation sent — awaiting the candidate" : "Interview in progress"}
          </p>
          <button
            type="button"
            onClick={() => {
              void navigator.clipboard?.writeText(data.interview_url);
              setCopied(true);
            }}
            className="mt-2 text-xs font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
          >
            {copied ? "Link copied ✓" : "Copy interview link"}
          </button>
        </div>
      )}

      {completed && (
        <>
          <div className="flex items-center gap-4 rounded-lg bg-brand-soft/60 p-4 ring-1 ring-brand/10">
            <div
              className="grid size-16 shrink-0 place-items-center rounded-lg text-2xl font-semibold tabular-nums text-white"
              style={{ backgroundColor: tone }}
              aria-label={score === null ? "Not scored" : `Interview score ${Math.round(score)}`}
            >
              {score === null ? "—" : Math.round(score)}
            </div>
            <div className="min-w-0">
              {s.recommendation && (
                <span
                  className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold text-white"
                  style={{ backgroundColor: recTone(s.recommendation) }}
                >
                  {REC_LABEL[s.recommendation] ?? s.recommendation}
                </span>
              )}
              {s.summary && <p className="mt-1.5 text-xs leading-relaxed text-muted-foreground">{s.summary}</p>}
            </div>
          </div>

          {s.strengths && s.strengths.length > 0 && (
            <div>
              <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
                จุดแข็ง
              </p>
              <ul className="space-y-1.5 text-sm text-foreground">
                {s.strengths.map((t, i) => (
                  <li key={i} className="flex gap-2">
                    <span className="text-[var(--score-high)]">•</span>
                    <span>{t}</span>
                  </li>
                ))}
              </ul>
            </div>
          )}

          {s.concerns && s.concerns.length > 0 && (
            <div>
              <p className="mb-2 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
                ข้อสังเกต
              </p>
              <ul className="space-y-1.5 text-sm text-foreground">
                {s.concerns.map((t, i) => (
                  <li key={i} className="flex gap-2">
                    <span className="text-[var(--score-low)]">•</span>
                    <span>{t}</span>
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
            {showTranscript ? "Hide transcript" : `View transcript (${s.conversation.length} turns)`}
          </button>
          {showTranscript && (
            <ul className="mt-3 space-y-3">
              {s.conversation.map((t, i) => (
                <li key={`${i}-${t.role}`} className={t.role === "user" ? "flex justify-end" : "flex justify-start"}>
                  <div
                    className={
                      t.role === "user"
                        ? "max-w-[85%] rounded-2xl rounded-br-sm bg-primary px-3 py-2 text-xs leading-relaxed text-primary-foreground"
                        : "max-w-[85%] rounded-2xl rounded-bl-sm bg-muted px-3 py-2 text-xs leading-relaxed text-foreground"
                    }
                  >
                    {t.content}
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
