"use client";

// Share a job to LINE / Facebook / native share sheet, or copy the link. Reads the
// canonical job URL from window.location at click time (no SSR/hydration mismatch).
import { useState } from "react";

interface Props {
  title: string;
}

function shareUrl(): string {
  if (typeof window === "undefined") return "";
  return window.location.href;
}

export function ShareButtons({ title }: Props) {
  const [copied, setCopied] = useState(false);

  function openShare(target: "line" | "facebook") {
    const url = encodeURIComponent(shareUrl());
    const href =
      target === "line"
        ? `https://social-plugins.line.me/lineit/share?url=${url}`
        : `https://www.facebook.com/sharer/sharer.php?u=${url}`;
    window.open(href, "_blank", "noopener,noreferrer,width=600,height=600");
  }

  async function nativeOrCopy() {
    const url = shareUrl();
    // Prefer the OS share sheet on mobile; fall back to clipboard.
    if (navigator.share) {
      try {
        await navigator.share({ title, url });
        return;
      } catch {
        // user cancelled or unsupported — fall through to copy
      }
    }
    try {
      await navigator.clipboard?.writeText(url);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard blocked — nothing more we can do silently
    }
  }

  const btn =
    "inline-flex items-center justify-center gap-2 rounded-lg border border-line bg-secondary px-3 py-2 text-sm font-medium text-foreground transition-colors hover:bg-secondary/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-muted-foreground">แชร์ตำแหน่งนี้</p>
      <div className="grid grid-cols-3 gap-2">
        <button type="button" onClick={() => openShare("line")} className={btn} aria-label="แชร์ไปยัง LINE">
          LINE
        </button>
        <button type="button" onClick={() => openShare("facebook")} className={btn} aria-label="แชร์ไปยัง Facebook">
          Facebook
        </button>
        <button type="button" onClick={nativeOrCopy} className={btn} aria-label="คัดลอกลิงก์หรือแชร์">
          {copied ? "คัดลอกแล้ว ✓" : "คัดลอกลิงก์"}
        </button>
      </div>
    </div>
  );
}
