"use client";

// Member offers (Module-3 3.6): a logged-in candidate views their offer(s) and
// accepts or declines. Session-gated client-side, mirroring app/account/page.tsx.
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";

import { PortalShell } from "@/components/PortalShell";
import { Button } from "@/components/ui/button";
import { useMyLetters, useMyOffers, useRespondOffer } from "@/lib/queries";
import { useCandidate } from "@/lib/session";
import type { Offer, OfferStatus } from "@/lib/types";

// Typed status → i18n-key map (no unsound `as` cast; exhaustive over OfferStatus).
const STATUS_KEY: Record<OfferStatus, "status_draft" | "status_sent" | "status_accepted" | "status_declined" | "status_expired"> = {
  draft: "status_draft",
  sent: "status_sent",
  accepted: "status_accepted",
  declined: "status_declined",
  expired: "status_expired",
};

// Dates are stored UTC-midnight; render in UTC so a non-UTC viewer never sees an
// off-by-one calendar day.
function formatThaiDate(iso: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return new Intl.DateTimeFormat("th-TH", { dateStyle: "long", timeZone: "UTC" }).format(d);
}

export default function OffersPage() {
  const t = useTranslations("offers");
  const router = useRouter();
  const { isAuthenticated, isLoading } = useCandidate();
  const { data: offers, isError } = useMyOffers();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.replace("/login?return=/offers");
  }, [isLoading, isAuthenticated, router]);

  if (isLoading || !isAuthenticated) {
    return (
      <PortalShell backHref="/account" narrow>
        <p className="text-center text-sm text-muted-foreground">{t("loading")}</p>
      </PortalShell>
    );
  }

  return (
    <PortalShell backHref="/account" narrow>
      <div className="flex flex-col gap-6">
        <header className="border-b border-line pb-5">
          <h1 className="[font-size:var(--text-h2)] font-semibold leading-tight text-foreground">{t("title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        </header>

        {isError ? (
          <p role="alert" className="rounded-xl border border-line bg-card p-6 text-center text-sm text-destructive">
            {t("loadFailed")}
          </p>
        ) : !offers ? (
          <p className="text-center text-sm text-muted-foreground">{t("loading")}</p>
        ) : offers.length === 0 ? (
          <p className="rounded-xl border border-line bg-card p-6 text-center text-sm text-muted-foreground">
            {t("empty")}
          </p>
        ) : (
          <ul className="flex flex-col gap-4">
            {offers.map((o) => (
              <li key={o.id}>
                <OfferCard offer={o} t={t} formatDate={formatThaiDate} />
              </li>
            ))}
          </ul>
        )}

        <DocumentsSection t={t} formatDate={formatThaiDate} />
      </div>
    </PortalShell>
  );
}

function DocumentsSection({
  t,
  formatDate,
}: {
  t: ReturnType<typeof useTranslations>;
  formatDate: (iso: string | null) => string;
}) {
  const { data: letters, isError } = useMyLetters();
  if (isError) {
    return (
      <section className="border-t border-line pt-6">
        <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">{t("documentsTitle")}</h2>
        <p role="alert" className="mt-3 text-sm text-destructive">{t("documentsFailed")}</p>
      </section>
    );
  }
  if (!letters || letters.length === 0) return null;
  return (
    <section className="border-t border-line pt-6">
      <h2 className="[font-size:var(--text-h3)] font-semibold text-foreground">{t("documentsTitle")}</h2>
      <ul className="mt-3 flex flex-col gap-2">
        {letters.map((l) => (
          <li key={l.id} className="flex items-center justify-between gap-3 rounded-lg border border-line bg-card px-4 py-3">
            <span className="text-sm text-foreground">
              {l.type === "interview" ? t("docInterview") : t("docOffer")}
              <span className="ml-2 text-xs text-muted-foreground">{formatDate(l.created_at)}</span>
            </span>
            {l.url && (
              <a href={l.url} target="_blank" rel="noopener noreferrer" className="text-sm font-medium text-primary underline-offset-2 hover:underline">
                {t("download")}
              </a>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

function OfferCard({
  offer,
  t,
  formatDate,
}: {
  offer: Offer;
  t: ReturnType<typeof useTranslations>;
  formatDate: (iso: string | null) => string;
}) {
  const respond = useRespondOffer(offer.id);
  const [declining, setDeclining] = useState(false);
  const [reason, setReason] = useState("");

  const respondable = offer.status === "sent";

  function accept() {
    respond.mutate({ decision: "accept" });
  }
  function decline(e: React.FormEvent) {
    e.preventDefault();
    if (!reason.trim()) return;
    respond.mutate({ decision: "decline", reason: reason.trim() });
  }

  return (
    <div className="space-y-4 rounded-xl border border-line bg-card p-6">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="font-medium text-foreground">{offer.position_title || t("position")}</p>
          <p className="text-xs text-muted-foreground">{t(STATUS_KEY[offer.status] ?? "status_sent")}</p>
        </div>
        {offer.salary != null && (
          <p className="text-right text-sm font-semibold tabular-nums text-foreground">
            {offer.salary.toLocaleString("th-TH", { maximumFractionDigits: 0 })} {t("baht")}
          </p>
        )}
      </div>

      <dl className="space-y-2 border-t border-line pt-4 text-sm">
        <div className="flex justify-between gap-4">
          <dt className="text-muted-foreground">{t("startDate")}</dt>
          <dd className="text-right font-medium">{formatDate(offer.start_date)}</dd>
        </div>
        {offer.expires_at && (
          <div className="flex justify-between gap-4">
            <dt className="text-muted-foreground">{t("expires")}</dt>
            <dd className="text-right font-medium">{formatDate(offer.expires_at)}</dd>
          </div>
        )}
      </dl>

      {offer.terms && <p className="rounded-lg bg-secondary px-3 py-2 text-sm text-foreground/80">{offer.terms}</p>}

      {offer.status === "accepted" && <p className="text-sm font-medium text-primary">{t("acceptedNote")}</p>}
      {offer.status === "declined" && <p className="text-sm text-muted-foreground">{t("declinedNote")}</p>}
      {offer.status === "expired" && <p className="text-sm text-destructive">{t("expiredNote")}</p>}

      {respond.isError && (
        <p role="alert" className="text-sm font-medium text-destructive">
          {respond.error instanceof Error ? respond.error.message : t("respondFailed")}
        </p>
      )}

      {respondable && !declining && (
        <div className="flex gap-3">
          <Button onClick={accept} disabled={respond.isPending} className="gap-2">
            {respond.isPending && <Loader2 className="size-4 animate-spin" />}
            {t("accept")}
          </Button>
          <Button variant="outline" onClick={() => setDeclining(true)} disabled={respond.isPending}>
            {t("decline")}
          </Button>
        </div>
      )}

      {respondable && declining && (
        <form onSubmit={decline} className="space-y-3" noValidate>
          <label className="block space-y-1.5">
            <span className="text-sm font-medium text-foreground">{t("declineReasonLabel")}</span>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              rows={3}
              required
              className="w-full rounded-lg border border-line bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-primary"
              placeholder={t("declineReasonPlaceholder")}
            />
          </label>
          <div className="flex gap-3">
            <Button type="button" variant="ghost" onClick={() => setDeclining(false)}>
              {t("cancel")}
            </Button>
            <Button type="submit" variant="outline" className="gap-2" disabled={!reason.trim() || respond.isPending}>
              {respond.isPending && <Loader2 className="size-4 animate-spin" />}
              {t("confirmDecline")}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}
