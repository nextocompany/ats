import type { Metadata, Viewport } from "next";
import { Noto_Sans_Thai } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale } from "next-intl/server";
import "./globals.css";

import { Providers } from "./providers";

// Noto Sans Thai for all UI text (Thai + Latin). One family for body and headings;
// hierarchy comes from weight + scale. display: swap to avoid FOIT. Bound to
// --font-body; globals.css points --font-heading and .num at the same var.
const thai = Noto_Sans_Thai({
  variable: "--font-body",
  subsets: ["thai", "latin"],
  weight: ["300", "400", "500", "600", "700"],
  display: "swap",
});

export const metadata: Metadata = {
  // Resolves relative Open Graph URLs (e.g. per-job /jobs/[id]) to absolute links
  // for LINE/Facebook previews. Falls back to localhost in dev when unset.
  // Use `||` not `??`: the Dockerfile sets ENV NEXT_PUBLIC_PORTAL_BASE_URL to an
  // empty string when the build-arg is omitted, and `new URL("")` throws. `||`
  // catches both undefined and "" so the build never breaks on an unset arg.
  metadataBase: new URL(process.env.NEXT_PUBLIC_PORTAL_BASE_URL || "http://localhost:3000"),
  title: "ร่วมงานกับเรา | สมัครงาน",
  description: "ดูตำแหน่งงานที่เปิดรับและสมัครงานได้ในไม่กี่ขั้นตอน",
  // PWA (Sprint 6c): App Router serves the manifest at /manifest.webmanifest.
  manifest: "/manifest.webmanifest",
  applicationName: "สมัครงาน",
  appleWebApp: { capable: true, statusBarStyle: "default", title: "สมัครงาน" },
  icons: {
    icon: "/favicon.ico",
    apple: "/apple-touch-icon.png",
  },
};

export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  // Match the manifest theme_color (CP Axtra blue) so the standalone PWA chrome
  // and the address bar share the portal's identity.
  themeColor: "#0B47B8",
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const locale = await getLocale();
  return (
    <html lang={locale} className={`${thai.variable} h-full antialiased`}>
      <body
        className="min-h-full bg-background text-foreground"
        style={{ fontFamily: "var(--font-body), system-ui, sans-serif" }}
      >
        <NextIntlClientProvider>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
