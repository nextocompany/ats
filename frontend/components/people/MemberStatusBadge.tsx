import type { Member } from "@/lib/types";

const STATUS_MAP: Record<Member["status"], { label: string; tone: string }> = {
  active: { label: "ใช้งาน", tone: "var(--score-high)" },
  suspended: { label: "ระงับ", tone: "var(--score-mid)" },
  anonymized: { label: "ลบข้อมูลแล้ว", tone: "var(--muted-foreground)" },
};

// MemberStatusBadge renders a member's account status with a CP-Axtra status tone.
export function MemberStatusBadge({ status }: { status: Member["status"] }) {
  const m = STATUS_MAP[status] ?? STATUS_MAP.active;
  return (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-[0.6875rem] font-semibold text-white"
      style={{ backgroundColor: m.tone }}
    >
      {m.label}
    </span>
  );
}
