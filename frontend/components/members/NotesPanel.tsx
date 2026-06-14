"use client";

import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAddNote, useMemberNotes } from "@/lib/queries";

function when(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleString("th-TH", { dateStyle: "medium", timeStyle: "short" });
}

export function NotesPanel({ memberId }: { memberId: string }) {
  const { data: notes, isLoading } = useMemberNotes(memberId);
  const add = useAddNote(memberId);
  const [body, setBody] = useState("");

  const submit = () => {
    const text = body.trim();
    if (!text) return;
    add.mutate(text, {
      onSuccess: () => {
        setBody("");
        toast.success("เพิ่มโน้ตแล้ว");
      },
      onError: (e) => toast.error(e instanceof Error ? e.message : "บันทึกโน้ตไม่สำเร็จ"),
    });
  };

  return (
    <section className="rounded-xl bg-card p-5 ring-1 ring-hairline">
      <h2 className="eyebrow mb-3">โน้ต (เฉพาะ HR)</h2>

      <div className="flex flex-col gap-2">
        <textarea
          value={body}
          onChange={(e) => setBody(e.target.value)}
          placeholder="บันทึกข้อสังเกตเกี่ยวกับสมาชิกรายนี้…"
          rows={3}
          maxLength={2000}
          className="w-full resize-y rounded-lg border border-hairline bg-background px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring"
        />
        <div className="flex justify-end">
          <Button size="sm" onClick={submit} disabled={add.isPending || !body.trim()}>
            เพิ่มโน้ต
          </Button>
        </div>
      </div>

      <div className="mt-4 space-y-3">
        {isLoading && <Skeleton className="h-16 w-full rounded-lg" />}
        {!isLoading && (notes?.length ?? 0) === 0 && (
          <p className="text-sm text-muted-foreground">ยังไม่มีโน้ต</p>
        )}
        {notes?.map((n) => (
          <article key={n.id} className="border-b border-hairline pb-3 last:border-0 last:pb-0">
            <p className="whitespace-pre-wrap text-sm text-foreground">{n.body}</p>
            <p className="mt-1 text-[0.6875rem] text-muted-foreground">
              {n.author_email || "ระบบ"} · {when(n.created_at)}
            </p>
          </article>
        ))}
      </div>
    </section>
  );
}
