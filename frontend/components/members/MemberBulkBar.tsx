"use client";

import { useState } from "react";
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
  const bulk = useMemberBulk();
  const [tag, setTag] = useState("");
  if (selected.length === 0) return null;

  const run = (action: string, value: string | undefined, label: string) => {
    bulk.mutate(
      { ids: selected, action, value },
      {
        onSuccess: (res) => {
          toast.success(`${label}: ${res.updated} สำเร็จ${res.failed ? ` · ${res.failed} ล้มเหลว` : ""}`);
          onDone();
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : "ดำเนินการไม่สำเร็จ"),
      },
    );
  };

  const applyTag = () => {
    const t = tag.trim();
    if (!t) return;
    run("tag", t, `ติดแท็ก "${t}"`);
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
        เลือก
      </span>
      <span className="mx-1 h-5 w-px bg-sidebar-border" />
      <span className="flex items-center gap-1.5">
        <Input
          value={tag}
          onChange={(e) => setTag(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") applyTag();
          }}
          placeholder="แท็ก…"
          maxLength={50}
          className="h-7 w-24 bg-background/90 text-foreground"
        />
        <Button size="sm" variant="secondary" disabled={bulk.isPending || !tag.trim()} onClick={applyTag}>
          ติดแท็ก
        </Button>
      </span>
      <span className="mx-1 h-5 w-px bg-sidebar-border" />
      <Button size="sm" variant="secondary" disabled={bulk.isPending} onClick={() => run("reactivate", undefined, "เปิดใช้งาน")}>
        เปิดใช้งาน
      </Button>
      <Button size="sm" variant="destructive" disabled={bulk.isPending} onClick={() => run("suspend", undefined, "ระงับ")}>
        ระงับ
      </Button>
    </div>
  );
}
