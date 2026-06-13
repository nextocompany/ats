"use client";

import { Suspense, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";

import { AuthMethods } from "@/components/auth/AuthMethods";
import { EmailOtpForm } from "@/components/auth/EmailOtpForm";
import { PortalShell } from "@/components/PortalShell";
import { useCandidate } from "@/lib/session";

// Login is the returning-user entrypoint. It honors ?return= so the apply flow
// can send unauthenticated users here and bounce them back after login.
export default function LoginPage() {
  return (
    <PortalShell backHref="/jobs" narrow>
      <Suspense fallback={<p className="text-center text-sm text-muted-foreground">กำลังโหลด…</p>}>
        <LoginFlow />
      </Suspense>
    </PortalShell>
  );
}

function LoginFlow() {
  const router = useRouter();
  const params = useSearchParams();
  const returnTo = params.get("return") || "/jobs";

  const { isAuthenticated, isLoading } = useCandidate();
  const [emailMode, setEmailMode] = useState(false);

  useEffect(() => {
    if (isAuthenticated) router.replace(returnTo);
  }, [isAuthenticated, returnTo, router]);

  if (isLoading) {
    return <p className="text-center text-sm text-muted-foreground">กำลังโหลด…</p>;
  }

  const oauthReturn =
    typeof window !== "undefined"
      ? `${window.location.origin}/login?return=${encodeURIComponent(returnTo)}`
      : "/login";

  return (
    <div className="space-y-6">
      <header className="space-y-2 text-center">
        <h1 className="text-xl font-semibold">เข้าสู่ระบบ</h1>
        <p className="text-sm text-muted-foreground">เข้าสู่ระบบเพื่อสมัครงานได้รวดเร็วด้วยข้อมูลที่บันทึกไว้</p>
      </header>
      {emailMode ? (
        <EmailOtpForm onVerified={() => router.replace(returnTo)} onBack={() => setEmailMode(false)} />
      ) : (
        <AuthMethods returnUrl={oauthReturn} onChooseEmail={() => setEmailMode(true)} />
      )}
      <p className="text-center text-sm text-muted-foreground">
        ยังไม่มีบัญชี?{" "}
        <Link href={`/signup?return=${encodeURIComponent(returnTo)}`} className="font-medium text-accent underline-offset-4 hover:underline">
          สมัครสมาชิก
        </Link>
      </p>
    </div>
  );
}
