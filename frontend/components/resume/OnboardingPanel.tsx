"use client";

// Onboarding documents (Module-3 3.8). Once a candidate is hired they upload a
// checklist of required documents from the career-portal; HR (onboarding roles)
// reviews each one here — approve, or reject with a reason. Completion is derived
// (every required document approved). HR-only panel; shown for hired applications.
import { useState } from "react";
import { Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { Application, DocStatus, OnboardingDoc } from "@/lib/types";
import { useMe, useOnboarding, useReviewOnboardingDoc } from "@/lib/queries";
import { canManageOnboarding } from "@/lib/roles";
import { Button } from "@/components/ui/button";

// Typed status → i18n-key map: adding a DocStatus without a label is a compile
// error (no unsound `as` cast).
const STATUS_KEY: Record<DocStatus, "status_pending" | "status_approved" | "status_rejected"> = {
  pending: "status_pending",
  approved: "status_approved",
  rejected: "status_rejected",
};

interface Props {
  applicationId: string;
  app: Application;
}

export function OnboardingPanel({ applicationId, app }: Props) {
  const t = useTranslations("onboarding");
  const { data: me } = useMe();
  const { data: status } = useOnboarding(applicationId);
  const canManage = canManageOnboarding(me);

  // Onboarding only applies once hired; HR-only surface.
  if (app.status !== "hired" || !canManage || !status) return null;

  const byType = new Map(status.documents.map((d) => [d.doc_type, d]));

  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <div className="flex items-center justify-between">
        <p className="eyebrow">{t("title")}</p>
        <span className="text-xs font-medium tabular-nums text-muted-foreground">
          {t("progress", { approved: status.approved_count, required: status.required_count })}
        </span>
      </div>

      {status.closed && (
        <p className="mt-3 rounded-lg bg-emerald-500/10 px-3 py-2 text-xs font-medium text-emerald-700 dark:text-emerald-400">
          {t("caseClosed")}
        </p>
      )}

      <ul className="mt-3 flex flex-col gap-2">
        {status.required.map((type) => (
          <OnboardingRow key={type} applicationId={applicationId} docType={type} doc={byType.get(type) ?? null} t={t} />
        ))}
      </ul>
    </section>
  );
}

function OnboardingRow({
  applicationId,
  docType,
  doc,
  t,
}: {
  applicationId: string;
  docType: string;
  doc: OnboardingDoc | null;
  t: ReturnType<typeof useTranslations>;
}) {
  const review = useReviewOnboardingDoc(applicationId);
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");

  const pending = review.isPending && review.variables?.docId === doc?.id;

  function approve() {
    if (!doc) return;
    review.mutate(
      { docId: doc.id, decision: "approve" },
      {
        onSuccess: () => toast.success(t("reviewed")),
        onError: (e) => toast.error(e instanceof Error ? e.message : t("reviewFailed")),
      },
    );
  }

  function reject() {
    if (!doc) return;
    const trimmed = reason.trim();
    if (!trimmed) return;
    review.mutate(
      { docId: doc.id, decision: "reject", reason: trimmed },
      {
        onSuccess: () => {
          toast.success(t("reviewed"));
          setRejecting(false);
          setReason("");
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : t("reviewFailed")),
      },
    );
  }

  return (
    <li className="rounded-lg bg-muted/40 px-3 py-2">
      <div className="flex items-center justify-between gap-3">
        <span className="text-sm text-foreground">{t(`doc_${docType}`)}</span>
        {doc ? (
          <span className="flex items-center gap-3 text-xs">
            {doc.url && (
              <a
                href={doc.url}
                target="_blank"
                rel="noopener noreferrer"
                className="font-medium text-brand underline-offset-2 hover:underline"
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

      {doc && doc.status === "rejected" && doc.review_reason && !rejecting && (
        <p className="mt-1.5 text-xs text-destructive/90">
          {t("rejectedReason", { reason: doc.review_reason })}
        </p>
      )}

      {doc && (
        <div className="mt-2 flex flex-col gap-2">
          {rejecting && (
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={2}
              placeholder={t("reasonPlaceholder")}
              className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
            />
          )}
          <div className="flex justify-end gap-2">
            {rejecting ? (
              <>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    setRejecting(false);
                    setReason("");
                  }}
                  disabled={pending}
                >
                  {t("cancel")}
                </Button>
                <Button
                  size="sm"
                  variant="destructive"
                  className="gap-2"
                  onClick={reject}
                  disabled={pending || !reason.trim()}
                >
                  {pending && <Loader2 className="size-4 animate-spin" />}
                  {t("reject")}
                </Button>
              </>
            ) : (
              <>
                <Button size="sm" variant="ghost" onClick={() => setRejecting(true)} disabled={pending}>
                  {t("reject")}
                </Button>
                <Button size="sm" variant="secondary" className="gap-2" onClick={approve} disabled={pending}>
                  {pending && <Loader2 className="size-4 animate-spin" />}
                  {t("approve")}
                </Button>
              </>
            )}
          </div>
        </div>
      )}
    </li>
  );
}

function statusClass(status: DocStatus): string {
  if (status === "approved") return "font-semibold text-brand";
  if (status === "rejected") return "font-semibold text-destructive";
  return "font-medium text-muted-foreground";
}
