"use client";

// TH/EN language toggle. Persists the choice in the NEXT_LOCALE cookie (read by
// i18n/request.ts) and refreshes so the server re-renders with the new locale.
import { useLocale, useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useTransition } from "react";

import { cn } from "@/lib/utils";

const OPTIONS = [
  { value: "th", label: "ไทย" },
  { value: "en", label: "EN" },
] as const;

export function LocaleSwitcher({ className }: { className?: string }) {
  const locale = useLocale();
  const t = useTranslations("common");
  const router = useRouter();
  const [pending, startTransition] = useTransition();

  function set(next: string) {
    if (next === locale) return;
    document.cookie = `NEXT_LOCALE=${next}; path=/; max-age=31536000; samesite=lax`;
    startTransition(() => router.refresh());
  }

  return (
    <div
      role="group"
      aria-label={t("language")}
      className={cn("inline-flex items-center gap-0.5 rounded-full bg-muted p-0.5 text-xs", className)}
    >
      {OPTIONS.map((o) => (
        <button
          key={o.value}
          type="button"
          onClick={() => set(o.value)}
          disabled={pending}
          aria-pressed={locale === o.value}
          className={cn(
            "rounded-full px-2.5 py-1 font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            locale === o.value ? "bg-card text-foreground shadow-sm" : "text-muted-foreground hover:text-foreground",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
