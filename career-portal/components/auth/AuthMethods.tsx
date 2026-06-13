"use client";

import { Button } from "@/components/ui/button";
import { googleLoginUrl, lineLoginUrl } from "@/lib/auth";

interface AuthMethodsProps {
  // returnUrl is where the backend redirects after OAuth (the page to resume on).
  returnUrl: string;
  // onChooseEmail switches the parent to the email-OTP form.
  onChooseEmail: () => void;
}

// AuthMethods presents the three signup/login providers. LINE and Google are
// top-level navigations to the backend OAuth entrypoints; Email is handled inline.
export function AuthMethods({ returnUrl, onChooseEmail }: AuthMethodsProps) {
  return (
    <div className="space-y-3">
      <Button
        type="button"
        size="tap"
        onClick={() => {
          window.location.href = lineLoginUrl(returnUrl);
        }}
        className="w-full bg-[oklch(64%_0.16_150)] text-white hover:bg-[oklch(60%_0.16_150)]"
      >
        <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" className="mr-1">
          <path d="M12 2C6.48 2 2 5.69 2 10.25c0 4.08 3.58 7.5 8.41 8.14.33.07.77.22.88.5.1.26.07.66.03.92l-.14.86c-.04.26-.2 1.02.9.56 1.1-.46 5.91-3.48 8.06-5.96C21.6 13.6 22 11.98 22 10.25 22 5.69 17.52 2 12 2Z" />
        </svg>
        สมัคร/เข้าสู่ระบบด้วย LINE
      </Button>

      <Button
        type="button"
        size="tap"
        variant="outline"
        onClick={() => {
          window.location.href = googleLoginUrl(returnUrl);
        }}
        className="w-full"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true" className="mr-1">
          <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.27-4.74 3.27-8.1Z" />
          <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.65l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84A11 11 0 0 0 12 23Z" />
          <path fill="#FBBC05" d="M5.84 14.11a6.6 6.6 0 0 1 0-4.22V7.05H2.18a11 11 0 0 0 0 9.9l3.66-2.84Z" />
          <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1A11 11 0 0 0 2.18 7.05l3.66 2.84C6.71 7.3 9.14 5.38 12 5.38Z" />
        </svg>
        สมัคร/เข้าสู่ระบบด้วย Google
      </Button>

      <Button type="button" size="tap" variant="outline" onClick={onChooseEmail} className="w-full">
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true" className="mr-1">
          <path d="M4 6h16v12H4z" stroke="currentColor" strokeWidth="2" strokeLinejoin="round" />
          <path d="m4 7 8 6 8-6" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        ใช้อีเมล
      </Button>
    </div>
  );
}
