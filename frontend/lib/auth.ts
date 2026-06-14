// Dual-mode auth.
//
// DEV MODE (no NEXT_PUBLIC_AZURE_AD_CLIENT_ID): a `hr_session=dev` cookie gates
// the UI. The Go mock backend trusts a mock super_admin, so the cookie is purely
// UI gating and no bearer token is attached. This preserves local dev + e2e + CI.
//
// ENTRA MODE (NEXT_PUBLIC_AZURE_AD_CLIENT_ID set): real Entra ID (Azure AD) SSO
// via MSAL. We obtain an ID token (aud = our client ID) and attach it as the
// Bearer on API calls; the Go API validates it via OIDC discovery. A
// `hr_session=entra` marker cookie keeps the existing server middleware gating
// routes unchanged — it is only a UI marker; real security is the backend check.
import type {
  AccountInfo,
  Configuration,
  PublicClientApplication as PublicClientApplicationType,
} from "@azure/msal-browser";

export const SESSION_COOKIE = "hr_session";

// Scopes for an ID token only — no Graph/API access tokens needed; the ID token
// itself is the bearer (aud = client ID) the Go backend validates.
const LOGIN_SCOPES = ["openid", "profile"];

const CLIENT_ID = process.env.NEXT_PUBLIC_AZURE_AD_CLIENT_ID;
const TENANT_ID = process.env.NEXT_PUBLIC_AZURE_AD_TENANT_ID;

// Authority. Single-tenant pins login to the home tenant. Multi-tenant
// deployments set NEXT_PUBLIC_AZURE_AD_AUTHORITY to
// "https://login.microsoftonline.com/organizations" so MSAL accepts any
// work/school account; the backend then enforces the AZURE_AD_ALLOWED_TENANTS
// allowlist. (The app registration must be multi-tenant for this to work.)
const AUTHORITY =
  process.env.NEXT_PUBLIC_AZURE_AD_AUTHORITY ??
  `https://login.microsoftonline.com/${TENANT_ID}`;

/** True when Entra SSO is configured at build time. */
export function isEntraConfigured(): boolean {
  return Boolean(CLIENT_ID);
}

function redirectUri(): string {
  return (
    process.env.NEXT_PUBLIC_AZURE_AD_REDIRECT_URI ??
    (typeof window !== "undefined" ? window.location.origin : "")
  );
}

// --- MSAL instance (lazy, browser-only) -----------------------------------

let msalInstance: PublicClientApplicationType | null = null;
let initPromise: Promise<PublicClientApplicationType> | null = null;

function buildConfig(): Configuration {
  return {
    auth: {
      clientId: CLIENT_ID as string,
      authority: AUTHORITY,
      redirectUri: redirectUri(),
    },
    cache: {
      cacheLocation: "localStorage",
    },
  };
}

/**
 * Returns the singleton MSAL instance, constructing + initializing it lazily.
 * Only ever runs in the browser (guarded on `typeof window`) so it never
 * executes during SSR/prerender. Returns null when not in Entra mode or on
 * the server.
 */
export async function getMsalInstance(): Promise<PublicClientApplicationType | null> {
  if (!isEntraConfigured() || typeof window === "undefined") return null;
  if (msalInstance) return msalInstance;
  if (!initPromise) {
    initPromise = (async () => {
      const { PublicClientApplication } = await import("@azure/msal-browser");
      const instance = new PublicClientApplication(buildConfig());
      await instance.initialize();
      msalInstance = instance;
      return instance;
    })();
  }
  return initPromise;
}

// --- Marker cookie ---------------------------------------------------------

/** Sets the UI marker cookie so server middleware keeps gating routes. */
export function setSessionMarker(value = "entra") {
  document.cookie = `${SESSION_COOKIE}=${value}; path=/; max-age=43200; samesite=lax`;
}

function clearSessionMarker() {
  document.cookie = `${SESSION_COOKIE}=; path=/; max-age=0; samesite=lax`;
}

// --- Public API ------------------------------------------------------------

export async function signIn() {
  if (isEntraConfigured()) {
    const instance = await getMsalInstance();
    if (!instance) return;
    await instance.loginRedirect({ scopes: LOGIN_SCOPES });
    return;
  }
  // DEV: 12h dev session.
  document.cookie = `${SESSION_COOKIE}=dev; path=/; max-age=43200; samesite=lax`;
}

export async function signOut() {
  if (isEntraConfigured()) {
    clearSessionMarker();
    const instance = await getMsalInstance();
    if (!instance) return;
    await instance.logoutRedirect();
    return;
  }
  // DEV:
  clearSessionMarker();
}

function activeAccount(instance: PublicClientApplicationType): AccountInfo | null {
  return instance.getActiveAccount() ?? instance.getAllAccounts()[0] ?? null;
}

/**
 * Acquires an Entra ID token for the Bearer header. Returns null in DEV mode.
 * On silent-acquisition failure that requires interaction, kicks off a redirect
 * and returns null (the page will reload post-redirect).
 */
export async function getIdToken(): Promise<string | null> {
  if (!isEntraConfigured()) return null;
  const instance = await getMsalInstance();
  if (!instance) return null;

  const account = activeAccount(instance);
  if (!account) return null;

  try {
    const result = await instance.acquireTokenSilent({
      scopes: LOGIN_SCOPES,
      account,
    });
    return result.idToken;
  } catch (err) {
    const { InteractionRequiredAuthError } = await import("@azure/msal-browser");
    if (err instanceof InteractionRequiredAuthError) {
      await instance.acquireTokenRedirect({ scopes: LOGIN_SCOPES, account });
      return null;
    }
    throw err;
  }
}
