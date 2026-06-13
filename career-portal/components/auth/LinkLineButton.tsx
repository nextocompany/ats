"use client";

import { Button } from "@/components/ui/button";
import { lineLoginUrl } from "@/lib/auth";

// LinkLineButton lets an email/Google account attach a LINE identity so status /
// interview push notifications can reach them. It navigates (top-level) to the
// backend LINE entrypoint in link mode, returning to the current page.
export function LinkLineButton() {
  function linkLine() {
    const returnUrl = window.location.origin + window.location.pathname;
    window.location.href = lineLoginUrl(returnUrl, "link");
  }

  return (
    <Button
      type="button"
      size="tap"
      variant="outline"
      onClick={linkLine}
      className="w-full border-[oklch(64%_0.16_150)]/40 text-[oklch(45%_0.16_150)]"
    >
      <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" className="mr-1">
        <path d="M12 2C6.48 2 2 5.69 2 10.25c0 4.08 3.58 7.5 8.41 8.14.33.07.77.22.88.5.1.26.07.66.03.92l-.14.86c-.04.26-.2 1.02.9.56 1.1-.46 5.91-3.48 8.06-5.96C21.6 13.6 22 11.98 22 10.25 22 5.69 17.52 2 12 2Z" />
      </svg>
      เชื่อมบัญชี LINE (รับการแจ้งเตือน)
    </Button>
  );
}
