// LINE auth seam. In dev the backend mock verifier accepts any non-empty
// X-LINE-IdToken, so we return a stub token. For production this is the single
// swap point: initialise LIFF and return `window.liff.getIDToken()` — every
// caller already routes through getIdToken(), so nothing else changes.
//
// See developers.line.biz/liff. Real wiring (deploy-time):
//   import liff from "@line/liff";
//   await liff.init({ liffId: process.env.NEXT_PUBLIC_LIFF_ID! });
//   if (!liff.isLoggedIn()) liff.login();
//   return liff.getIDToken() ?? "";

const DEV_STUB_TOKEN = "dev-line-id-token";

export interface LineProfile {
  idToken: string;
  displayName?: string;
}

// isLiffConfigured reports whether a real LIFF id is wired up. Currently always
// false (dev stub); flip by setting NEXT_PUBLIC_LIFF_ID and the LIFF init above.
export function isLiffConfigured(): boolean {
  return Boolean(process.env.NEXT_PUBLIC_LIFF_ID);
}

// getIdToken resolves a LINE id-token to send as X-LINE-IdToken on apply.
// Async by design so the LIFF drop-in (which awaits liff.init) needs no caller change.
export async function getIdToken(): Promise<string> {
  return DEV_STUB_TOKEN;
}
