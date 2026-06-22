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

// DpoDirectory is the published Data Protection Officer contact block (PDPA s.41):
// the controller plus every active DPO-flagged account, shown in the contact section
// so a data subject can reach a DPO.
interface DpoOfficer {
  name: string;
  email: string;
  phone: string;
  is_primary: boolean;
}
interface DpoDirectory {
  company: string;
  officers: DpoOfficer[];
}

async function getDpo(): Promise<DpoDirectory | null> {
  try {
    const res = await fetch(`${API_BASE}/api/v1/pdpa/dpo`, { cache: "no-store" });
    if (!res.ok) return null;
    const env = (await res.json()) as { success: boolean; data: DpoDirectory };
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
  const [doc, dpo] = await Promise.all([getPolicy(locale), getDpo()]);
  const officers = dpo?.officers ?? [];

  // Body paragraphs split on blank lines; the registry seed is a single paragraph.
  const paragraphs = (doc?.body ?? t("fallbackBody")).split(/\n\s*\n/).filter(Boolean);

  // Feature the lead officer; the rest collapse behind a native <details> so the
  // legal contact stays visible without JS while a long roster stays compact.
  // The API sorts primary-first, so officers[0] is the fallback lead.
  const primaryOfficer = officers.find((o) => o.is_primary) ?? officers[0];
  const otherOfficers = officers.filter((o) => o !== primaryOfficer);

  const renderOfficer = (o: DpoOfficer, key: number, lead: boolean) => (
    <dl key={key} className="flex flex-col gap-1 rounded-lg border border-line p-3">
      {lead && otherOfficers.length > 0 ? (
        <span className="mb-1 inline-block w-fit rounded bg-foreground/5 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          {t("dpoPrimaryBadge")}
        </span>
      ) : null}
      {o.name.trim() !== "" ? (
        <div className="flex gap-2">
          <dt className="w-24 shrink-0 text-muted-foreground">{t("dpoName")}</dt>
          <dd>{o.name}</dd>
        </div>
      ) : null}
      {o.email.trim() !== "" ? (
        <div className="flex gap-2">
          <dt className="w-24 shrink-0 text-muted-foreground">{t("dpoEmail")}</dt>
          <dd>
            <a
              href={`mailto:${o.email}`}
              className="underline underline-offset-4 transition-colors hover:text-muted-foreground"
            >
              {o.email}
            </a>
          </dd>
        </div>
      ) : null}
      {o.phone.trim() !== "" ? (
        <div className="flex gap-2">
          <dt className="w-24 shrink-0 text-muted-foreground">{t("dpoPhone")}</dt>
          <dd>
            <a
              href={`tel:${o.phone.replace(/\s+/g, "")}`}
              className="underline underline-offset-4 transition-colors hover:text-muted-foreground"
            >
              {o.phone}
            </a>
          </dd>
        </div>
      ) : null}
    </dl>
  );

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
              {primaryOfficer ? (
                <div className="mt-1 flex flex-col gap-4 text-sm text-foreground/90">
                  {dpo?.company.trim() ? (
                    <p>
                      <span className="text-muted-foreground">{t("dpoCompany")}: </span>
                      {dpo.company}
                    </p>
                  ) : null}
                  {renderOfficer(primaryOfficer, 0, true)}
                  {otherOfficers.length > 0 ? (
                    <details className="group rounded-lg border border-line">
                      <summary className="cursor-pointer list-none px-3 py-2.5 font-medium text-foreground transition-colors hover:text-muted-foreground">
                        {t("dpoMore", { count: otherOfficers.length })}
                      </summary>
                      <div className="grid gap-3 p-3 pt-0 sm:grid-cols-2">
                        {otherOfficers.map((o, i) => renderOfficer(o, i + 1, false))}
                      </div>
                    </details>
                  ) : null}
                </div>
              ) : null}
            </section>
          </article>
        </Container>
      </main>
      <SiteFooter />
    </div>
  );
}
