"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { LogOut } from "lucide-react";

import { AccountIdentityHeader, type AccountSummaryFigure } from "@/components/account/AccountIdentityHeader";
import { AccountSection } from "@/components/account/AccountSection";
import { ApplicationHistorySection } from "@/components/account/ApplicationHistorySection";
import { DataRightsSection } from "@/components/auth/DataRightsSection";
import { ReconsentBanner } from "@/components/auth/ReconsentBanner";
import { LinkLineButton } from "@/components/auth/LinkLineButton";
import { ProfileForm } from "@/components/auth/ProfileForm";
import { ResumeLibrary } from "@/components/auth/ResumeLibrary";
import { OnboardingSection } from "@/components/onboarding/OnboardingSection";
import { PortalShell } from "@/components/PortalShell";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { logout, RESUME_LIMIT } from "@/lib/auth";
import { useCandidate } from "@/lib/session";
import type { Account } from "@/lib/types";

// profileCompleteness scores the five fields that make a candidate ready to apply,
// so the identity summary can show an at-a-glance percentage. Resume readiness uses
// the stable /me has_resume flag (not the async count) so the figure does not flicker.
function profileCompleteness(account: Account): number {
  const filled = [
    account.full_name.trim().length > 0,
    account.phone.trim().length > 0,
    account.province.trim().length > 0,
    account.line_linked,
    account.has_resume,
  ].filter(Boolean).length;
  return Math.round((filled / 5) * 100);
}

// Account is the member's self-service page: an identity header with an at-a-glance
// summary, an editorial bento (profile + resume library beside a LINE/onboarding
// rail), the PDPA data-rights surface, and logout. Session-gated client-side.
export default function AccountPage() {
  const router = useRouter();
  const { candidate, isAuthenticated, isLoading, refresh } = useCandidate();
  // Live resume count for the summary figure — reported up by ResumeLibrary so it
  // stays in sync as the member uploads/deletes (null until first load).
  const [resumeCount, setResumeCount] = useState<number | null>(null);
  // LINE-link result is returned by the OAuth callback as a URL fragment; surface
  // it as a one-time banner, then strip the hash so a refresh doesn't repeat it.
  const [lineNotice, setLineNotice] = useState<{ tone: "ok" | "error"; text: string } | null>(null);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.replace("/login?return=/account");
  }, [isLoading, isAuthenticated, router]);

  useEffect(() => {
    const hash = window.location.hash;
    if (!hash || !hash.includes("line_")) return;
    let notice: { tone: "ok" | "error"; text: string } | null = null;
    if (hash.includes("line_linked=1")) {
      notice = { tone: "ok", text: "เชื่อมบัญชี LINE เรียบร้อยแล้ว" };
      void refresh();
    } else if (hash.includes("line_error=line_in_use")) {
      notice = {
        tone: "error",
        text: "LINE นี้ถูกผูกกับอีกบัญชีอยู่แล้ว หากเป็นบัญชีของคุณ กรุณาเข้าสู่ระบบด้วย LINE นั้น หรือติดต่อทีมงานหากต้องการรวมบัญชี",
      };
    } else if (hash.includes("line_error=not_logged_in")) {
      notice = { tone: "error", text: "กรุณาเข้าสู่ระบบก่อนเชื่อมบัญชี LINE" };
    } else if (hash.includes("line_error")) {
      notice = { tone: "error", text: "เชื่อมบัญชี LINE ไม่สำเร็จ กรุณาลองใหม่อีกครั้ง" };
    }
    if (notice) {
      // Reading the OAuth-callback URL fragment is a client-only external source;
      // a lazy useState initializer can't be used (window is undefined during SSR).
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setLineNotice(notice);
      window.history.replaceState(null, "", window.location.pathname + window.location.search);
    }
  }, [refresh]);

  const summary = useMemo<AccountSummaryFigure[]>(() => {
    if (!candidate) return [];
    return [
      { value: `${profileCompleteness(candidate)}%`, numeric: true, label: "ความสมบูรณ์ของโปรไฟล์" },
      { value: `${resumeCount ?? "—"} / ${RESUME_LIMIT}`, numeric: true, label: "เรซูเม่ที่บันทึกไว้" },
      { value: candidate.line_linked ? "เชื่อมแล้ว" : "ยังไม่เชื่อม", label: "การแจ้งเตือน LINE" },
      { value: candidate.province || "—", label: "จังหวัด" },
    ];
  }, [candidate, resumeCount]);

  if (isLoading || !candidate) {
    return (
      <PortalShell backHref="/jobs" narrow>
        <p className="text-center text-sm text-muted-foreground">กำลังโหลด…</p>
      </PortalShell>
    );
  }

  async function doLogout() {
    await logout();
    await refresh();
    router.replace("/jobs");
  }

  // After a self-erase the account + session are already gone server-side. Best-
  // effort clear the cookie (the logout POST may 401 now - ignore it) and navigate
  // away. No refresh(): re-fetching /me here would 401 and race the redirect.
  async function onErased() {
    try {
      await logout();
    } catch {
      // session already invalid server-side; nothing to clean up
    }
    router.replace("/jobs?erased=1");
  }

  const contact = candidate.email || candidate.line_display_id || "สมาชิก";
  const secondary = candidate.email && candidate.line_display_id ? `LINE ${candidate.line_display_id}` : undefined;

  return (
    <PortalShell backHref="/jobs">
      <div className="reveal flex flex-col gap-12 lg:gap-14">
        <AccountIdentityHeader
          name={candidate.full_name || contact}
          contact={contact}
          secondary={secondary}
          summary={summary}
        />

        <ReconsentBanner />

        {lineNotice ? (
          <div
            role="status"
            className={`rounded-xl border px-4 py-3 text-sm ${
              lineNotice.tone === "ok"
                ? "border-accent/30 bg-accent-soft text-primary"
                : "border-destructive/30 bg-destructive/10 text-destructive"
            }`}
          >
            {lineNotice.text}
          </div>
        ) : null}

        {/* Editorial bento at >=1024: a dominant lead column (profile + resume)
            beside a quieter supporting rail (LINE + onboarding) on a faint muted
            surface for layered hierarchy; stacks to one column on mobile. */}
        <div className="grid grid-cols-1 gap-x-10 gap-y-12 lg:grid-cols-12">
          <div className="flex flex-col gap-12 lg:col-span-8">
            <AccountSection
              eyebrow="โปรไฟล์"
              title="ข้อมูลส่วนตัว"
              lead="ข้อมูลนี้จะถูกใช้ในการสมัครงานและการติดต่อกลับ"
            >
              <ProfileForm account={candidate} submitLabel="บันทึกข้อมูล" onSaved={() => undefined} />
            </AccountSection>

            <ApplicationHistorySection />

            <AccountSection
              eyebrow="เรซูเม่"
              title="คลังเรซูเม่"
              lead={`เก็บได้สูงสุด ${RESUME_LIMIT} ไฟล์ - ไฟล์เริ่มต้นจะถูกใช้ตอนสมัครงาน`}
              action={
                resumeCount !== null ? (
                  <span className="num text-sm font-semibold tabular-nums text-muted-foreground">
                    {resumeCount} / {RESUME_LIMIT} ไฟล์
                  </span>
                ) : undefined
              }
            >
              <ResumeLibrary hideHeading onCountChange={setResumeCount} />
            </AccountSection>
          </div>

          <aside className="flex flex-col gap-8 rounded-2xl border border-line bg-surface-muted p-5 sm:p-6 lg:col-span-4">
            <AccountSection eyebrow="การแจ้งเตือน" title="LINE">
              {candidate.line_linked ? (
                <p className="text-sm font-medium text-[oklch(45%_0.14_150)]">เชื่อมบัญชี LINE แล้ว</p>
              ) : (
                <div className="flex flex-col gap-3">
                  <p className="text-sm text-muted-foreground">เชื่อมบัญชี LINE เพื่อรับการแจ้งเตือนสถานะใบสมัคร</p>
                  <LinkLineButton />
                </div>
              )}
            </AccountSection>

            {/* Self-gates: renders nothing unless the member has a hired application. */}
            <OnboardingSection />
          </aside>
        </div>

        {/* PDPA compliance surface — full-width, prominent, not buried. */}
        <DataRightsSection onErased={onErased} />

        {/* Account actions — logout. */}
        <Card className="border-line">
          <CardContent className="flex flex-col items-start justify-between gap-3 py-1 sm:flex-row sm:items-center">
            <div className="flex flex-col gap-0.5">
              <p className="text-sm font-medium text-foreground">ออกจากระบบ</p>
              <p className="text-sm text-muted-foreground">ออกจากบัญชีนี้บนอุปกรณ์ปัจจุบัน</p>
            </div>
            <Button type="button" variant="outline" className="h-11 w-full sm:w-auto" onClick={doLogout}>
              <LogOut className="size-4" aria-hidden="true" />
              ออกจากระบบ
            </Button>
          </CardContent>
        </Card>
      </div>
    </PortalShell>
  );
}
