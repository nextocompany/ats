"use client";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";

// The `beforeinstallprompt` event isn't in the standard DOM lib. Only Chromium
// browsers (Android / desktop Chrome) fire it; iOS Safari never does, so the
// affordance simply stays hidden there and users rely on "Add to Home Screen".
interface BeforeInstallPromptEvent extends Event {
  readonly platforms: string[];
  readonly userChoice: Promise<{ outcome: "accepted" | "dismissed"; platform: string }>;
  prompt: () => Promise<void>;
}

const DISMISS_KEY = "pwa-install-dismissed";

// InstallPrompt offers a dismissible "Add to Home Screen" affordance. It renders
// nothing until the browser signals installability (beforeinstallprompt), is
// hidden once installed or already running standalone, and remembers a dismissal
// in localStorage so it doesn't nag on every visit.
export function InstallPrompt() {
  const [deferred, setDeferred] = useState<BeforeInstallPromptEvent | null>(null);
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    // Already installed (standalone) → never wire the prompt up.
    const standalone =
      window.matchMedia("(display-mode: standalone)").matches ||
      // iOS Safari exposes navigator.standalone instead of display-mode.
      (window.navigator as Navigator & { standalone?: boolean }).standalone === true;
    if (standalone) return;

    function onBeforeInstall(event: Event) {
      // Stop Chrome's mini-infobar; we present our own branded button instead.
      event.preventDefault();
      // Respect a prior dismissal — checked here (not in an effect body) so the
      // prompt simply never surfaces for users who waved it away.
      if (localStorage.getItem(DISMISS_KEY) === "1") return;
      setDeferred(event as BeforeInstallPromptEvent);
    }
    function onInstalled() {
      setDeferred(null);
    }

    window.addEventListener("beforeinstallprompt", onBeforeInstall);
    window.addEventListener("appinstalled", onInstalled);
    return () => {
      window.removeEventListener("beforeinstallprompt", onBeforeInstall);
      window.removeEventListener("appinstalled", onInstalled);
    };
  }, []);

  if (!deferred || dismissed) return null;

  async function handleInstall() {
    if (!deferred) return;
    await deferred.prompt();
    await deferred.userChoice;
    // The event can be used only once; drop it regardless of the outcome.
    setDeferred(null);
  }

  function handleDismiss() {
    localStorage.setItem(DISMISS_KEY, "1");
    setDismissed(true);
  }

  return (
    <div
      className="flex items-center gap-3 rounded-2xl bg-brand-soft px-4 py-3 ring-1 ring-primary/15"
      role="region"
      aria-label="ติดตั้งแอป"
    >
      <span className="grid size-9 shrink-0 place-content-center rounded-xl bg-primary text-sm font-bold text-primary-foreground" aria-hidden="true">
        N
      </span>
      <p className="min-w-0 flex-1 text-sm text-foreground/80">เพิ่มแอปลงหน้าจอหลักเพื่อเปิดได้เร็วขึ้น</p>
      <div className="flex shrink-0 items-center gap-1">
        <Button type="button" size="sm" onClick={handleInstall}>
          เพิ่มลงหน้าจอหลัก
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          onClick={handleDismiss}
          aria-label="ปิด"
          className="text-muted-foreground"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path d="M18 6L6 18M6 6l12 12" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
          </svg>
        </Button>
      </div>
    </div>
  );
}
