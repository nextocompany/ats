import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";

// Dashboard i18n: locale comes from the NEXT_LOCALE cookie (no URL routing — the
// dashboard is private and middleware-gated). Thai is the default; English is the
// toggle. The LocaleSwitcher writes the cookie + refreshes.
export const LOCALES = ["th", "en"] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = "th";
export const LOCALE_COOKIE = "NEXT_LOCALE";

export default getRequestConfig(async () => {
  const store = await cookies();
  const cookie = store.get(LOCALE_COOKIE)?.value;
  const locale: Locale = (LOCALES as readonly string[]).includes(cookie ?? "")
    ? (cookie as Locale)
    : DEFAULT_LOCALE;
  return {
    locale,
    messages: (await import(`../messages/${locale}.json`)).default,
  };
});
