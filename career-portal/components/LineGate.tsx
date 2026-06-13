"use client";

import { Button } from "@/components/ui/button";
import { lineLoginUrl } from "@/lib/line";

// LineGate is the LINE-authentication step shown BEFORE the apply form. Auth must
// happen first because the OAuth redirect navigates away from the page — doing it
// up front means no half-filled form (or selected resume file) is lost.
export function LineGate({ error }: { error: string | null }) {
  function login() {
    const returnUrl = window.location.origin + window.location.pathname;
    window.location.href = lineLoginUrl(returnUrl);
  }

  return (
    <div className="space-y-5 text-center">
      <div className="mx-auto grid size-16 place-content-center rounded-full bg-[oklch(64%_0.16_150)]/15 text-[oklch(50%_0.16_150)]">
        <svg width="30" height="30" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M12 2C6.48 2 2 5.69 2 10.25c0 4.08 3.58 7.5 8.41 8.14.33.07.77.22.88.5.1.26.07.66.03.92l-.14.86c-.04.26-.2 1.02.9.56 1.1-.46 5.91-3.48 8.06-5.96C21.6 13.6 22 11.98 22 10.25 22 5.69 17.52 2 12 2Z" />
        </svg>
      </div>
      <div className="space-y-2">
        <h1 className="text-xl font-semibold">ยืนยันตัวตนด้วย LINE</h1>
        <p className="text-sm text-muted-foreground">เข้าสู่ระบบด้วย LINE ก่อนเริ่มกรอกใบสมัคร</p>
      </div>
      <Button
        type="button"
        size="tap"
        onClick={login}
        className="w-full bg-[oklch(64%_0.16_150)] text-white hover:bg-[oklch(60%_0.16_150)]"
      >
        เข้าสู่ระบบด้วย LINE
      </Button>
      {error ? (
        <p role="alert" className="text-sm text-destructive">
          เชื่อมต่อ LINE ไม่สำเร็จ ({error}) — กรุณาลองใหม่อีกครั้ง
        </p>
      ) : null}
    </div>
  );
}
