"use client";

import { ShieldAlert, ShieldCheck } from "lucide-react";

import { PageHeader } from "@/components/shell/PageHeader";
import { UserManagement } from "@/components/admin/UserManagement";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { useAdminSettings, useMe, useUpdateAdminSettings } from "@/lib/queries";

export default function AdminPage() {
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
        <PageHeader eyebrow="Administration" title="Admin" />
        <section className="flex items-start gap-3 rounded-xl bg-card p-6 ring-1 ring-hairline">
          <ShieldAlert className="mt-0.5 size-5 shrink-0 text-muted-foreground" />
          <p className="text-sm text-muted-foreground">
            These settings are restricted to <span className="font-medium text-foreground">super admins</span>.
            Ask an administrator if you need access.
          </p>
        </section>
      </div>
    );
  }

  return (
    <div className="settle space-y-8">
      <PageHeader
        eyebrow="Administration"
        title="Admin"
        meta="System-wide settings. Changes apply to everyone — handle with care."
      />

      <section className="rounded-xl bg-card ring-1 ring-hairline">
        <header className="border-b border-hairline px-6 py-4">
          <p className="eyebrow">Microsoft sign-in</p>
          <h2 className="mt-1 font-heading text-lg font-semibold tracking-tight">Tenant access</h2>
          <p className="mt-0.5 text-sm text-muted-foreground">
            Controls which Microsoft organizations may sign in to the console.
          </p>
        </header>

        <div className="p-6">
          {isLoading ? (
            <Skeleton className="h-16 w-full" />
          ) : (
            <>
              <div className="flex items-start justify-between gap-6">
                <div className="min-w-0">
                  <label htmlFor="allow-all-tenants" className="text-sm font-medium">
                    Allow all organizations
                  </label>
                  <p className="mt-1 text-sm text-muted-foreground">
                    When <span className="font-medium text-foreground">off</span>, only tenants on the configured
                    allowlist can sign in. When <span className="font-medium text-foreground">on</span>, any Microsoft
                    work/school organization can sign in.
                  </p>
                </div>
                <Switch
                  id="allow-all-tenants"
                  checked={allowAll}
                  onCheckedChange={onToggle}
                  disabled={update.isPending}
                  aria-label="Allow all organizations to sign in"
                />
              </div>

              {/* State-aware advisory — green when restricted, amber when wide open. */}
              {allowAll ? (
                <div className="mt-5 flex items-start gap-3 rounded-lg bg-destructive/10 px-4 py-3 text-sm text-destructive">
                  <ShieldAlert className="mt-0.5 size-4 shrink-0" />
                  <p>
                    Sign-in is open to <span className="font-semibold">any</span> Microsoft organization. Tokens are
                    still cryptographically verified, and users without an assigned role get the most restricted view —
                    but anyone with a work/school account can reach the console. Prefer the allowlist unless you
                    explicitly need open access.
                  </p>
                </div>
              ) : (
                <div className="mt-5 flex items-start gap-3 rounded-lg bg-muted px-4 py-3 text-sm text-muted-foreground">
                  <ShieldCheck className="mt-0.5 size-4 shrink-0 text-brand" />
                  <p>Restricted to the configured tenant allowlist (recommended).</p>
                </div>
              )}

              <p className="mt-4 text-xs text-muted-foreground">
                Note: this controls backend acceptance only. For other organizations to actually reach the Microsoft
                login, the dashboard must be built with the multi-tenant authority and the Entra app registration must
                be multi-tenant.
              </p>

              {update.isError && (
                <p className="mt-3 text-sm text-destructive">Could not save. Please try again.</p>
              )}
            </>
          )}
        </div>
      </section>

      <UserManagement />
    </div>
  );
}
