"use client";

// Manual branch (re)assignment. Branch assignment is automatic at intake (nearest
// store with an open vacancy); when there's no nearby branch the candidate sits in
// the central pool. This control lets broader-visibility HR roles override that —
// pick a store, or move the candidate (back) to the central pool to be assigned
// later. Role-gated to mirror the backend (super_admin / hr_manager / sgm).
import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import type { Application } from "@/lib/types";
import { useMe, useReassign, useStores } from "@/lib/queries";
import { canReassignPlacement } from "@/lib/roles";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";

export function ReassignControl({ applicationId, app }: { applicationId: string; app: Application }) {
  const t = useTranslations("resume");
  const { data: me } = useMe();
  const { data: stores } = useStores();
  const reassign = useReassign(applicationId);
  const [value, setValue] = useState<string>(app.assigned_store_id ? String(app.assigned_store_id) : "");

  if (!canReassignPlacement(me)) return null;

  const current = app.store_name ?? (app.talent_pool ? t("badgePool") : "-");

  function assignStore(storeNo: string | null) {
    if (!storeNo) return;
    setValue(storeNo);
    reassign.mutate(
      { store_no: Number(storeNo) },
      {
        onSuccess: () => toast.success(t("reassignedStore")),
        onError: (e) => toast.error(e instanceof Error ? e.message : t("reassignStoreFailed")),
      },
    );
  }

  function moveToPool() {
    setValue("");
    reassign.mutate(
      { talent_pool: true },
      {
        onSuccess: () => toast.success(t("reassignedPool")),
        onError: (e) => toast.error(e instanceof Error ? e.message : t("reassignPoolFailed")),
      },
    );
  }

  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <p className="eyebrow">{t("reassignTitle")}</p>
      <p className="mt-1 text-sm text-muted-foreground">
        {t("reassignCurrent")} <span className="font-medium text-foreground">{current}</span>
      </p>
      <div className="mt-3 flex flex-col gap-2">
        <Select value={value} onValueChange={assignStore} disabled={reassign.isPending}>
          <SelectTrigger size="sm">
            <SelectValue placeholder={t("reassignPlaceholder")} />
          </SelectTrigger>
          <SelectContent>
            {(stores ?? []).map((s) => (
              <SelectItem key={s.store_no} value={String(s.store_no)}>
                {s.store_name} · {s.province}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {!app.talent_pool && (
          <Button size="sm" variant="outline" onClick={moveToPool} disabled={reassign.isPending}>
            {t("reassignToPool")}
          </Button>
        )}
      </div>
    </section>
  );
}
