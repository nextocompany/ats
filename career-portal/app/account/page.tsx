"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import { LinkLineButton } from "@/components/auth/LinkLineButton";
import { ProfileForm } from "@/components/auth/ProfileForm";
import { ResumeUploadStep } from "@/components/auth/ResumeUploadStep";
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
      <div className="space-y-8">
        <header className="flex items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold">บัญชีของฉัน</h1>
            <p className="text-sm text-muted-foreground">{candidate.email || candidate.line_display_id || "สมาชิก"}</p>
          </div>
          <Button type="button" size="sm" variant="ghost" onClick={doLogout}>
            ออกจากระบบ
          </Button>
        </header>

        <section className="space-y-4">
          <h2 className="text-sm font-semibold text-foreground/70">ข้อมูลส่วนตัว</h2>
          <ProfileForm account={candidate} submitLabel="บันทึกข้อมูล" onSaved={() => undefined} />
        </section>

        {!candidate.line_linked ? (
          <section className="space-y-3">
            <h2 className="text-sm font-semibold text-foreground/70">การแจ้งเตือนผ่าน LINE</h2>
            <p className="text-sm text-muted-foreground">เชื่อมบัญชี LINE เพื่อรับการแจ้งเตือนสถานะใบสมัคร</p>
            <LinkLineButton />
          </section>
        ) : (
          <section className="space-y-2">
            <h2 className="text-sm font-semibold text-foreground/70">การแจ้งเตือนผ่าน LINE</h2>
            <p className="text-sm text-[oklch(50%_0.16_150)]">เชื่อมบัญชี LINE แล้ว</p>
          </section>
        )}

        <section className="space-y-4">
          <h2 className="text-sm font-semibold text-foreground/70">เรซูเม่</h2>
          <ResumeUploadStep account={candidate} submitLabel="อัปเดตเรซูเม่" onUploaded={() => undefined} />
        </section>
      </div>
    </PortalShell>
  );
}
