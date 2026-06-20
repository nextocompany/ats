import type { Metadata } from "next";
import { Anuphan, IBM_Plex_Sans_Thai_Looped } from "next/font/google";
import { NextIntlClientProvider } from "next-intl";
import { getLocale } from "next-intl/server";
import "./globals.css";

import { Providers } from "./providers";
import { AuthProvider } from "@/components/auth/AuthProvider";

// Institutional type, shared with the career portal: Anuphan (display headings) +
// IBM Plex Sans Thai Looped (body/UI). One neutral superfamily, Thai+Latin parity.
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
  title: "HR ATS - Recruitment Console",
  description: "AI-powered recruitment screening dashboard",
};

export default async function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) {
  const locale = await getLocale();
  return (
    <html lang={locale} className={`${display.variable} ${body.variable} h-full antialiased`}>
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
