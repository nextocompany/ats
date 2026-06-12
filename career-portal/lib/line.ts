// LINE auth seam. Without NEXT_PUBLIC_LIFF_ID (CI/local) we return a stub token —
// the backend mock verifier accepts any non-empty X-LINE-IdToken. When the LIFF id
// is set we initialise LIFF and return the real id-token. Every caller already
// routes through getIdToken(), so nothing else changes. See developers.line.biz/liff.

const DEV_STUB_TOKEN = "dev-line-id-token";

export interface LineProfile {
  idToken: string;
  displayName?: string;
}

// isLiffConfigured reports whether a real LIFF id is wired up. When false we fall
// back to the dev stub so the portal builds and runs without LINE credentials.
export function isLiffConfigured(): boolean {
  return Boolean(process.env.NEXT_PUBLIC_LIFF_ID);
}

// getIdToken resolves a LINE id-token to send as X-LINE-IdToken on apply. Async by
// design so the LIFF init (which is awaited) needs no caller change. The @line/liff
// SDK is browser-only, so it is imported dynamically inside the configured branch —
// SSR/build never executes it.
export async function getIdToken(): Promise<string> {
  if (!isLiffConfigured()) return DEV_STUB_TOKEN;
  const { default: liff } = await import("@line/liff");
  await liff.init({ liffId: process.env.NEXT_PUBLIC_LIFF_ID! });
  if (!liff.isLoggedIn()) {
    liff.login();
    return ""; // login() redirects; nothing to return on this pass
  }
  return liff.getIDToken() ?? "";
}
