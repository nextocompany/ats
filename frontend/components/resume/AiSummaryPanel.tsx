"use client";

import { toast } from "sonner";

import type { Application } from "@/lib/types";
import { useSetStatus } from "@/lib/queries";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

const NEXT_ACTIONS: { label: string; value: string; variant?: "secondary" | "destructive" }[] = [
  { label: "Shortlist", value: "shortlisted", variant: "secondary" },
  { label: "Interview", value: "interview", variant: "secondary" },
  { label: "Hire", value: "hired" },
  { label: "Reject", value: "rejected", variant: "destructive" },
];

export function AiSummaryPanel({ app }: { app: Application }) {
  const setStatus = useSetStatus(app.id);

  const act = (value: string, label: string) =>
    setStatus.mutate(value, {
      onSuccess: () =>
        toast.success(value === "hired" ? "Hired — pushed to PeopleSoft" : `Status: ${label}`),
      onError: (e) => toast.error(e instanceof Error ? e.message : "Update failed"),
    });

  const score = app.ai_score;
  const tone =
    score === null
      ? "var(--muted-foreground)"
      : score >= 75
        ? "var(--score-high)"
        : score >= 50
          ? "var(--score-mid)"
          : "var(--score-low)";

  return (
    <div className="space-y-6">
      <div>
        <p className="eyebrow">AI summary</p>
        <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">Screening verdict</h2>
      </div>

      {/* Score hero */}
      <div className="flex items-center gap-4 rounded-lg bg-brand-soft/60 p-4 ring-1 ring-brand/10">
        <div
          className="grid size-16 shrink-0 place-items-center rounded-lg text-2xl font-semibold tabular-nums text-white"
          style={{ backgroundColor: tone }}
          aria-label={score === null ? "Not yet scored" : `AI score ${Math.round(score)}`}
        >
          {score === null ? "—" : Math.round(score)}
        </div>
        <div className="min-w-0">
          <p className="text-sm font-medium text-foreground">
            {score === null ? "Awaiting score" : score >= 75 ? "Strong fit" : score >= 50 ? "Worth a review" : "Below threshold"}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {app.must_have_passed === null
              ? "Not yet scored"
              : app.must_have_passed
                ? "Passed must-have gate"
                : "Failed must-have gate"}
          </p>
        </div>
      </div>

      <div className="flex flex-wrap gap-1.5">
        <Badge variant="secondary" className="capitalize">status: {app.status}</Badge>
        {app.assigned_store_id !== null && <Badge variant="outline">store {app.assigned_store_id}</Badge>}
        {app.talent_pool && <Badge variant="outline">talent pool</Badge>}
        {app.needs_manual_review && (
          <span className="inline-flex items-center rounded-full bg-brass-soft px-2 py-0.5 text-xs font-medium text-brass">
            manual review
          </span>
        )}
        {app.dedup_state && app.dedup_state !== "none" && <Badge variant="outline">dedup: {app.dedup_state}</Badge>}
      </div>

      <div className="h-px bg-hairline" />

      <div>
        <p className="mb-2.5 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
          Move to
        </p>
        <div className="grid grid-cols-2 gap-2">
          {NEXT_ACTIONS.map((a) => (
            <Button
              key={a.value}
              size="sm"
              variant={a.variant ?? "default"}
              disabled={setStatus.isPending}
              onClick={() => act(a.value, a.label)}
              className="w-full"
            >
              {a.label}
            </Button>
          ))}
        </div>
      </div>

      <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2.5 border-t border-hairline pt-5 text-xs">
        <dt className="text-muted-foreground">OCR confidence</dt>
        <dd className="text-right font-medium tabular-nums">
          {app.ocr_confidence !== null ? app.ocr_confidence.toFixed(2) : "—"}
        </dd>
        <dt className="text-muted-foreground">Parsed at</dt>
        <dd className="text-right tabular-nums">
          {app.parsed_at ? new Date(app.parsed_at).toLocaleString() : "—"}
        </dd>
        <dt className="text-muted-foreground">Profile JSON</dt>
        <dd className="truncate text-right">{app.parsed_profile_blob_url ? "stored" : "—"}</dd>
      </dl>
    </div>
  );
}
