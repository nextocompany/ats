"use client";

// Onboarding documents (Module-3 3.8). A hired member uploads each required
// document; HR reviews them. Self-gates: when the member has no hired application
// useMyOnboarding resolves to null and the section renders nothing. The portal has
// no global toaster, so errors render inline (role="alert").
import { useState } from "react";

import { AccountSection } from "@/components/account/AccountSection";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useMyOnboarding, useUploadOnboardingDoc } from "@/lib/queries";
import type { DocStatus, OnboardingDoc } from "@/lib/types";
import { UPLOAD_ACCEPT_ATTR, validateUploadFile, type UploadFileError } from "@/lib/upload";
import { useTranslations } from "next-intl";

const STATUS_KEY: Record<DocStatus, "status_pending" | "status_approved" | "status_rejected"> = {
  pending: "status_pending",
  approved: "status_approved",
  rejected: "status_rejected",
};

export function OnboardingSection() {
  const t = useTranslations("onboarding");
  const { data: status } = useMyOnboarding();

  // No hired application / no checklist → render nothing (normal for most members).
  if (!status) return null;

  const byType = new Map(status.documents.map((d) => [d.doc_type, d]));

  return (
    <AccountSection
      eyebrow="เอกสารเริ่มงาน"
      title={t("title")}
      action={
        <span className="num text-sm font-semibold tabular-nums text-muted-foreground">
          {t("progress", { approved: status.approved_count, required: status.required_count })}
        </span>
      }
    >
      <div className="flex flex-col gap-3">
        {status.required.map((type) => (
          <OnboardingDocRow key={type} docType={type} doc={byType.get(type) ?? null} t={t} />
        ))}
      </div>
    </AccountSection>
  );
}

function OnboardingDocRow({
  docType,
  doc,
  t,
}: {
  docType: string;
  doc: OnboardingDoc | null;
  t: ReturnType<typeof useTranslations>;
}) {
  const upload = useUploadOnboardingDoc();
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState<UploadFileError | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const pending = upload.isPending && upload.variables?.docType === docType;

  function handleFile(e: React.ChangeEvent<HTMLInputElement>) {
    const picked = e.target.files?.[0] ?? null;
    if (!picked) {
      setFile(null);
      setFileError(null);
      return;
    }
    const err = validateUploadFile(picked);
    setFileError(err);
    setFile(err ? null : picked);
  }

  function submit() {
    if (!file || fileError) return;
    setSubmitError(null);
    upload.mutate(
      { docType, file },
      {
        onSuccess: () => {
          setFile(null);
        },
        onError: (e) => setSubmitError(e instanceof Error ? e.message : t("uploadFailed")),
      },
    );
  }

  return (
    <div className="flex flex-col gap-2 border-b border-line pb-3 last:border-0 last:pb-0">
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm font-medium text-foreground">{t(`doc_${docType}`)}</span>
        {doc ? (
          <span className="flex items-center gap-3 text-xs">
            {doc.url && (
              <a
                href={doc.url}
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-primary underline-offset-2 hover:underline"
              >
                {t("view")}
              </a>
            )}
            <span className={statusClass(doc.status)}>{t(STATUS_KEY[doc.status])}</span>
          </span>
        ) : (
          <span className="text-xs text-muted-foreground">{t("notUploaded")}</span>
        )}
      </div>

      {doc && doc.status === "rejected" && doc.review_reason && (
        <p className="text-xs text-destructive">{t("rejectedReason", { reason: doc.review_reason })}</p>
      )}

      <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
        <Input
          type="file"
          accept={UPLOAD_ACCEPT_ATTR}
          onChange={handleFile}
          aria-invalid={!!fileError}
          className="h-auto py-2 file:mr-3 file:rounded-md file:bg-secondary file:px-3 file:py-1.5"
        />
        <Button type="button" size="sm" onClick={submit} disabled={!file || !!fileError || pending} className="shrink-0">
          {pending ? t("uploading") : doc ? t("replace") : t("upload")}
        </Button>
      </div>

      {fileError && (
        <p role="alert" className="text-xs text-destructive">
          {t(fileError)}
        </p>
      )}
      {submitError && (
        <p role="alert" className="text-xs text-destructive">
          {submitError}
        </p>
      )}
    </div>
  );
}

function statusClass(status: DocStatus): string {
  if (status === "approved") return "font-semibold text-[oklch(45%_0.14_150)]";
  if (status === "rejected") return "font-semibold text-destructive";
  return "font-medium text-muted-foreground";
}
