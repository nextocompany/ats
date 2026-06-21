"use client";

import { useTranslations } from "next-intl";

import { useMe } from "@/lib/queries";
import { signOut } from "@/lib/auth";

// AccessGuard enforces default-deny across the console. Authentication is handled
// upstream (Entra SSO / password session); here we gate on AUTHORIZATION: a signed-in
// account with no assigned permissions sees a "contact your administrator" screen
// instead of the app. Roles are granted in-app by an admin (CPO/CHRO/super_admin),
// so a freshly SSO-provisioned user lands here until access is assigned.
export function AccessGuard({ children }: { children: React.ReactNode }) {
  const t = useTranslations("access");
  const { data: me, isLoading, isError } = useMe();

  // Still resolving identity: brief checking state (avoids flashing the deny screen).
  if (isLoading) {
    return (
      <div
        className="flex min-h-[60dvh] items-center justify-center text-sm text-muted-foreground"
        role="status"
        aria-live="polite"
      >
        <span className="size-4 animate-spin rounded-full border-2 border-current border-t-transparent" aria-hidden />
        <span className="ml-3">{t("checking")}</span>
      </div>
    );
  }

  // On a load error, defer to the page/API layer (a 401 triggers the login redirect).
  // Only an authenticated identity with zero permissions is treated as "no access".
  if (!isError && me && (me.permissions?.length ?? 0) === 0) {
    return (
      <section
        aria-labelledby="no-access-title"
        className="mx-auto flex min-h-[60dvh] max-w-xl flex-col items-center justify-center text-center"
      >
        <div className="w-full rounded-2xl border border-hairline bg-card px-7 py-10 shadow-sm">
          <h1 id="no-access-title" className="text-balance text-xl font-semibold text-foreground">
            {t("title")}
          </h1>
          <p className="mt-3 text-pretty text-sm leading-relaxed text-muted-foreground">{t("body")}</p>
          <p className="mt-2 text-pretty text-sm leading-relaxed text-muted-foreground">{t("contact")}</p>

          {me.email ? (
            <p className="mt-6 text-xs text-muted-foreground/80">
              {t("signedInAs")} <span className="font-medium text-foreground">{me.email}</span>
            </p>
          ) : null}

          <button
            type="button"
            onClick={() => void signOut()}
            className="mt-4 inline-flex items-center justify-center rounded-lg border border-hairline px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            {t("signOut")}
          </button>
        </div>
      </section>
    );
  }

  return <>{children}</>;
}
