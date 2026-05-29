"use client";

import { toast } from "sonner";

import type { Application } from "@/lib/types";
import { useSetStatus } from "@/lib/queries";
import { ScoreBadge } from "@/components/inbox/ScoreBadge";
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

  return (
    <div className="space-y-5">
      <div className="flex items-center gap-3">
        <ScoreBadge score={app.ai_score} />
        <div>
          <div className="text-sm font-semibold">AI Screening</div>
          <div className="text-xs text-muted-foreground">
            {app.must_have_passed === null
              ? "Not yet scored"
              : app.must_have_passed
                ? "Passed must-have gate"
                : "Failed must-have gate"}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-2 text-xs">
        <Badge variant="secondary">status: {app.status}</Badge>
        {app.assigned_store_id !== null && <Badge variant="outline">store {app.assigned_store_id}</Badge>}
        {app.talent_pool && <Badge variant="outline">talent pool</Badge>}
        {app.needs_manual_review && <Badge variant="outline">manual review</Badge>}
        {app.dedup_state && app.dedup_state !== "none" && <Badge variant="outline">dedup: {app.dedup_state}</Badge>}
      </div>

      <div>
        <div className="mb-2 text-xs font-medium uppercase tracking-wide text-muted-foreground">Actions</div>
        <div className="flex flex-wrap gap-2">
          {NEXT_ACTIONS.map((a) => (
            <Button
              key={a.value}
              size="sm"
              variant={a.variant ?? "default"}
              disabled={setStatus.isPending}
              onClick={() => act(a.value, a.label)}
            >
              {a.label}
            </Button>
          ))}
        </div>
      </div>

      <dl className="grid grid-cols-2 gap-x-4 gap-y-2 border-t pt-4 text-xs">
        <dt className="text-muted-foreground">OCR confidence</dt>
        <dd className="tabular-nums">{app.ocr_confidence !== null ? app.ocr_confidence.toFixed(2) : "—"}</dd>
        <dt className="text-muted-foreground">Parsed at</dt>
        <dd>{app.parsed_at ? new Date(app.parsed_at).toLocaleString() : "—"}</dd>
        <dt className="text-muted-foreground">Profile JSON</dt>
        <dd className="truncate">{app.parsed_profile_blob_url ? "stored" : "—"}</dd>
      </dl>
    </div>
  );
}
