import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";

// Career-portal i18n: locale from the NEXT_LOCALE cookie (no URL routing in v1 —
// keeps existing /interview, /status deep links and the PWA manifest stable). Thai
// is the default; English is the toggle. The LocaleSwitcher writes the cookie.
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
