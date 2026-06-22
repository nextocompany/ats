"use client";

// Share a job to LINE / Facebook / native share sheet, or copy the link. Reads the
// canonical job URL from window.location at click time (no SSR/hydration mismatch).
import { useTranslations } from "next-intl";
import { useState } from "react";
import { CheckIcon, LinkIcon } from "lucide-react";

interface Props {
  title: string;
}

// Brand marks: lucide dropped brand icons, so LINE and Facebook are inline SVGs
// (simple-icons paths) coloured via the button's text colour (currentColor).
function LineIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden="true">
      <path d="M19.365 9.863c.349 0 .63.285.63.631 0 .345-.281.63-.63.63H17.61v1.125h1.755c.348 0 .63.283.63.63 0 .344-.282.629-.63.629h-2.386c-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63h2.386c.346 0 .627.285.627.63 0 .349-.281.63-.63.63H17.61v1.125h1.755zm-3.855 3.016c0 .27-.174.51-.432.596-.064.021-.133.031-.199.031-.211 0-.391-.09-.51-.25l-2.443-3.317v2.94c0 .344-.279.629-.631.629-.346 0-.626-.285-.626-.629V8.108c0-.27.173-.51.43-.595.06-.023.136-.033.194-.033.195 0 .375.104.495.254l2.462 3.33V8.108c0-.345.282-.63.63-.63.345 0 .63.285.63.63v4.771zm-5.741 0c0 .344-.282.629-.631.629-.345 0-.627-.285-.627-.629V8.108c0-.345.282-.63.63-.63.346 0 .628.285.628.63v4.771zm-2.466.629H4.917c-.345 0-.63-.285-.63-.629V8.108c0-.345.285-.63.63-.63.348 0 .63.285.63.63v4.141h1.756c.348 0 .629.283.629.63 0 .344-.282.629-.629.629M24 10.314C24 4.943 18.615.572 12 .572S0 4.943 0 10.314c0 4.811 4.27 8.842 10.035 9.608.391.082.923.258 1.058.59.12.301.079.766.038 1.08l-.164 1.02c-.045.301-.24 1.186 1.049.645 1.291-.539 6.916-4.078 9.436-6.975C23.176 14.393 24 12.458 24 10.314" />
    </svg>
  );
}

function FacebookIcon({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" className={className} fill="currentColor" aria-hidden="true">
      <path d="M9.101 23.691v-7.98H6.627v-3.667h2.474v-1.58c0-4.085 1.848-5.978 5.858-5.978.401 0 .955.042 1.468.103a8.68 8.68 0 0 1 1.141.195v3.325a8.623 8.623 0 0 0-.653-.036 26.805 26.805 0 0 0-.733-.009c-.707 0-1.259.096-1.675.309a1.686 1.686 0 0 0-.679.622c-.258.42-.374.995-.374 1.752v1.297h3.919l-.386 2.103-.287 1.564h-3.246v8.245C19.396 23.238 24 18.179 24 12.044c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.628 3.874 10.35 9.101 11.647Z" />
    </svg>
  );
}

function shareUrl(): string {
  if (typeof window === "undefined") return "";
  return window.location.href;
}

export function ShareButtons({ title }: Props) {
  const t = useTranslations("share");
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
    "inline-flex h-10 items-center justify-center rounded-lg border border-line bg-secondary text-foreground transition-colors hover:bg-secondary/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring";

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-muted-foreground">{t("label")}</p>
      <div className="grid grid-cols-3 gap-2">
        <button
          type="button"
          onClick={() => openShare("line")}
          className={`${btn} hover:text-[#06C755]`}
          aria-label={t("toLine")}
          title={t("toLine")}
        >
          <LineIcon className="size-5" />
        </button>
        <button
          type="button"
          onClick={() => openShare("facebook")}
          className={`${btn} hover:text-[#1877F2]`}
          aria-label={t("toFacebook")}
          title={t("toFacebook")}
        >
          <FacebookIcon className="size-5" />
        </button>
        <button
          type="button"
          onClick={nativeOrCopy}
          className={`${btn} ${copied ? "text-emerald-600" : ""}`}
          aria-label={copied ? t("copied") : t("copyOrShare")}
          title={copied ? t("copied") : t("copyOrShare")}
        >
          {copied ? <CheckIcon className="size-5" /> : <LinkIcon className="size-5" />}
        </button>
      </div>
    </div>
  );
}
