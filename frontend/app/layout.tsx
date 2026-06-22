import type { Metadata } from "next";
import { Noto_Sans_Thai } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale } from "next-intl/server";
import "./globals.css";

import { Providers } from "./providers";
import { AuthProvider } from "@/components/auth/AuthProvider";

// Noto Sans Thai for all UI text (Thai + Latin), shared with the career portal.
// One family for body and headings; hierarchy from weight + scale. display: swap.
const thai = Noto_Sans_Thai({
  variable: "--font-body",
  subsets: ["thai", "latin"],
  weight: ["300", "400", "500", "600", "700"],
  display: "swap",
});

export const metadata: Metadata = {
  title: "HR ATS - Recruitment Console",
  description: "AI-powered recruitment screening dashboard",
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const locale = await getLocale();
  return (
    <html lang={locale} className={`${thai.variable} h-full antialiased`}>
      <body
        className="min-h-full flex flex-col bg-background text-foreground"
        style={{ fontFamily: "var(--font-body), system-ui, sans-serif" }}
      >
        <NextIntlClientProvider>
          <Providers>
            <AuthProvider>{children}</AuthProvider>
          </Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
