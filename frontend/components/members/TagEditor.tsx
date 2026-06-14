"use client";

import { useState } from "react";
import { X } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useAddTag, useMemberTags, useRemoveTag } from "@/lib/queries";

export function TagEditor({ memberId }: { memberId: string }) {
  const { data: tags, isLoading } = useMemberTags(memberId);
  const add = useAddTag(memberId);
  const remove = useRemoveTag(memberId);
  const [value, setValue] = useState("");

  const submit = () => {
    const tag = value.trim();
    if (!tag) return;
    add.mutate(tag, {
      onSuccess: () => setValue(""),
      onError: (e) => toast.error(e instanceof Error ? e.message : "เพิ่มแท็กไม่สำเร็จ"),
    });
  };

  return (
    <div className="rounded-xl bg-card p-5 ring-1 ring-hairline">
      <h2 className="eyebrow mb-3">แท็ก</h2>

      {isLoading ? (
        <Skeleton className="h-6 w-32 rounded-full" />
      ) : (
        <div className="flex flex-wrap gap-1.5">
          {(tags?.length ?? 0) === 0 && <span className="text-sm text-muted-foreground">ยังไม่มีแท็ก</span>}
          {tags?.map((t) => (
            <span
              key={t}
              className="inline-flex items-center gap-1 rounded-full bg-brand-soft px-2 py-0.5 text-xs font-medium text-brand"
            >
              {t}
              <button
                type="button"
                aria-label={`ลบแท็ก ${t}`}
                className="rounded-full p-0.5 hover:bg-brand/15 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() =>
                  remove.mutate(t, {
                    onError: (e) => toast.error(e instanceof Error ? e.message : "ลบแท็กไม่สำเร็จ"),
                  })
                }
              >
                <X className="size-3" />
              </button>
            </span>
          ))}
        </div>
      )}

      <div className="mt-3 flex gap-2">
        <Input
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              submit();
            }
          }}
          placeholder="เพิ่มแท็ก…"
          maxLength={50}
          className="h-8"
        />
        <Button size="sm" variant="outline" onClick={submit} disabled={add.isPending || !value.trim()}>
          เพิ่ม
        </Button>
      </div>
    </div>
  );
}
