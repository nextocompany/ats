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
      authority: `https://login.microsoftonline.com/${TENANT_ID}`,
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

// --- Self-healing re-authentication (token expiry / 401) -------------------
//
// When the Entra session lapses (ID token ~1h; silent refresh can fail when the
// refresh token expires or third-party cookies are blocked), we bounce the user
// through an interactive Microsoft login instead of stalling on 401s.

// True once a redirect has been kicked off this page load (a redirect navigates
// away and reloads the module, so this naturally resets).
let redirecting = false;
const REAUTH_GUARD_KEY = "hr_reauth_ts";
const REAUTH_MIN_INTERVAL_MS = 60_000;

// Cross-reload loop guard: if a token still fails right after re-login (e.g. a
// valid sign-in the API rejects for a non-expiry reason), don't bounce again —
// let the error surface instead of looping login↔401.
function reauthThrottled(): boolean {
  try {
    return Date.now() - Number(sessionStorage.getItem(REAUTH_GUARD_KEY) ?? "0") < REAUTH_MIN_INTERVAL_MS;
  } catch {
    return false;
  }
}

function markReauth() {
  try {
    sessionStorage.setItem(REAUTH_GUARD_KEY, String(Date.now()));
  } catch {
    /* sessionStorage unavailable — proceed without the cross-reload guard */
  }
}

/** Clears the re-auth throttle after a healthy authenticated call. */
function clearReauthThrottle() {
  try {
    sessionStorage.removeItem(REAUTH_GUARD_KEY);
  } catch {
    /* ignore */
  }
}

// Kicks off an interactive Microsoft login (full-page redirect). Loop-safe:
// no-ops if a redirect is already in flight or one fired in the last minute.
async function reauthenticate(account: AccountInfo | null) {
  if (redirecting || reauthThrottled()) return;
  const instance = await getMsalInstance();
  if (!instance) return;
  redirecting = true;
  markReauth();
  try {
    if (account) {
      await instance.acquireTokenRedirect({ scopes: LOGIN_SCOPES, account });
    } else {
      await instance.loginRedirect({ scopes: LOGIN_SCOPES });
    }
  } catch {
    // interaction_in_progress (a redirect is already happening) or a transient
    // failure — let any in-flight redirect proceed.
    redirecting = false;
  }
}

/** Re-authenticate after the API rejects a request with 401 (Entra mode only). */
export async function handleApiUnauthorized() {
  if (!isEntraConfigured()) return;
  const instance = await getMsalInstance();
  await reauthenticate(instance ? activeAccount(instance) : null);
}

/**
 * Acquires an Entra ID token for the Bearer header. Returns null in DEV mode.
 * On any silent-acquisition failure (or a lapsed account) it kicks off an
 * interactive login redirect and returns null — the page reloads authenticated
 * rather than firing unauthenticated requests that stall on 401.
 */
export async function getIdToken(): Promise<string | null> {
  if (!isEntraConfigured()) return null;
  const instance = await getMsalInstance();
  if (!instance) return null;

  const account = activeAccount(instance);
  if (!account) {
    // No signed-in account — start an interactive login instead of sending an
    // unauthenticated request the API would 401.
    await reauthenticate(null);
    return null;
  }

  try {
    const result = await instance.acquireTokenSilent({
      scopes: LOGIN_SCOPES,
      account,
    });
    clearReauthThrottle(); // healthy token — reset the loop guard
    return result.idToken;
  } catch {
    // Expired refresh token, interaction required, blocked silent iframe, … →
    // bounce through an interactive login instead of stalling on 401s.
    await reauthenticate(account);
    return null;
  }
}
