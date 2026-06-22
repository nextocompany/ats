"use client";

// Create / edit a requisition (a manual position opening). A single dialog serves
// both modes via a `mode` prop, mirroring components/admin/RolesPermissions.tsx.
import { useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { useCreateRequisition, useUpdateRequisition, usePositions, useStores } from "@/lib/queries";
import type { Requisition, RequisitionInput } from "@/lib/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

type Props =
  | { mode: "create"; open: boolean; onClose: () => void; requisition?: undefined }
  | { mode: "edit"; open: boolean; onClose: () => void; requisition: Requisition | null };

function errMessage(err: unknown): string | null {
  return err instanceof Error ? err.message : null;
}

export function RequisitionDialog({ mode, open, onClose, requisition }: Props) {
  const t = useTranslations("requisitions");
  const locale = useLocale();
  const { data: positions } = usePositions();
  const { data: stores } = useStores();
  const create = useCreateRequisition();
  const update = useUpdateRequisition();

  // Hydrate from the edited row's identity (no useEffect — keyed remount via `open`).
  const [positionId, setPositionId] = useState(requisition?.position_id ?? "");
  const [storeId, setStoreId] = useState(requisition?.store_id ? String(requisition.store_id) : "");
  const [headcount, setHeadcount] = useState(String(requisition?.headcount ?? 1));

  const busy = create.isPending || update.isPending;
  const activeError = errMessage(create.error) || errMessage(update.error);
  const heads = Number(headcount);
  const canSubmit = positionId !== "" && storeId !== "" && heads >= 1 && !busy;

  function close() {
    create.reset();
    update.reset();
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    if (mode === "create") {
      const input: RequisitionInput = { position_id: positionId, store_id: Number(storeId), headcount: heads };
      await create.mutateAsync(input, {
        onSuccess: () => {
          toast.success(t("createdToast"));
          close();
        },
        onError: (err) => toast.error(errMessage(err) ?? t("actionFailed")),
      });
    } else if (requisition) {
      const input: RequisitionInput = { position_id: positionId, store_id: Number(storeId), headcount: heads };
      await update.mutateAsync(
        { id: requisition.id, input },
        {
          onSuccess: close,
          onError: (err) => toast.error(errMessage(err) ?? t("actionFailed")),
        },
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : close())}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{mode === "create" ? t("createTitle") : t("editTitle")}</DialogTitle>
          <DialogDescription>{mode === "create" ? t("createDesc") : t("editDesc")}</DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-4" noValidate>
          <label className="block space-y-1.5">
            <span className="text-xs font-medium text-foreground">{t("fieldPosition")}</span>
            <Select value={positionId} onValueChange={(v) => setPositionId(v ?? "")}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("positionPlaceholder")}>
                  {(v: string | null) => {
                    const p = (positions ?? []).find((p) => p.id === v)
                    if (!p) return t("positionPlaceholder")
                    return locale === "th" ? p.title_th || p.title_en : p.title_en || p.title_th
                  }}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {(positions ?? []).map((p) => (
                  <SelectItem key={p.id} value={p.id}>
                    {locale === "th" ? p.title_th || p.title_en : p.title_en || p.title_th}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>

          <label className="block space-y-1.5">
            <span className="text-xs font-medium text-foreground">{t("fieldStore")}</span>
            <Select value={storeId} onValueChange={(v) => setStoreId(v ?? "")}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("storePlaceholder")}>
                  {(v: string | null) =>
                    (stores ?? []).find((s) => String(s.store_no) === v)?.store_name ?? t("storePlaceholder")
                  }
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {(stores ?? []).map((s) => (
                  <SelectItem key={s.store_no} value={String(s.store_no)}>
                    {s.store_name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </label>

          <label className="block space-y-1.5">
            <span className="text-xs font-medium text-foreground">{t("fieldHeadcount")}</span>
            <Input
              type="number"
              min={1}
              max={999}
              value={headcount}
              onChange={(e) => setHeadcount(e.target.value)}
            />
          </label>

          {activeError && <p role="alert" className="text-xs font-medium text-destructive">{activeError}</p>}

          <DialogFooter className="gap-2 sm:gap-2">
            <Button type="button" variant="ghost" onClick={close}>
              {t("cancel")}
            </Button>
            <Button type="submit" disabled={!canSubmit} className="gap-2">
              {busy && <Loader2 className="size-4 animate-spin" />}
              {mode === "create" ? t("createBtn") : t("saveBtn")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
