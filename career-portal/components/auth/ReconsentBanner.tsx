"use client";

// Phase 2 reconsent prompt: shown to a consenting member when a newer privacy
// notice version is current than the one they accepted (/me → pdpa_needs_reconsent).
// They review the updated notice and re-accept; accepting records a fresh consent
// ledger row at the current version, after which the banner clears.
import { useState } from "react";
import Link from "next/link";

import { Button } from "@/components/ui/button";
import { reacceptConsent } from "@/lib/auth";
import { ApiError } from "@/lib/api";
import { useCandidate } from "@/lib/session";

export function ReconsentBanner() {
  const { candidate, refresh } = useCandidate();
  const [accepting, setAccepting] = useState(false);
  const [err, setErr] = useState("");

  if (!candidate?.pdpa_needs_reconsent) return null;

  async function onAccept() {
    setAccepting(true);
    setErr("");
    try {
      await reacceptConsent();
      await refresh();
      // refresh() only schedules the /me refetch; reset the spinner so the button
      // is not stuck during the 1-2 RTT before the banner clears on re-render.
      setAccepting(false);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "เกิดข้อผิดพลาด กรุณาลองใหม่อีกครั้ง");
      setAccepting(false);
    }
  }

  return (
    <section
      aria-labelledby="reconsent-heading"
      className="flex flex-col gap-3 rounded-xl border border-line border-l-4 border-l-foreground bg-secondary p-5"
    >
      <div className="flex flex-col gap-1">
        <p id="reconsent-heading" className="text-sm font-semibold text-foreground">
          ประกาศความเป็นส่วนตัวมีการอัปเดต
        </p>
        <p className="text-sm text-muted-foreground">
          เราได้ปรับปรุงประกาศการคุ้มครองข้อมูลส่วนบุคคล กรุณาตรวจสอบและให้ความยินยอมในเวอร์ชันล่าสุดเพื่อใช้งานบัญชีต่อ
        </p>
      </div>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" size="sm" onClick={onAccept} disabled={accepting}>
          {accepting ? "กำลังบันทึก…" : "ยอมรับเวอร์ชันล่าสุด"}
        </Button>
        <Link
          href="/privacy"
          className="text-sm font-medium text-foreground underline underline-offset-4 transition-colors hover:text-muted-foreground"
        >
          อ่านประกาศความเป็นส่วนตัว
        </Link>
      </div>
      {err ? (
        <p role="status" aria-live="polite" className="text-sm text-destructive">
          {err}
        </p>
      ) : null}
    </section>
  );
}
