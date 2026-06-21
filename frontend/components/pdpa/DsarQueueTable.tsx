"use client";

// The held DSAR request queue: each row is a data-subject erasure request that
// could not be auto-fulfilled (legal hold). The DPO completes it (after handling
// the data outside the system) or rejects it with a reason. Both actions are
// audited server-side.
import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useCompleteDsar, useRejectDsar } from "@/lib/queries";
import { formatDateTime } from "@/lib/format";
import type { DsarRequest } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

const STATUS_TONE: Record<string, string> = {
  pending: "bg-[var(--score-mid)]/15 text-[var(--score-mid)]",
  completed: "bg-brand-soft text-brand",
  rejected: "bg-destructive/10 text-destructive",
};

function errMessage(err: unknown): string {
  return err instanceof Error ? err.message : "";
}

export function DsarQueueTable({ requests }: { requests: DsarRequest[] }) {
  const t = useTranslations("pdpa");
  const complete = useCompleteDsar();
  const reject = useRejectDsar();
  const [rejecting, setRejecting] = useState<DsarRequest | null>(null);
  const [reason, setReason] = useState("");

  const busy = complete.isPending || reject.isPending;

  async function onComplete(id: string) {
    await complete.mutateAsync(id, {
      onSuccess: () => toast.success(t("completedToast")),
      onError: (e) => toast.error(errMessage(e) || t("actionFailed")),
    });
  }

  async function onReject() {
    if (!rejecting || reason.trim() === "") return;
    await reject.mutateAsync(
      { id: rejecting.id, reason: reason.trim() },
      {
        onSuccess: () => {
          toast.success(t("rejectedToast"));
          setRejecting(null);
          setReason("");
        },
        onError: (e) => toast.error(errMessage(e) || t("actionFailed")),
      },
    );
  }

  function statusLabel(s: string): string {
    return t.has(`dsarStatus_${s}`) ? t(`dsarStatus_${s}`) : s;
  }

  return (
    <>
      <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-hairline text-left text-xs uppercase tracking-wide text-muted-foreground">
              <th scope="col" className="px-5 py-3 font-medium">{t("colSubject")}</th>
              <th scope="col" className="px-5 py-3 font-medium">{t("colType")}</th>
              <th scope="col" className="px-5 py-3 font-medium">{t("colRequested")}</th>
              <th scope="col" className="px-5 py-3 font-medium">{t("colStatus")}</th>
              <th scope="col" className="px-5 py-3">
                <span className="sr-only">{t("colActions")}</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {requests.map((r) => {
              const pending = r.status === "pending";
              return (
                <tr key={r.id} className="border-b border-hairline last:border-0">
                  <td className="px-5 py-3">
                    <div className="font-medium text-foreground">{r.account_name || t("unnamedSubject")}</div>
                    <div className="text-xs text-muted-foreground">{r.account_email || "-"}</div>
                    {r.reason && !pending && (
                      <div className="mt-1 text-xs text-muted-foreground">{t("heldReason", { reason: r.reason })}</div>
                    )}
                  </td>
                  <td className="px-5 py-3 text-foreground">{r.request_type}</td>
                  <td className="px-5 py-3 text-muted-foreground tabular-nums">{formatDateTime(r.requested_at)}</td>
                  <td className="px-5 py-3">
                    <Badge variant="outline" className={`rounded-full border-0 ${STATUS_TONE[r.status] ?? ""}`}>
                      {statusLabel(r.status)}
                    </Badge>
                  </td>
                  <td className="px-5 py-3 text-right">
                    {pending && (
                      <div className="flex justify-end gap-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          aria-label={t("rejectFor", { name: r.account_name || r.account_email || r.id })}
                          onClick={() => {
                            setRejecting(r);
                            setReason("");
                          }}
                          disabled={busy}
                        >
                          {t("reject")}
                        </Button>
                        <Button
                          size="sm"
                          aria-label={t("completeFor", { name: r.account_name || r.account_email || r.id })}
                          onClick={() => void onComplete(r.id)}
                          disabled={busy}
                        >
                          {t("complete")}
                        </Button>
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      <Dialog
        open={!!rejecting}
        onOpenChange={(o) => {
          if (!o) {
            setRejecting(null);
            setReason("");
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("rejectTitle")}</DialogTitle>
            <DialogDescription>{t("rejectDescription")}</DialogDescription>
          </DialogHeader>
          <label htmlFor="dsar-reject-reason" className="sr-only">
            {t("rejectReasonPlaceholder")}
          </label>
          <textarea
            id="dsar-reject-reason"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            rows={4}
            maxLength={1000}
            placeholder={t("rejectReasonPlaceholder")}
            className="w-full rounded-md border border-hairline bg-background px-3 py-2 text-sm outline-none ring-brand focus:ring-2"
          />
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => {
                setRejecting(null);
                setReason("");
              }}
              disabled={reject.isPending}
            >
              {t("cancel")}
            </Button>
            <Button
              variant="destructive"
              onClick={() => void onReject()}
              disabled={reason.trim() === "" || reject.isPending}
            >
              {t("confirmReject")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
