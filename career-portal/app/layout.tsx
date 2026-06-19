import type { Metadata, Viewport } from "next";
import { Anuphan, IBM_Plex_Sans_Thai_Looped } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale } from "next-intl/server";
import "./globals.css";

import { Providers } from "./providers";

// Institutional type — one neutral superfamily, Thai+Latin parity. Anuphan
// (loopless, modern) carries display headings; IBM Plex Sans Thai Looped (looped
// for Thai reading comfort, IBM's corporate face) carries body/UI. Both ship
// Thai+Latin on the Plex/Cadson-Demak lineage, both display: swap.
const display = Anuphan({
  variable: "--font-display",
  subsets: ["thai", "latin"],
  weight: ["400", "500", "600", "700"],
  display: "swap",
});
const body = IBM_Plex_Sans_Thai_Looped({
  variable: "--font-body",
  subsets: ["thai", "latin"],
  weight: ["300", "400", "500", "600"],
  display: "swap",
});

export const metadata: Metadata = {
  // Resolves relative Open Graph URLs (e.g. per-job /jobs/[id]) to absolute links
  // for LINE/Facebook previews. Falls back to localhost in dev when unset.
  metadataBase: new URL(process.env.NEXT_PUBLIC_PORTAL_BASE_URL ?? "http://localhost:3000"),
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
    <html lang={locale} className={`${display.variable} ${body.variable} h-full antialiased`}>
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
