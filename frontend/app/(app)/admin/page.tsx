"use client";

import { useTranslations } from "next-intl";
import { ShieldAlert, ShieldCheck } from "lucide-react";

import { PageHeader } from "@/components/shell/PageHeader";
import { UserManagement } from "@/components/admin/UserManagement";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { useAdminSettings, useMe, useUpdateAdminSettings } from "@/lib/queries";

export default function AdminPage() {
  const t = useTranslations("admin");
  const { data: me, isLoading: meLoading } = useMe();
  const isSuperAdmin = me?.role === "super_admin";

  const { data: settings, isLoading } = useAdminSettings(isSuperAdmin);
  const update = useUpdateAdminSettings();

  // Optimistic display without local state: while a save is in flight show the
  // pending value; otherwise the server value. On error isPending clears and it
  // falls back to the server value (automatic rollback).
  const allowAll = update.isPending
    ? Boolean(update.variables?.allow_all_tenants)
    : Boolean(settings?.allow_all_tenants);

  function onToggle(next: boolean) {
    update.mutate({ allow_all_tenants: next });
  }

  if (meLoading) {
    return <Skeleton className="h-40 w-full rounded-xl" />;
  }

  if (!isSuperAdmin) {
    return (
      <div className="settle space-y-8">
        <PageHeader eyebrow={t("eyebrow")} title={t("title")} />
        <section className="flex items-start gap-3 rounded-xl bg-card p-6 ring-1 ring-hairline">
          <ShieldAlert className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            {t.rich("restricted", {
              b: (chunks) => <span className="font-medium text-foreground">{chunks}</span>,
            })}
          </p>
        </section>
      </div>
    );
  }

  return (
    <div className="settle space-y-8">
      <PageHeader eyebrow={t("eyebrow")} title={t("title")} meta={t("meta")} />

      <section className="rounded-xl bg-card ring-1 ring-hairline">
        <header className="border-b border-hairline px-6 py-4">
          <p className="eyebrow">{t("ssoEyebrow")}</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">{t("tenantAccess")}</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">{t("tenantAccessDesc")}</p>
        </header>

        <div className="p-6">
          {isLoading ? (
            <Skeleton className="h-16 w-full" />
          ) : (
            <>
              <div className="flex items-start justify-between gap-6">
                <div className="min-w-0">
                  <label htmlFor="allow-all-tenants" className="text-sm font-medium">
                    {t("allowAll")}
                  </label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    {t.rich("allowAllHelp", {
                      b: (chunks) => <span className="font-medium text-foreground">{chunks}</span>,
                    })}
                  </p>
                </div>
                <Switch
                  id="allow-all-tenants"
                  checked={allowAll}
                  onCheckedChange={onToggle}
                  disabled={update.isPending}
                  aria-label={t("allowAllAria")}
                />
              </div>

              {/* State-aware advisory — green when restricted, amber when wide open. */}
              {allowAll ? (
                <div className="mt-5 flex items-start gap-3 rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
                  <ShieldAlert className="mt-0.5 size-4 shrink-0" />
                  <p>
                    {t.rich("advisoryOpen", {
                      b: (chunks) => <span className="font-semibold">{chunks}</span>,
                    })}
                  </p>
                </div>
              ) : (
                <div className="mt-5 flex items-start gap-3 rounded-lg bg-muted px-4 py-3 text-sm text-muted-foreground">
                  <ShieldCheck className="mt-0.5 size-4 shrink-0 text-brand" />
                  <p>{t("advisoryRestricted")}</p>
                </div>
              )}

              <p className="mt-4 text-xs text-muted-foreground">{t("tenantNote")}</p>

              {update.isError && (
                <p className="mt-3 text-sm text-destructive">{t("saveError")}</p>
              )}
            </>
          )}
        </div>
      </section>

      <UserManagement />
    </div>
  );
}
