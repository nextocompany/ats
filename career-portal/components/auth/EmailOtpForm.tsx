"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { startEmailOtp, verifyEmailOtp } from "@/lib/auth";
import { useCandidate } from "@/lib/session";

interface EmailOtpFormProps {
  // onVerified fires after a successful login (session cookie set + session refreshed).
  onVerified: () => void;
  onBack: () => void;
}

// EmailOtpForm runs the passwordless flow: enter email → receive a 6-digit code →
// verify. The session cookie is set by the verify response; we refresh the session
// before handing control back to the parent.
export function EmailOtpForm({ onVerified, onBack }: EmailOtpFormProps) {
  const { refresh } = useCandidate();
  const [stage, setStage] = useState<"email" | "code">("email");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function sendCode() {
    setError(null);
    if (!/.+@.+\..+/.test(email)) {
      setError("กรุณากรอกอีเมลให้ถูกต้อง");
      return;
    }
    setBusy(true);
    try {
      await startEmailOtp(email.trim());
      setStage("code");
    } catch {
      setError("ส่งรหัสไม่สำเร็จ กรุณาลองใหม่");
    } finally {
      setBusy(false);
    }
  }

  async function verify() {
    setError(null);
    if (code.trim().length < 4) {
      setError("กรุณากรอกรหัสที่ได้รับทางอีเมล");
      return;
    }
    setBusy(true);
    try {
      await verifyEmailOtp(email.trim(), code.trim());
      await refresh();
      onVerified();
    } catch {
      setError("รหัสไม่ถูกต้องหรือหมดอายุ");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      {stage === "email" ? (
        <>
          <div className="space-y-2">
            <Label htmlFor="otp-email">อีเมล</Label>
            <Input
              id="otp-email"
              type="email"
              inputMode="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
            />
          </div>
          {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}
          <div className="flex gap-3">
            <Button type="button" size="tap" variant="outline" onClick={onBack} className="flex-1">
              ย้อนกลับ
            </Button>
            <Button type="button" size="tap" onClick={sendCode} disabled={busy} className="flex-1">
              {busy ? "กำลังส่ง…" : "ส่งรหัส"}
            </Button>
          </div>
        </>
      ) : (
        <>
          <div className="space-y-2">
            <Label htmlFor="otp-code">รหัสยืนยัน</Label>
            <Input
              id="otp-code"
              inputMode="numeric"
              autoComplete="one-time-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="------"
              className="text-center tracking-[0.5em]"
            />
            <p className="text-xs text-muted-foreground">ส่งรหัสไปที่ {email} แล้ว</p>
          </div>
          {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}
          <div className="flex gap-3">
            <Button type="button" size="tap" variant="outline" onClick={() => setStage("email")} className="flex-1">
              เปลี่ยนอีเมล
            </Button>
            <Button type="button" size="tap" onClick={verify} disabled={busy} className="flex-1">
              {busy ? "กำลังตรวจสอบ…" : "ยืนยัน"}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
