import withSerwistInit from "@serwist/next";

import type { NextConfig } from "next";

// API origin the candidate's browser calls; allowed in CSP connect-src.
const apiOrigin = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// Header-based CSP (Sprint 6a; script-src relaxed Sprint 6c). Next's App Router
// emits inline hydration/streaming scripts; a strict `script-src 'self'` blocks
// them in production builds (the page renders but never hydrates). A per-request
// nonce can't fix statically-prerendered pages (Next emits nonce=undefined for
// them), so we allow 'unsafe-inline' for scripts — Next's inline scripts are
// framework-generated and same-origin. worker-src/manifest-src 'self' permit the
// PWA service worker + manifest (Sprint 6c).
// `next dev --webpack` relies on eval() for HMR/source-maps, which a strict
// script-src blocks (pages render but never hydrate). Allow 'unsafe-eval' in
// development ONLY; production stays strict. Dev also widens connect-src to local
// origins so a mock API on any localhost port works during design work.
const isDev = process.env.NODE_ENV === "development";

const csp = [
  "default-src 'self'",
  isDev ? "script-src 'self' 'unsafe-inline' 'unsafe-eval'" : "script-src 'self' 'unsafe-inline'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self'",
  isDev ? `connect-src 'self' ${apiOrigin} http://localhost:* ws://localhost:*` : `connect-src 'self' ${apiOrigin}`,
  "worker-src 'self'",
  "manifest-src 'self'",
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
  // Emit a self-contained server bundle (.next/standalone) for the container image.
  output: "standalone",
  async headers() {
    return [{ source: "/(.*)", headers: securityHeaders }];
  },
};

// Serwist (Sprint 6c): compiles app/sw.ts → public/sw.js (gitignored) and injects
// the SW registration into the client bundle (same-origin, satisfies the 6a CSP
// script-src/worker-src 'self'). Disabled in `next dev` so HMR isn't cached.
const withSerwist = withSerwistInit({
  swSrc: "app/sw.ts",
  swDest: "public/sw.js",
  disable: process.env.NODE_ENV === "development",
});

export default withSerwist(nextConfig);
