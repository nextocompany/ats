"use client";

// Hiring approval chain (Module-3 3.5). From the interviewed stage an hr_staff
// submits the request (Staff sign-off); the four-level chain (Staff → HR Manager →
// SGM → Regional Director) then signs off in order. Reads are open to anyone who
// can see the application; the approve/reject controls appear only for the role
// whose level is currently active. The server is the real gate.
import { useState } from "react";
import { Check, Clock, Loader2, X } from "lucide-react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { Application, ApprovalStep } from "@/lib/types";
import { useApprovalForApplication, useDecideApproval, useMe, useSubmitApproval } from "@/lib/queries";
import { canDecideApprovalLevel, canSubmitApproval } from "@/lib/roles";
import { Button } from "@/components/ui/button";

// Typed level → i18n-key map. Avoids an unsound `as "level1"` cast and degrades to
// the raw number for any out-of-range level rather than showing a raw key string.
const LEVEL_KEYS: Record<number, "level1" | "level2" | "level3" | "level4"> = {
  1: "level1",
  2: "level2",
  3: "level3",
  4: "level4",
};

interface Props {
  applicationId: string;
  app: Application;
}

export function ApprovalPanel({ applicationId, app }: Props) {
  const t = useTranslations("approvals");
  const { data: me } = useMe();
  const { data: req, isLoading } = useApprovalForApplication(applicationId);
  const submit = useSubmitApproval(applicationId);

  if (isLoading) return null;

  // No request yet → offer the submit CTA only at the interviewed stage to the
  // Staff-level roles. Otherwise render nothing (keeps the aside uncluttered).
  if (!req) {
    if (app.status !== "interviewed" || !canSubmitApproval(me)) return null;
    return (
      <Section title={t("chainTitle")}>
        <p className="text-xs text-muted-foreground">{t("submitHint")}</p>
        <Button
          size="sm"
          variant="default"
          className="mt-3 gap-2"
          disabled={submit.isPending}
          onClick={() =>
            submit.mutate(undefined, {
              onSuccess: () => toast.success(t("submitted")),
              onError: (e) => toast.error(e instanceof Error ? e.message : t("submitFailed")),
            })
          }
        >
          {submit.isPending && <Loader2 className="size-4 animate-spin" />}
          {t("submit")}
        </Button>
      </Section>
    );
  }

  const myTurn = req.status === "pending" && canDecideApprovalLevel(me, req.current_level);

  return (
    <Section title={t("chainTitle")}>
      <ol className="flex flex-col gap-2.5">
        {req.steps.map((s) => (
          <StepRow key={s.id} step={s} active={req.status === "pending" && s.level === req.current_level} t={t} />
        ))}
      </ol>

      {req.status === "rejected" && req.decision_reason && (
        <div className="mt-3 rounded-lg bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <p className="font-medium">{t("rejectedTitle")}</p>
          <p className="mt-0.5 text-destructive/90">{req.decision_reason}</p>
        </div>
      )}

      {myTurn && <DecisionControls requestId={req.id} applicationId={applicationId} t={t} />}
    </Section>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <p className="eyebrow">{title}</p>
      <div className="mt-3">{children}</div>
    </section>
  );
}

function StepRow({
  step,
  active,
  t,
}: {
  step: ApprovalStep;
  active: boolean;
  t: ReturnType<typeof useTranslations>;
}) {
  const levelKey = LEVEL_KEYS[step.level];
  const levelLabel = levelKey ? t(levelKey) : String(step.level);
  const icon =
    step.status === "approved" ? (
      <Check className="size-4 text-[var(--score-high)]" />
    ) : step.status === "rejected" ? (
      <X className="size-4 text-destructive" />
    ) : active ? (
      <Clock className="size-4 text-brand" />
    ) : (
      <span className="size-2 rounded-full bg-muted-foreground/40" />
    );

  return (
    <li className="flex items-start gap-2.5">
      <span className="mt-0.5 grid size-5 shrink-0 place-items-center">{icon}</span>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <span className={`text-sm ${active ? "font-semibold text-foreground" : "text-muted-foreground"}`}>
            {step.level}. {levelLabel}
          </span>
          <span className="text-[0.6875rem] uppercase tracking-wide text-muted-foreground">
            {step.status === "approved"
              ? t("approved")
              : step.status === "rejected"
                ? t("rejected")
                : active
                  ? t("awaiting")
                  : t("queued")}
          </span>
        </div>
        {step.status === "approved" && step.approver_name && (
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("approvedBy", { name: step.approver_name })}
            {step.decided_at ? ` · ${new Date(step.decided_at).toLocaleDateString()}` : ""}
          </p>
        )}
        {active && step.due_at && (
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("dueOn", { date: new Date(step.due_at).toLocaleString() })}
            {step.escalated ? ` · ${t("escalated")}` : ""}
          </p>
        )}
      </div>
    </li>
  );
}

function DecisionControls({
  requestId,
  applicationId,
  t,
}: {
  requestId: string;
  applicationId: string;
  t: ReturnType<typeof useTranslations>;
}) {
  const decide = useDecideApproval(requestId, applicationId);
  const [rejecting, setRejecting] = useState(false);
  const [reason, setReason] = useState("");

  function approve() {
    decide.mutate(
      { decision: "approve" },
      {
        onSuccess: () => toast.success(t("approvedToast")),
        onError: (e) => toast.error(e instanceof Error ? e.message : t("decideFailed")),
      },
    );
  }

  function reject(e: React.FormEvent) {
    e.preventDefault();
    const trimmed = reason.trim();
    if (!trimmed) return;
    // mutate (not mutateAsync): the callbacks own all UI feedback and the return is
    // unused — mutateAsync would re-throw after onError and leave an unhandled
    // rejection on this form's submit handler.
    decide.mutate(
      { decision: "reject", reason: trimmed },
      {
        onSuccess: () => {
          toast.success(t("rejectedToast"));
          setRejecting(false);
          setReason("");
        },
        onError: (err) => toast.error(err instanceof Error ? err.message : t("decideFailed")),
      },
    );
  }

  if (!rejecting) {
    return (
      <div className="mt-4 grid grid-cols-2 gap-2">
        <Button size="sm" variant="default" className="gap-2" disabled={decide.isPending} onClick={approve}>
          {decide.isPending && <Loader2 className="size-4 animate-spin" />}
          {t("approve")}
        </Button>
        <Button size="sm" variant="destructive" disabled={decide.isPending} onClick={() => setRejecting(true)}>
          {t("reject")}
        </Button>
      </div>
    );
  }

  return (
    <form onSubmit={reject} className="mt-4 space-y-2" noValidate>
      <span className="text-xs font-medium text-foreground">{t("rejectReasonLabel")}</span>
      <textarea
        value={reason}
        onChange={(e) => setReason(e.target.value)}
        rows={3}
        required
        className="w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        placeholder={t("rejectReasonPlaceholder")}
      />
      {decide.isError && (
        <p role="alert" className="text-xs font-medium text-destructive">
          {decide.error instanceof Error ? decide.error.message : t("decideFailed")}
        </p>
      )}
      <div className="flex justify-end gap-2">
        <Button type="button" size="sm" variant="ghost" onClick={() => setRejecting(false)}>
          {t("cancel")}
        </Button>
        <Button type="submit" size="sm" variant="destructive" className="gap-2" disabled={!reason.trim() || decide.isPending}>
          {decide.isPending && <Loader2 className="size-4 animate-spin" />}
          {t("confirmReject")}
        </Button>
      </div>
    </form>
  );
}
