"use client";

import { useTranslations } from "next-intl";

import type { Member } from "@/lib/types";

const STATUS_MAP: Record<Member["status"], { labelKey: string; tone: string }> = {
  active: { labelKey: "badgeActive", tone: "var(--score-high)" },
  suspended: { labelKey: "badgeSuspended", tone: "var(--score-mid)" },
  anonymized: { labelKey: "badgeAnonymized", tone: "var(--muted-foreground)" },
};

// MemberStatusBadge renders a member's account status with a CP-Axtra status tone.
export function MemberStatusBadge({ status }: { status: Member["status"] }) {
  const t = useTranslations("members");
  const m = STATUS_MAP[status] ?? STATUS_MAP.active;
  return (
    <span
      className="inline-flex items-center rounded-full px-2 py-0.5 text-[0.6875rem] font-semibold text-white"
      style={{ backgroundColor: m.tone }}
    >
      {t(m.labelKey)}
    </span>
  );
}
