"use client";

import { useResumeUrl } from "@/lib/queries";
import { Skeleton } from "@/components/ui/skeleton";

export function ResumeViewer({ applicationId }: { applicationId: string }) {
  const { data: url, isLoading, isError } = useResumeUrl(applicationId);

  if (isLoading) return <Skeleton className="h-[70vh] w-full rounded-lg" />;
  if (isError || !url) {
    return (
      <div className="grid h-[70vh] place-items-center rounded-lg border bg-muted/20 text-sm text-muted-foreground">
        No resume available for this application.
      </div>
    );
  }
  return (
    <div className="overflow-hidden rounded-lg border">
      <iframe title="resume" src={url} className="h-[70vh] w-full" sandbox="allow-same-origin allow-scripts" />
      <a
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className="block border-t bg-muted/30 px-3 py-2 text-xs text-[var(--color-accent)] hover:underline"
      >
        Open resume in new tab (signed link, expires shortly)
      </a>
    </div>
  );
}
