"use client";

// Create / edit a requisition (a manual position opening). A single dialog serves
// both modes via a `mode` prop, mirroring components/admin/RolesPermissions.tsx.
import { useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import {
  useCreateRequisition,
  useUpdateRequisition,
  usePositions,
  usePosition,
  useStores,
} from "@/lib/queries";
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

// Enum option values; their labels come from i18n keys (employment_*, priority_*, reason_*).
const EMPLOYMENT_TYPES = ["full_time", "part_time", "contract", "seasonal"];
const PRIORITIES = ["normal", "urgent"];
const OPEN_REASONS = ["new_headcount", "replacement"];

const TEXTAREA_CLASS =
  "w-full rounded-lg border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50";

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

  // Hydrate from the edited row's identity (no useEffect for these — keyed remount via `open`).
  const [positionId, setPositionId] = useState(requisition?.position_id ?? "");
  const [storeId, setStoreId] = useState(requisition?.store_id ? String(requisition.store_id) : "");
  const [headcount, setHeadcount] = useState(String(requisition?.headcount ?? 1));
  const [employmentType, setEmploymentType] = useState(requisition?.employment_type ?? "");
  const [priority, setPriority] = useState(requisition?.priority || "normal");
  const [openReason, setOpenReason] = useState(requisition?.open_reason ?? "");
  const [salaryMin, setSalaryMin] = useState(
    requisition?.salary_min != null ? String(requisition.salary_min) : "",
  );
  const [salaryMax, setSalaryMax] = useState(
    requisition?.salary_max != null ? String(requisition.salary_max) : "",
  );
  const [responsibilities, setResponsibilities] = useState(requisition?.responsibilities ?? "");
  const [qualifications, setQualifications] = useState(requisition?.qualifications ?? "");
  const [benefits, setBenefits] = useState(requisition?.benefits ?? "");
  const [other, setOther] = useState(requisition?.other_details ?? "");

  // Prefill the JD text from the chosen position (create mode only). Switching the
  // position re-prefills, so the JD always reflects the selected position. This uses
  // React's "adjust state while rendering" pattern (a once-per-position guard) instead
  // of an effect, so the project's react-hooks/set-state-in-effect rule stays happy.
  const prefillEnabled = mode === "create" && positionId !== "";
  const { data: posDetail } = usePosition(positionId, prefillEnabled);
  const [prefilledFrom, setPrefilledFrom] = useState<string | null>(null);
  if (mode === "create" && posDetail && posDetail.id === positionId && prefilledFrom !== positionId) {
    setPrefilledFrom(positionId);
    setResponsibilities(posDetail.responsibilities ?? "");
    setQualifications(posDetail.qualifications ?? "");
    setBenefits(posDetail.benefits ?? "");
  }

  const busy = create.isPending || update.isPending;
  const activeError = errMessage(create.error) || errMessage(update.error);
  const heads = Number(headcount);
  const canSubmit = positionId !== "" && storeId !== "" && heads >= 1 && !busy;
  // Once approved (open), position + store are locked — only the details are editable
  // (matches the backend Update guard). Pending requisitions remain fully editable.
  const lockIdentity = mode === "edit" && requisition?.status === "open";

  function close() {
    create.reset();
    update.reset();
    onClose();
  }

  function buildInput(): RequisitionInput {
    return {
      position_id: positionId,
      store_id: Number(storeId),
      headcount: heads,
      responsibilities,
      qualifications,
      benefits,
      other_details: other,
      employment_type: employmentType,
      priority,
      open_reason: openReason,
      salary_min: salaryMin === "" ? undefined : Number(salaryMin),
      salary_max: salaryMax === "" ? undefined : Number(salaryMax),
    };
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    if (mode === "create") {
      await create.mutateAsync(buildInput(), {
        onSuccess: () => {
          toast.success(t("createdToast"));
          close();
        },
        onError: (err) => toast.error(errMessage(err) ?? t("actionFailed")),
      });
    } else if (requisition) {
      await update.mutateAsync(
        { id: requisition.id, input: buildInput() },
        {
          onSuccess: close,
          onError: (err) => toast.error(errMessage(err) ?? t("actionFailed")),
        },
      );
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => (o ? null : close())}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{mode === "create" ? t("createTitle") : t("editTitle")}</DialogTitle>
          <DialogDescription>{mode === "create" ? t("createDesc") : t("editDesc")}</DialogDescription>
        </DialogHeader>

        <form onSubmit={submit} className="space-y-4" noValidate>
          <div className="max-h-[65vh] space-y-4 overflow-y-auto pr-1">
            <label className="block space-y-1.5">
              <span className="text-xs font-medium text-foreground">{t("fieldPosition")}</span>
              <Select value={positionId} onValueChange={(v) => setPositionId(v ?? "")} disabled={lockIdentity}>
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
              <Select value={storeId} onValueChange={(v) => setStoreId(v ?? "")} disabled={lockIdentity}>
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

            {lockIdentity && <p className="text-xs text-muted-foreground">{t("lockIdentityHint")}</p>}

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
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

              <label className="block space-y-1.5">
                <span className="text-xs font-medium text-foreground">{t("fieldEmploymentType")}</span>
                <Select value={employmentType} onValueChange={(v) => setEmploymentType(v ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t("employmentPlaceholder")}>
                      {(v: string | null) => (v ? t(`employment_${v}`) : t("employmentPlaceholder"))}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {EMPLOYMENT_TYPES.map((e) => (
                      <SelectItem key={e} value={e}>
                        {t(`employment_${e}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </label>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <label className="block space-y-1.5">
                <span className="text-xs font-medium text-foreground">{t("fieldPriority")}</span>
                <Select value={priority} onValueChange={(v) => setPriority(v ?? "normal")}>
                  <SelectTrigger className="w-full">
                    <SelectValue>
                      {(v: string | null) => t(`priority_${v || "normal"}`)}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {PRIORITIES.map((p) => (
                      <SelectItem key={p} value={p}>
                        {t(`priority_${p}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </label>

              <label className="block space-y-1.5">
                <span className="text-xs font-medium text-foreground">{t("fieldReason")}</span>
                <Select value={openReason} onValueChange={(v) => setOpenReason(v ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t("reasonPlaceholder")}>
                      {(v: string | null) => (v ? t(`reason_${v}`) : t("reasonPlaceholder"))}
                    </SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {OPEN_REASONS.map((r) => (
                      <SelectItem key={r} value={r}>
                        {t(`reason_${r}`)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </label>
            </div>

            <fieldset className="space-y-1.5">
              <legend className="text-xs font-medium text-foreground">{t("fieldSalary")}</legend>
              <div className="grid grid-cols-2 gap-4">
                <Input
                  type="number"
                  min={0}
                  value={salaryMin}
                  onChange={(e) => setSalaryMin(e.target.value)}
                  placeholder={t("salaryMinPlaceholder")}
                />
                <Input
                  type="number"
                  min={0}
                  value={salaryMax}
                  onChange={(e) => setSalaryMax(e.target.value)}
                  placeholder={t("salaryMaxPlaceholder")}
                />
              </div>
              <p className="text-xs text-muted-foreground">{t("salaryHint")}</p>
            </fieldset>

            <label className="block space-y-1.5">
              <span className="text-xs font-medium text-foreground">{t("fieldResponsibilities")}</span>
              <textarea
                value={responsibilities}
                onChange={(e) => setResponsibilities(e.target.value)}
                rows={4}
                className={TEXTAREA_CLASS}
                placeholder={t("responsibilitiesPlaceholder")}
              />
            </label>

            <label className="block space-y-1.5">
              <span className="text-xs font-medium text-foreground">{t("fieldQualifications")}</span>
              <textarea
                value={qualifications}
                onChange={(e) => setQualifications(e.target.value)}
                rows={4}
                className={TEXTAREA_CLASS}
                placeholder={t("qualificationsPlaceholder")}
              />
            </label>

            <label className="block space-y-1.5">
              <span className="text-xs font-medium text-foreground">{t("fieldBenefits")}</span>
              <textarea
                value={benefits}
                onChange={(e) => setBenefits(e.target.value)}
                rows={3}
                className={TEXTAREA_CLASS}
                placeholder={t("benefitsPlaceholder")}
              />
            </label>

            <label className="block space-y-1.5">
              <span className="text-xs font-medium text-foreground">{t("fieldOther")}</span>
              <textarea
                value={other}
                onChange={(e) => setOther(e.target.value)}
                rows={3}
                className={TEXTAREA_CLASS}
                placeholder={t("otherPlaceholder")}
              />
            </label>

            <p className="text-xs text-muted-foreground">{t("jdInternalHint")}</p>
          </div>

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
