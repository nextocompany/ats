import createNextIntlPlugin from "next-intl/plugin";

import type { NextConfig } from "next";

// next-intl: cookie-based locale (config in i18n/request.ts), no URL routing.
const withNextIntl = createNextIntlPlugin();

// API origin the browser calls; allowed in CSP connect-src.
const apiOrigin = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// Header-based CSP (Sprint 6a; script-src relaxed Sprint 6c). Next's App Router
// emits inline hydration/streaming scripts; a strict `script-src 'self'` blocks
// them in production builds (page renders but never hydrates). A per-request nonce
// can't fix statically-prerendered pages (Next emits nonce=undefined for them), so
// we allow 'unsafe-inline' for scripts — Next's inline scripts are framework-
// generated and same-origin. connect-src must include the Go API origin.
//
// Dev-only: Next's webpack dev runtime wraps modules in eval(); without 'unsafe-eval'
// the client bundle never executes under `next dev` (blank/unstyled page). We add it
// ONLY when not building for production, so the prod CSP stays strict (no eval).
const isProd = process.env.NODE_ENV === "production";
const scriptSrc = isProd
  ? "script-src 'self' 'unsafe-inline'"
  : "script-src 'self' 'unsafe-inline' 'unsafe-eval'";
const csp = [
  "default-src 'self'",
  scriptSrc,
  "style-src 'self' 'unsafe-inline'",
  // Azure Blob: source-document previews load the resume directly from a
  // short-lived SAS URL on <account>.blob.core.windows.net (iframe + any <img>).
  "img-src 'self' data: blob: https://*.blob.core.windows.net",
  "font-src 'self'",
  // Entra SSO: MSAL talks to Microsoft login over connect-src, and its silent
  // token acquisition renders a hidden login.microsoftonline.com iframe.
  `connect-src 'self' ${apiOrigin} https://login.microsoftonline.com`,
  // frame-src: MSAL login iframe + the resume preview iframe (Azure Blob SAS).
  "frame-src https://login.microsoftonline.com https://*.blob.core.windows.net",
  "frame-ancestors 'none'",
  "object-src 'none'",
  "base-uri 'self'",
].join("; ");

const securityHeaders = [
  { key: "Content-Security-Policy", value: csp },
  { key: "X-Frame-Options", value: "DENY" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
  { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
];

const nextConfig: NextConfig = {
  // Emit a self-contained server bundle for the container image.
  output: "standalone",
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
};

export default withNextIntl(nextConfig);
