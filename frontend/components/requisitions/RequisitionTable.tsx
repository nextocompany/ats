"use client";

// Requisition rows with a status badge and per-row actions. Approve is shown only
// to approvers on a pending row; Edit/Close to managers per status.
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { useApproveRequisition, useCloseRequisition } from "@/lib/queries";
import { canApproveRequisitions } from "@/lib/roles";
import type { Me, Requisition } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

const STATUS_TONE: Record<string, string> = {
  pending_approval: "bg-[var(--score-mid)]/15 text-[var(--score-mid)]",
  open: "bg-brand-soft text-brand",
  closed: "bg-muted text-muted-foreground",
  cancelled: "bg-destructive/10 text-destructive",
};

function errMessage(err: unknown): string {
  return err instanceof Error ? err.message : "";
}

export function RequisitionTable({
  requisitions,
  me,
  onEdit,
}: {
  requisitions: Requisition[];
  me?: Me;
  onEdit: (r: Requisition) => void;
}) {
  const t = useTranslations("requisitions");
  const approve = useApproveRequisition();
  const close = useCloseRequisition();
  const canApprove = canApproveRequisitions(me);

  function statusLabel(status: string): string {
    return t.has(`status_${status}`) ? t(`status_${status}`) : status;
  }

  async function onApprove(id: string) {
    await approve.mutateAsync(id, {
      onSuccess: () => toast.success(t("approvedToast")),
      onError: (e) => toast.error(errMessage(e) || t("actionFailed")),
    });
  }
  async function onClose(id: string) {
    await close.mutateAsync(id, {
      onSuccess: () => toast.success(t("closedToast")),
      onError: (e) => toast.error(errMessage(e) || t("actionFailed")),
    });
  }

  const busy = approve.isPending || close.isPending;

  return (
    <div className="overflow-hidden rounded-xl bg-card ring-1 ring-hairline">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-hairline text-left text-xs uppercase tracking-wide text-muted-foreground">
            <th className="px-5 py-3 font-medium">{t("colPosition")}</th>
            <th className="px-5 py-3 font-medium">{t("colStore")}</th>
            <th className="px-5 py-3 font-medium tabular-nums">{t("colHeadcount")}</th>
            <th className="px-5 py-3 font-medium">{t("colStatus")}</th>
            <th className="px-5 py-3" aria-hidden />
          </tr>
        </thead>
        <tbody>
          {requisitions.map((r) => {
            const pending = r.status === "pending_approval";
            const manual = r.source === "manual";
            return (
              <tr key={r.id} className="border-b border-hairline last:border-0">
                <td className="px-5 py-3 font-medium text-foreground">
                  {r.position_title || "-"}
                  {!manual && (
                    <span className="ml-2 text-xs text-muted-foreground">{t("source_peoplesoft")}</span>
                  )}
                </td>
                <td className="px-5 py-3 text-foreground">{r.store_name || "-"}</td>
                <td className="px-5 py-3 tabular-nums text-foreground">{r.headcount}</td>
                <td className="px-5 py-3">
                  <Badge variant="outline" className={`rounded-full border-0 ${STATUS_TONE[r.status] ?? ""}`}>
                    {statusLabel(r.status)}
                  </Badge>
                </td>
                <td className="px-5 py-3 text-right">
                  <div className="flex justify-end gap-2">
                    {manual && pending && (
                      <Button variant="ghost" size="sm" onClick={() => onEdit(r)} disabled={busy}>
                        {t("edit")}
                      </Button>
                    )}
                    {manual && pending && canApprove && (
                      <Button size="sm" onClick={() => onApprove(r.id)} disabled={busy}>
                        {t("approve")}
                      </Button>
                    )}
                    {manual && r.status === "open" && (
                      <Button variant="ghost" size="sm" onClick={() => onClose(r.id)} disabled={busy}>
                        {t("close")}
                      </Button>
                    )}
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
