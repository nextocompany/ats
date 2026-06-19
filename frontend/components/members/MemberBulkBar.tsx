"use client";

import { useState } from "react";
import { useTranslations } from "next-intl";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useMemberBulk } from "@/lib/queries";

interface MemberBulkBarProps {
  selected: string[];
  onDone: () => void;
}

// MemberBulkBar applies a bulk action (tag/suspend/reactivate) to the selected
// members. Mirrors the applications BulkActionBar but with member actions; the
// irreversible erase is deliberately NOT here (single, super_admin-only, confirmed).
export function MemberBulkBar({ selected, onDone }: MemberBulkBarProps) {
  const t = useTranslations("members");
  const bulk = useMemberBulk();
  const [tag, setTag] = useState("");
  if (selected.length === 0) return null;

  const run = (action: string, value: string | undefined, label: string) => {
    bulk.mutate(
      { ids: selected, action, value },
      {
        onSuccess: (res) => {
          const base = t("bulkOk", { label, updated: res.updated });
          toast.success(res.failed ? base + t("bulkFailSuffix", { failed: res.failed }) : base);
          onDone();
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : t("actionFailed")),
      },
    );
  };

  const applyTag = () => {
    const value = tag.trim();
    if (!value) return;
    run("tag", value, t("bulkTagLabel", { tag: value }));
    setTag("");
  };

  return (
    <div
      role="region"
      aria-label="Bulk member actions"
      className="settle sticky bottom-5 z-10 mx-auto flex w-fit flex-wrap items-center gap-2.5 rounded-xl bg-sidebar px-4 py-2.5 text-sidebar-foreground shadow-2xl ring-1 ring-black/20"
    >
      <span className="flex items-center gap-2 text-sm font-medium">
        <span className="grid size-5 place-items-center rounded-full bg-sidebar-primary text-[0.6875rem] font-semibold text-sidebar-primary-foreground tabular-nums">
          {selected.length}
        </span>
        {t("bulkSelected")}
      </span>
      <span className="mx-1 h-5 w-px bg-sidebar-border" />
      <span className="flex items-center gap-1.5">
        <Input
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") applyTag();
          }}
          placeholder={t("bulkTagPlaceholder")}
          maxLength={50}
          className="h-7 w-24 bg-background/90 text-foreground"
        />
        <Button size="sm" variant="secondary" disabled={bulk.isPending || !tag.trim()} onClick={applyTag}>
          {t("bulkTag")}
        </Button>
      </span>
      <span className="mx-1 h-5 w-px bg-sidebar-border" />
      <Button size="sm" variant="secondary" disabled={bulk.isPending} onClick={() => run("reactivate", undefined, t("bulkReactivate"))}>
        {t("bulkReactivate")}
      </Button>
      <Button size="sm" variant="destructive" disabled={bulk.isPending} onClick={() => run("suspend", undefined, t("bulkSuspend"))}>
        {t("bulkSuspend")}
      </Button>
    </div>
  );
}
