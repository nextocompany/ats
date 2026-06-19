"use client";

import { useState } from "react";
import { toast } from "sonner";

import type { Application } from "@/lib/types";
import { allowedActions, type Action } from "@/lib/statusMachine";
import { useInviteInterview, useSetStatus } from "@/lib/queries";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ScoreBreakdown } from "@/components/resume/ScoreBreakdown";
import { ScheduleInterviewDialog } from "@/components/resume/ScheduleInterviewDialog";
import { RejectDialog } from "@/components/resume/RejectDialog";

export function AiSummaryPanel({ app }: { app: Application }) {
  const setStatus = useSetStatus(app.id);
  const inviteInterview = useInviteInterview(app.id);
  const [scheduleOpen, setScheduleOpen] = useState(false);
  const [rejectOpen, setRejectOpen] = useState(false);

  // Only the actions the state machine permits from the current status are shown.
  const actions = allowedActions(app.status);

  const move = (status: string, msg: string) =>
    setStatus.mutate(
      { status },
      {
        onSuccess: () => toast.success(msg),
        onError: (e) => toast.error(e instanceof Error ? e.message : "Update failed"),
      },
    );

  const sendInterview = () =>
    inviteInterview.mutate(undefined, {
      onSuccess: () => toast.success("AI interview invite sent"),
      onError: (e) => toast.error(e instanceof Error ? e.message : "Could not send interview"),
    });

  // One renderer per action keeps the button set declarative + ordered.
  function renderAction(a: Action) {
    const busy = setStatus.isPending || inviteInterview.isPending;
    switch (a) {
      case "send_ai_interview":
        return (
          <Button key={a} size="sm" variant="default" disabled={busy} onClick={sendInterview} className="col-span-2 w-full">
            {inviteInterview.isPending ? "Sending…" : <><span aria-hidden="true">▶</span> Send AI interview</>}
          </Button>
        );
      case "shortlist":
        return <Button key={a} size="sm" variant="secondary" disabled={busy} onClick={() => move("shortlisted", "Shortlisted")} className="w-full">Shortlist</Button>;
      case "interview":
        return <Button key={a} size="sm" variant="secondary" disabled={busy} onClick={() => setScheduleOpen(true)} className="w-full">Interview…</Button>;
      case "mark_interviewed":
        return <Button key={a} size="sm" variant="secondary" disabled={busy} onClick={() => move("interviewed", "Marked as interviewed")} className="w-full">Mark interview done</Button>;
      case "submit_approval":
        // The hiring approval submit lives in ApprovalPanel (with the chain view);
        // nothing is rendered here so the action grid only shows generic moves.
        return null;
      case "reject":
        return <Button key={a} size="sm" variant="destructive" disabled={busy} onClick={() => setRejectOpen(true)} className="w-full">Reject…</Button>;
      default:
        return null;
    }
  }

  // Filter nulls (e.g. submit_approval renders nothing here — ApprovalPanel owns
  // it) so a lone action isn't orphaned into the second grid column.
  const nextStepButtons = actions.map(renderAction).filter(Boolean);

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
        {app.assigned_store_id !== null && <Badge variant="outline">สาขา {app.assigned_store_id}</Badge>}
        {app.talent_pool && <Badge variant="outline">พูลกลาง · รอจัดสาขา</Badge>}
        {app.needs_manual_review && (
          <span className="inline-flex items-center rounded-full bg-brass-soft px-2 py-0.5 text-xs font-medium text-brass">
            manual review
          </span>
        )}
        {app.dedup_state && app.dedup_state !== "none" && <Badge variant="outline">dedup: {app.dedup_state}</Badge>}
      </div>

      {app.ai_score_breakdown && (
        <ScoreBreakdown
          breakdown={app.ai_score_breakdown}
          summary={app.ai_summary}
          redFlags={app.ai_red_flags}
        />
      )}

      {app.status === "rejected" && app.rejection_reason && (
        <div className="rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
          <p className="font-medium">Not selected</p>
          <p className="mt-0.5 text-destructive/90">{app.rejection_reason}</p>
        </div>
      )}

      <div className="h-px bg-hairline" />

      <div>
        <p className="mb-2.5 text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-muted-foreground">
          Next step
        </p>
        {nextStepButtons.length > 0 ? (
          <div className="grid grid-cols-2 gap-2">{nextStepButtons}</div>
        ) : (
          <p className="text-sm text-muted-foreground">
            {app.status === "ai_interview"
              ? "AI interview in progress — awaiting the candidate."
              : "No actions available at this stage."}
          </p>
        )}
      </div>

      <ScheduleInterviewDialog applicationId={app.id} open={scheduleOpen} onClose={() => setScheduleOpen(false)} />
      <RejectDialog applicationId={app.id} open={rejectOpen} onClose={() => setRejectOpen(false)} />

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
