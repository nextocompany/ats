"use client";

import { useTranslations } from "next-intl";
import { FileText } from "lucide-react";

import { useResumeUrl } from "@/lib/queries";
import { Skeleton } from "@/components/ui/skeleton";

// Browsers can only render these inline in an iframe. Any other type (notably
// .docx) makes the browser DOWNLOAD the file the moment the iframe loads — which
// looked like "opening a candidate randomly downloads their resume". So we only
// iframe renderable types and offer an explicit open button for the rest.
const INLINE_TYPES = new Set(["pdf", "image"]);

export function ResumeViewer({ applicationId, fileType }: { applicationId: string; fileType?: string }) {
  const t = useTranslations("resume");
  const { data: url, isLoading, isError } = useResumeUrl(applicationId);

  if (isLoading) return <Skeleton className="h-[70vh] w-full rounded-xl" />;
  if (isError || !url) {
    return (
      <div className="grid h-[70vh] place-items-center rounded-xl bg-card text-center ring-1 ring-hairline">
        <div>
          <p className="text-sm font-medium text-foreground">{t("rvNoResume")}</p>
          <p className="mt-1 text-sm text-muted-foreground">{t("rvNoResumeBody")}</p>
        </div>
      </div>
    );
  }

  // Non-renderable (e.g. .docx): never iframe it (that auto-downloads on load).
  // Show a card with an explicit, user-initiated open instead.
  if (fileType && !INLINE_TYPES.has(fileType)) {
    return (
      <div className="grid h-[70vh] place-items-center rounded-xl bg-card text-center ring-1 ring-hairline">
        <div className="px-6">
          <FileText className="mx-auto size-10 text-muted-foreground/60" />
          <p className="mt-3 text-sm font-medium text-foreground">{t("rvNoPreview")}</p>
          <p className="mt-1 text-sm text-muted-foreground">{t("rvNoPreviewBody", { type: fileType.toUpperCase() })}</p>
          <a
            href={url}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-4 inline-flex items-center gap-2 rounded-lg bg-brand px-4 py-2 text-sm font-medium text-brand-foreground transition-colors hover:bg-brand/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t("rvOpenFile")}
          </a>
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
      <iframe title={t("rvTitle")} src={url} className="h-[70vh] w-full bg-white" referrerPolicy="no-referrer" />
      <a
        href={url}
        target="_blank"
        rel="noopener noreferrer"
        className="flex items-center justify-between border-t border-hairline bg-muted/40 px-4 py-2.5 text-xs font-medium text-brand transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
      >
        {t("rvOpenNewTab")}
        <span className="text-muted-foreground">{t("rvSignedLink")}</span>
      </a>
    </div>
  );
}
