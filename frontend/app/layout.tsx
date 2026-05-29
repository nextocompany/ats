import type { Metadata } from "next";
import { Geist, Noto_Sans_Thai } from "next/font/google";
import "./globals.css";

import { Providers } from "./providers";

const geist = Geist({ variable: "--font-geist-sans", subsets: ["latin"], display: "swap" });
// Thai support is the deliberate exception to the two-font rule.
const notoThai = Noto_Sans_Thai({ variable: "--font-thai", subsets: ["thai"], display: "swap" });

export const metadata: Metadata = {
  title: "HR ATS — Recruitment Console",
  description: "AI-powered recruitment screening dashboard",
};

export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className={`${geist.variable} ${notoThai.variable} h-full antialiased`}>
      <body
        className="min-h-full flex flex-col bg-background text-foreground"
        style={{ fontFamily: "var(--font-geist-sans), var(--font-thai), system-ui, sans-serif" }}
      >
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
