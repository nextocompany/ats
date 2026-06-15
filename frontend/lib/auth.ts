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

// Marker values written to SESSION_COOKIE. The cookie is a readable UI marker only
// (route gating + which credential the API client should attach); real security is
// the backend bearer/cookie check.
const MARKER_ENTRA = "entra";
const MARKER_PASSWORD = "pw";
const MARKER_DEV = "dev";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

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
export function setSessionMarker(value = MARKER_ENTRA) {
  document.cookie = `${SESSION_COOKIE}=${value}; path=/; max-age=43200; samesite=lax`;
}

function clearSessionMarker() {
  document.cookie = `${SESSION_COOKIE}=; path=/; max-age=0; samesite=lax`;
}

/** Reads the current session marker, or null when signed out / on the server. */
function readMarker(): string | null {
  if (typeof document === "undefined") return null;
  const m = document.cookie.match(new RegExp(`(?:^|;\\s*)${SESSION_COOKIE}=([^;]+)`));
  return m ? decodeURIComponent(m[1]) : null;
}

/**
 * True when the active session is a local password login. Such a session is
 * authenticated by the backend's httpOnly `hr_auth` cookie, so the API client
 * must NOT attach an Entra bearer (which would override it / fail when no MSAL
 * account exists).
 */
export function isPasswordSession(): boolean {
  return readMarker() === MARKER_PASSWORD;
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
  setSessionMarker(MARKER_DEV);
}

/**
 * Signs in with a local username/password. The backend verifies the credentials,
 * sets the httpOnly `hr_auth` session cookie, and we record a readable "pw" marker
 * so route gating + the API client know this is a password (non-Entra) session.
 * Throws with the server message on failure so the form can surface it.
 */
export async function signInWithPassword(email: string, password: string): Promise<void> {
  const res = await fetch(`${API_BASE}/api/v1/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
    credentials: "include",
  });
  let env: { success?: boolean; error?: string } = {};
  try {
    env = await res.json();
  } catch {
    throw new Error(`Sign-in failed (${res.status})`);
  }
  if (!res.ok || !env.success) {
    throw new Error(env.error ?? "Invalid email or password");
  }
  setSessionMarker(MARKER_PASSWORD);
}

export async function signOut() {
  // Password session: revoke server-side + clear cookies, then return to login.
  if (isPasswordSession()) {
    try {
      await fetch(`${API_BASE}/api/v1/auth/logout`, { method: "POST", credentials: "include" });
    } catch {
      // best-effort: clearing the local marker still logs the UI out
    }
    clearSessionMarker();
    if (typeof window !== "undefined") window.location.href = "/login";
    return;
  }
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
  // A password session authenticates via the httpOnly cookie — never attach an
  // Entra bearer (there is no MSAL account, and a stale token would 401).
  if (isPasswordSession()) return null;
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
