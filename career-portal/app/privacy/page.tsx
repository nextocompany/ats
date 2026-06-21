import type { Metadata } from "next";
import Link from "next/link";
import { getLocale, getTranslations } from "next-intl/server";

import { Container, Eyebrow } from "@/components/ds";
import { SiteFooter } from "@/components/SiteFooter";
import { SiteHeader } from "@/components/SiteHeader";

export const metadata: Metadata = {
  title: "นโยบายความเป็นส่วนตัว (PDPA) | CP Axtra Careers",
  description:
    "นโยบายการคุ้มครองข้อมูลส่วนบุคคลของผู้สมัครงาน CP Axtra ตาม พ.ร.บ. คุ้มครองข้อมูลส่วนบุคคล พ.ศ. 2562",
};

// The privacy notice is rendered fresh from the consent-document registry so the
// page always shows the CURRENT version (the same version stamped on consents).
interface PolicyDoc {
  version: string;
  locale: string;
  title: string;
  body: string;
  effective_at: string;
}

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

async function getPolicy(locale: string): Promise<PolicyDoc | null> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/pdpa/policy/current?locale=${locale}`, {
      // Always fetch the live current document; it changes only on a policy update.
      cache: "no-store",
    });
    if (!res.ok) return null;
    const env = (await res.json()) as { success: boolean; data: PolicyDoc };
    return env.success ? env.data : null;
  } catch {
    return null;
  }
}

function formatDate(iso: string, locale: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return new Intl.DateTimeFormat(locale === "en" ? "en-GB" : "th-TH", {
    year: "numeric",
    month: "long",
    day: "numeric",
  }).format(d);
}

export default async function PrivacyPage() {
  const locale = await getLocale();
  const t = await getTranslations("privacy");
  const doc = await getPolicy(locale);

  // Body paragraphs split on blank lines; the registry seed is a single paragraph.
  const paragraphs = (doc?.body ?? t("fallbackBody")).split(/\n\s*\n/).filter(Boolean);

  return (
    <div className="flex min-h-screen flex-col">
      <SiteHeader />
      <main className="flex-1">
        <Container narrow className="py-14 sm:py-20">
          <article className="flex flex-col gap-10">
            <header className="flex flex-col gap-4">
              <Eyebrow>PDPA</Eyebrow>
              <h1 className="text-balance text-3xl font-semibold tracking-tight text-foreground sm:text-4xl">
                {doc?.title ?? t("heading")}
              </h1>
              {doc ? (
                <p className="text-sm text-muted-foreground">
                  {t("version", { version: doc.version })}
                  <span aria-hidden="true" className="mx-2">
                    &middot;
                  </span>
                  {t("lastUpdated", { date: formatDate(doc.effective_at, locale) })}
                </p>
              ) : null}
            </header>

            <section className="flex flex-col gap-4 text-base leading-relaxed text-foreground/90">
              {paragraphs.map((p, i) => (
                <p key={i}>{p}</p>
              ))}
            </section>

            <section aria-labelledby="rights-heading" className="flex flex-col gap-3 border-t border-line pt-8">
              <h2 id="rights-heading" className="text-xl font-semibold text-foreground">
                {t("rightsTitle")}
              </h2>
              <p className="leading-relaxed text-foreground/90">{t("rightsBody")}</p>
              <Link
                href="/account"
                className="font-medium text-foreground underline underline-offset-4 transition-colors hover:text-muted-foreground"
              >
                {t("rightsCta")}
              </Link>
            </section>

            <section aria-labelledby="cookies-heading" className="flex flex-col gap-3 border-t border-line pt-8">
              <h2 id="cookies-heading" className="text-xl font-semibold text-foreground">
                {t("cookieTitle")}
              </h2>
              <p className="leading-relaxed text-foreground/90">{t("cookieBody")}</p>
            </section>

            <section aria-labelledby="contact-heading" className="flex flex-col gap-3 border-t border-line pt-8">
              <h2 id="contact-heading" className="text-xl font-semibold text-foreground">
                {t("contactTitle")}
              </h2>
              <p className="leading-relaxed text-foreground/90">{t("contactBody")}</p>
            </section>
          </article>
        </Container>
      </main>
      <SiteFooter />
    </div>
  );
}
