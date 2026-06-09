"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { getIdToken } from "@/lib/line";

interface LineLoginButtonProps {
  // onToken fires with the resolved LINE id-token (dev stub today, LIFF later).
  onToken: (token: string) => void;
  connected: boolean;
}

// LineLoginButton resolves a LINE id-token via the lib/line seam. In dev this is
// an instant stub; in production it becomes the LIFF login. Once connected it
// shows a confirmed state so the candidate knows they can submit.
export function LineLoginButton({ onToken, connected }: LineLoginButtonProps) {
  const [loading, setLoading] = useState(false);

  async function handleLogin() {
    setLoading(true);
    try {
      const token = await getIdToken();
      onToken(token);
    } finally {
      setLoading(false);
    }
  }

  if (connected) {
    return (
      <div
        className="flex items-center justify-center gap-2 rounded-xl bg-brand-soft px-4 py-3 text-sm font-medium text-accent"
        role="status"
      >
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
          <path d="M20 6L9 17l-5-5" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        เชื่อมต่อ LINE แล้ว
      </div>
    );
  }

  return (
    <Button
      type="button"
      size="tap"
      onClick={handleLogin}
      disabled={loading}
      className="w-full bg-[oklch(64%_0.16_150)] text-white hover:bg-[oklch(60%_0.16_150)]"
    >
      {loading ? "กำลังเชื่อมต่อ…" : "เข้าสู่ระบบด้วย LINE"}
    </Button>
  );
}
