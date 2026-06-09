import type { Metadata, Viewport } from "next";
import { Noto_Sans_Thai, Inter } from "next/font/google";
import "./globals.css";

import { Providers } from "./providers";

// Thai-first: Noto Sans Thai is the primary face; Inter covers Latin/numerals.
// Two families total, both with display: swap (perf rules).
const notoThai = Noto_Sans_Thai({
  variable: "--font-thai",
  subsets: ["thai"],
  display: "swap",
});
const inter = Inter({ variable: "--font-latin", subsets: ["latin"], display: "swap" });

export const metadata: Metadata = {
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
  // Match the manifest theme_color (deep emerald) so the standalone PWA chrome
  // and the address bar share the portal's identity.
  themeColor: "#0f5132",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="th" className={`${notoThai.variable} ${inter.variable} h-full antialiased`}>
      <body
        className="min-h-full bg-background text-foreground"
        style={{ fontFamily: "var(--font-thai), var(--font-latin), system-ui, sans-serif" }}
      >
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
