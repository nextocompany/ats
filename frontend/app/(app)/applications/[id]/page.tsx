"use client";

import Link from "next/link";
import { useParams } from "next/navigation";

import { AiSummaryPanel } from "@/components/resume/AiSummaryPanel";
import { ResumeViewer } from "@/components/resume/ResumeViewer";
import { Skeleton } from "@/components/ui/skeleton";
import { useApplication } from "@/lib/queries";

export default function ApplicationDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: app, isLoading, isError } = useApplication(id);

  return (
    <div className="space-y-4">
      <Link href="/applications" className="text-sm text-muted-foreground hover:text-foreground">
        ← Back to inbox
      </Link>

      {isLoading && <Skeleton className="h-[70vh] w-full" />}
      {isError && <p className="text-sm text-destructive">Application not found.</p>}

      {app && (
        <div className="grid gap-6 lg:grid-cols-[1fr_360px]">
          <section aria-label="Resume">
            <ResumeViewer applicationId={app.id} />
          </section>
          <aside aria-label="AI summary and actions" className="rounded-lg border p-5">
            <AiSummaryPanel app={app} />
          </aside>
        </div>
      )}
    </div>
  );
}
