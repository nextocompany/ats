"use client";

import { useResumeUrl } from "@/lib/queries";
import { Skeleton } from "@/components/ui/skeleton";

export function ResumeViewer({ applicationId }: { applicationId: string }) {
  const { data: url, isLoading, isError } = useResumeUrl(applicationId);

  if (isLoading) return <Skeleton className="h-[70vh] w-full rounded-xl" />;
  if (isError || !url) {
    return (
      <div className="grid h-[70vh] place-items-center rounded-xl bg-card text-center ring-1 ring-hairline">
        <div>
          <p className="text-sm font-medium text-foreground">No resume on file</p>
          <p className="mt-1 text-sm text-muted-foreground">This application has no stored source document.</p>
        </div>
      </div>
    );
  }
  return (
    <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
      {/* No `sandbox`: Chrome refuses to render PDFs inside a sandboxed iframe
          ("This page has been blocked by Chrome"). The document is cross-origin
          (a short-lived Azure Blob SAS URL), so same-origin policy already keeps
          it isolated from the dashboard. */}
      <iframe title="resume" src={url} className="h-[70vh] w-full bg-white" referrerPolicy="no-referrer" />
      <a
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className="flex items-center justify-between border-t border-hairline bg-muted/40 px-4 py-2.5 text-xs font-medium text-brand transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
      >
        Open resume in new tab
        <span className="text-muted-foreground">signed link · expires shortly ↗</span>
      </a>
    </div>
  );
}
