"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { MsalProvider } from "@azure/msal-react";
import type { IPublicClientApplication } from "@azure/msal-browser";

import { getMsalInstance, isEntraConfigured, setSessionMarker } from "@/lib/auth";

/**
 * Auth boundary. In DEV mode (Entra not configured) it renders children
 * unchanged so local dev / e2e / CI behavior is identical. In ENTRA mode it
 * wraps children in MsalProvider, finishes any pending redirect on mount, sets
 * the UI marker cookie, and routes to /dashboard after a successful login.
 */
export function AuthProvider({ children }: { children: React.ReactNode }) {
  if (!isEntraConfigured()) {
    return <>{children}</>;
  }
  return <EntraAuthProvider>{children}</EntraAuthProvider>;
}

function EntraAuthProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [instance, setInstance] = useState<IPublicClientApplication | null>(null);

  useEffect(() => {
    let cancelled = false;

    (async () => {
      const msal = await getMsalInstance();
      if (!msal || cancelled) return;
      setInstance(msal);

      const result = await msal.handleRedirectPromise();
      if (cancelled) return;

      if (result?.account) {
        msal.setActiveAccount(result.account);
        setSessionMarker();
        router.replace("/dashboard");
      } else if (msal.getAllAccounts().length > 0) {
        // Returning visit with a cached account — ensure active + marker set.
        msal.setActiveAccount(msal.getActiveAccount() ?? msal.getAllAccounts()[0]);
        setSessionMarker();
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [router]);

  // Until the instance is ready, render children directly. The login page and
  // server middleware still function; token acquisition simply no-ops until the
  // instance initializes.
  if (!instance) {
    return <>{children}</>;
  }

  return <MsalProvider instance={instance}>{children}</MsalProvider>;
}
