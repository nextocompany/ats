"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";

import { AuthMethods } from "@/components/auth/AuthMethods";
import { EmailOtpForm } from "@/components/auth/EmailOtpForm";
import { ProfileForm } from "@/components/auth/ProfileForm";
import { ResumeUploadStep } from "@/components/auth/ResumeUploadStep";
import { PortalShell } from "@/components/PortalShell";
import { useCandidate } from "@/lib/session";

// Signup is account-first and auth-aware. Before login it shows the method
// chooser; after login (incl. returning from LINE/Google OAuth) it continues the
// one-time setup: profile → resume. A fully set-up account is redirected on.
export default function SignupPage() {
  return (
    <PortalShell backHref="/jobs" narrow>
      <Suspense fallback={<p className="text-center text-sm text-muted-foreground">กำลังโหลด…</p>}>
        <SignupFlow />
      </Suspense>
    </PortalShell>
  );
}

function SignupFlow() {
  const router = useRouter();
  const params = useSearchParams();
  const returnTo = params.get("return") || "/jobs";

  const { candidate, isAuthenticated, isLoading } = useCandidate();
  const [emailMode, setEmailMode] = useState(false);
  const [setupStep, setSetupStep] = useState<"profile" | "resume">("profile");

  // Once authenticated with a saved resume, setup is complete.
  useEffect(() => {
    if (isAuthenticated && candidate?.has_resume) router.replace(returnTo);
  }, [isAuthenticated, candidate?.has_resume, returnTo, router]);

  if (isLoading) {
    return <p className="text-center text-sm text-muted-foreground">กำลังโหลด…</p>;
  }

  const oauthReturn =
    typeof window !== "undefined"
      ? `${window.location.origin}/signup?return=${encodeURIComponent(returnTo)}`
      : "/signup";

  if (!isAuthenticated || !candidate) {
    return (
      <div className="space-y-6">
        <header className="space-y-2 text-center">
          <h1 className="text-xl font-semibold">สมัครสมาชิก</h1>
          <p className="text-sm text-muted-foreground">สมัครครั้งเดียว ใช้สมัครงานได้รวดเร็วทุกตำแหน่ง</p>
        </header>
        {emailMode ? (
          <EmailOtpForm onVerified={() => setEmailMode(false)} onBack={() => setEmailMode(false)} />
        ) : (
          <AuthMethods returnUrl={oauthReturn} onChooseEmail={() => setEmailMode(true)} />
        )}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <p className="text-sm text-muted-foreground">ขั้นตอนที่ {setupStep === "profile" ? "1" : "2"} จาก 2</p>
        <h1 className="text-xl font-semibold">{setupStep === "profile" ? "กรอกข้อมูลเบื้องต้น" : "อัปโหลดเรซูเม่"}</h1>
      </header>
      {setupStep === "profile" ? (
        <ProfileForm account={candidate} requireConsent submitLabel="ถัดไป" onSaved={() => setSetupStep("resume")} />
      ) : (
        <ResumeUploadStep account={candidate} submitLabel="เสร็จสิ้นการสมัคร" onUploaded={() => router.replace(returnTo)} />
      )}
    </div>
  );
}
