"use client";

// Per-offer deep link (notification CTA target): /offers/<applicationId>. Auth-
// gated like the offers list; finds the offer for that application and renders it.
// Falls back to a link to the full list when the id has no matching offer (e.g.
// the offer was withdrawn, or the link is stale).
import { use, useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";

import { OfferCard } from "@/components/offers/OfferCard";
import { PortalShell } from "@/components/PortalShell";
import { useMyOffers } from "@/lib/queries";
import { useCandidate } from "@/lib/session";

export default function OfferDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const t = useTranslations("offers");
  const router = useRouter();
  const { isAuthenticated, isLoading } = useCandidate();
  const { data: offers, isError } = useMyOffers();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.replace(`/login?return=/offers/${id}`);
  }, [isLoading, isAuthenticated, router, id]);

  if (isLoading || !isAuthenticated) {
    return (
      <PortalShell backHref="/account" narrow>
        <p className="text-center text-sm text-muted-foreground">{t("loading")}</p>
      </PortalShell>
    );
  }

  // Deep link carries the application id; match the offer for that application.
  const offer = offers?.find((o) => o.application_id === id);

  return (
    <PortalShell backHref="/offers" narrow>
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
        ) : offer ? (
          <OfferCard offer={offer} t={t} />
        ) : (
          <div className="rounded-xl border border-line bg-card p-6 text-center text-sm text-muted-foreground">
            <p>{t("empty")}</p>
            <Link href="/offers" className="mt-3 inline-block font-medium text-primary underline-offset-2 hover:underline">
              {t("title")}
            </Link>
          </div>
        )}
      </div>
    </PortalShell>
  );
}
