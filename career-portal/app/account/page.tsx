"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import { LinkLineButton } from "@/components/auth/LinkLineButton";
import { ProfileForm } from "@/components/auth/ProfileForm";
import { ResumeUploadStep } from "@/components/auth/ResumeUploadStep";
import { OnboardingSection } from "@/components/onboarding/OnboardingSection";
import { PortalShell } from "@/components/PortalShell";
import { Button } from "@/components/ui/button";
import { logout } from "@/lib/auth";
import { useCandidate } from "@/lib/session";

// Account is the member's self-service page: edit profile, replace the saved
// resume, link LINE (email/Google accounts), and log out. Session-gated client-side.
export default function AccountPage() {
  const router = useRouter();
  const { candidate, isAuthenticated, isLoading, refresh } = useCandidate();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.replace("/login?return=/account");
  }, [isLoading, isAuthenticated, router]);

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

  return (
    <PortalShell backHref="/jobs" narrow>
      <div className="flex flex-col gap-10">
        <header className="flex items-start justify-between gap-3 border-b border-line pb-6">
          <div className="flex flex-col gap-1.5">
            <h1 className="[font-size:var(--text-h2)] font-semibold leading-tight text-foreground">บัญชีของฉัน</h1>
            <p className="text-sm text-muted-foreground">{candidate.email || candidate.line_display_id || "สมาชิก"}</p>
          </div>
          <Button type="button" size="sm" variant="ghost" onClick={doLogout}>
            ออกจากระบบ
          </Button>
        </header>

        <section className="flex flex-col gap-4">
          <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-foreground">ข้อมูลส่วนตัว</h2>
          <div className="rounded-xl border border-line bg-card p-6">
            <ProfileForm account={candidate} submitLabel="บันทึกข้อมูล" onSaved={() => undefined} />
          </div>
        </section>

        <section className="flex flex-col gap-4">
          <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-foreground">การแจ้งเตือนผ่าน LINE</h2>
          {!candidate.line_linked ? (
            <div className="flex flex-col gap-3 rounded-xl border border-line bg-card p-6">
              <p className="text-sm text-muted-foreground">เชื่อมบัญชี LINE เพื่อรับการแจ้งเตือนสถานะใบสมัคร</p>
              <LinkLineButton />
            </div>
          ) : (
            <div className="rounded-xl border border-line bg-card p-6">
              <p className="text-sm text-[oklch(45%_0.14_150)]">เชื่อมบัญชี LINE แล้ว</p>
            </div>
          )}
        </section>

        <section className="flex flex-col gap-4">
          <h2 className="text-xs font-semibold uppercase tracking-[0.14em] text-foreground">เรซูเม่</h2>
          <div className="rounded-xl border border-line bg-card p-6">
            <ResumeUploadStep account={candidate} submitLabel="อัปเดตเรซูเม่" onUploaded={() => undefined} />
          </div>
        </section>

        <OnboardingSection />
      </div>
    </PortalShell>
  );
}
