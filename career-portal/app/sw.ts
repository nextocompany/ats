/// <reference lib="webworker" />
import { defaultCache } from "@serwist/next/worker";
import type { PrecacheEntry, RuntimeCaching, SerwistGlobalConfig } from "serwist";
import { NetworkFirst, Serwist } from "serwist";

// Serwist runs in the service-worker global scope, not a window. These declarations
// give `self` the worker typing and the precache manifest Serwist injects at build.
declare global {
  interface WorkerGlobalScope extends SerwistGlobalConfig {
    __SW_MANIFEST: (PrecacheEntry | string)[] | undefined;
  }
}
declare const self: ServiceWorkerGlobalScope;

// The Go API origin the candidate's browser calls (inlined at build time, mirrors
// lib/api.ts). Only GETs to the public endpoints are cached — the apply POST
// (PII + a LINE id-token) is never cached or replayed offline.
const apiOrigin = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
const apiOriginUrl = new URL(apiOrigin).origin;

// NetworkFirst for public GETs: a connected user always gets fresh positions/
// status, but a warm offline user still sees the last-known response.
const publicApiCache: RuntimeCaching = {
  matcher: ({ url, request }) =>
    request.method === "GET" &&
    url.origin === apiOriginUrl &&
    url.pathname.startsWith("/api/v1/public/"),
  handler: new NetworkFirst({
    cacheName: "career-public-api",
    networkTimeoutSeconds: 5,
    plugins: [
      {
        // Only persist successful, opaque-free responses.
        cacheWillUpdate: async ({ response }) =>
          response && response.status === 200 ? response : null,
      },
    ],
  }),
};

const serwist = new Serwist({
  precacheEntries: self.__SW_MANIFEST,
  skipWaiting: true,
  clientsClaim: true,
  navigationPreload: true,
  // API rule first so it wins over the generic defaultCache navigation/static rules.
  runtimeCaching: [publicApiCache, ...defaultCache],
  // Any uncached document navigation while offline falls back to the branded
  // /offline shell (precached as part of the build manifest).
  fallbacks: {
    entries: [
      {
        url: "/offline",
        matcher: ({ request }) => request.destination === "document",
      },
    ],
  },
});

serwist.addEventListeners();
