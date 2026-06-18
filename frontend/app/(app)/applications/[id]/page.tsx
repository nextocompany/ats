"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { ArrowLeft } from "lucide-react";

import { AiSummaryPanel } from "@/components/resume/AiSummaryPanel";
import { ApprovalPanel } from "@/components/resume/ApprovalPanel";
import { OfferPanel } from "@/components/resume/OfferPanel";
import { LettersPanel } from "@/components/resume/LettersPanel";
import { FitAnalysisPanel } from "@/components/resume/FitAnalysisPanel";
import { InterviewPanel } from "@/components/resume/InterviewPanel";
import {
  LineManagerScorecard,
  ScorecardSummary,
  TaScorecard,
} from "@/components/resume/Scorecards";
import { ResumeViewer } from "@/components/resume/ResumeViewer";
import { Skeleton } from "@/components/ui/skeleton";
import { useApplication } from "@/lib/queries";

export default function ApplicationDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: app, isLoading, isError } = useApplication(id);

  return (
    <div className="settle space-y-5">
      <Link
        href="/applications"
        className="inline-flex items-center gap-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
      >
        <ArrowLeft className="size-4" /> Back to inbox
      </Link>

      {isLoading && <Skeleton className="h-[70vh] w-full rounded-xl" />}
      {isError && (
        <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
          Application not found, or it has been removed.
        </div>
      )}

      {app && (
        <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
          <section aria-label="Resume" className="min-w-0">
            <div className="mb-3 flex items-baseline justify-between">
              <p className="eyebrow">Source document</p>
              <span className="font-mono text-xs text-muted-foreground">{app.id.slice(0, 8)}</span>
            </div>
            <ResumeViewer applicationId={app.id} />
          </section>
          <aside
            aria-label="AI summary and actions"
            className="h-fit rounded-xl bg-card p-6 ring-1 ring-hairline lg:sticky lg:top-6"
          >
            <AiSummaryPanel app={app} />
            <ApprovalPanel applicationId={app.id} app={app} />
            <OfferPanel applicationId={app.id} app={app} />
            <LettersPanel applicationId={app.id} app={app} />
            <InterviewPanel applicationId={app.id} />
            <ScorecardSummary applicationId={app.id} />
            <TaScorecard applicationId={app.id} status={app.status} />
            <LineManagerScorecard applicationId={app.id} status={app.status} />
            <FitAnalysisPanel applicationId={app.id} app={app} />
          </aside>
        </div>
      )}
    </div>
  );
}
