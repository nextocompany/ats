"use client";

// Candidate journey timeline on the application detail. Reuses the existing
// candidate-scoped timeline endpoint (via useTimeline on the application's
// candidate). Surfaces status changes, resume views, re-engagement, and consent as
// a compact reverse-chronological list. Hidden until at least one event exists.
import { useTimeline } from "@/lib/queries";
import type { TimelineEntry } from "@/lib/types";

const ACTION_LABEL: Record<string, string> = {
  status_change: "เปลี่ยนสถานะ",
  bulk_action: "เปลี่ยนสถานะ (กลุ่ม)",
  view_resume: "เปิดดูเรซูเม่",
  reengage: "ติดต่อกลับ",
  consent: "ให้ความยินยอม PDPA",
  retention_anonymize: "ลบข้อมูลตามนโยบาย",
};

const STATUS_LABEL: Record<string, string> = {
  pending: "รอประมวลผล",
  scored: "คัดกรองแล้ว",
  ai_interview: "AI สัมภาษณ์",
  ai_interviewed: "AI สัมภาษณ์เสร็จ",
  shortlisted: "เข้ารอบ",
  interview: "นัดสัมภาษณ์",
  interviewed: "สัมภาษณ์แล้ว",
  pending_approval: "รออนุมัติ",
  offer: "เสนองาน",
  hired: "รับเข้าทำงาน",
  rejected: "ไม่ผ่าน",
};

function statusText(s?: string): string {
  if (!s) return "";
  return STATUS_LABEL[s] ?? s;
}

function detail(e: TimelineEntry): string | null {
  if ((e.action === "status_change" || e.action === "bulk_action") && e.new_value) {
    const to = statusText(e.new_value.to ?? e.new_value.status);
    const from = statusText(e.new_value.from);
    if (from && to) return `${from} → ${to}`;
    if (to) return to;
  }
  return null;
}

export function JourneyPanel({ candidateId }: { candidateId: string }) {
  const { data, isLoading } = useTimeline(candidateId);

  if (isLoading) return null;
  if (!data || data.length === 0) return null;

  return (
    <section className="mt-6 border-t border-hairline pt-5">
      <p className="eyebrow">ไทม์ไลน์ผู้สมัคร</p>
      <ol className="mt-3 flex flex-col gap-3">
        {data.map((e, i) => {
          const d = detail(e);
          return (
            <li key={`${e.created_at}-${i}`} className="flex gap-3">
              <span aria-hidden="true" className="mt-1.5 size-2 shrink-0 rounded-full bg-brand/60" />
              <div className="min-w-0">
                <p className="text-sm text-foreground">
                  {ACTION_LABEL[e.action] ?? e.action}
                  {d && <span className="text-muted-foreground"> · {d}</span>}
                </p>
                <p className="text-xs text-muted-foreground">{new Date(e.created_at).toLocaleString("th-TH")}</p>
              </div>
            </li>
          );
        })}
      </ol>
    </section>
  );
}
