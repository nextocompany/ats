# syntax=docker/dockerfile:1
# Multi-stage build for the public career portal (Next.js 16, standalone output).
# Build context is the career-portal/ directory (pnpm-lock.yaml lives here).
#   docker build -f career-portal/Dockerfile \
#     --build-arg NEXT_PUBLIC_API_URL=https://api.example.com \
#     -t hr-ats/career-portal career-portal
ARG NODE_VERSION=20-alpine

# ---- Stage 1: deps ----------------------------------------------------------
FROM node:${NODE_VERSION} AS deps
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

# ---- Stage 2: build ---------------------------------------------------------
FROM node:${NODE_VERSION} AS build
WORKDIR /app
RUN corepack enable && corepack prepare pnpm@9.15.9 --activate
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# Public API origin baked into the client bundle at build time.
ARG NEXT_PUBLIC_API_URL=http://localhost:8080
ENV NEXT_PUBLIC_API_URL=${NEXT_PUBLIC_API_URL}
# LIFF id (slice 2.3) — compile-time baked. Unset → portal uses the dev LINE
# stub; set → real LIFF login.
ARG NEXT_PUBLIC_LIFF_ID
ENV NEXT_PUBLIC_LIFF_ID=${NEXT_PUBLIC_LIFF_ID}
# Portal public origin for absolute Open Graph share links (LINE/Facebook previews).
# Unset → falls back to localhost (share buttons still work via window.location).
ARG NEXT_PUBLIC_PORTAL_BASE_URL
ENV NEXT_PUBLIC_PORTAL_BASE_URL=${NEXT_PUBLIC_PORTAL_BASE_URL}
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm build

# ---- Stage 3: runtime -------------------------------------------------------
FROM node:${NODE_VERSION} AS runtime
WORKDIR /app
ENV NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=3000 \
    HOSTNAME=0.0.0.0
# Non-root runtime user (uid/gid 1001).
RUN addgroup -g 1001 -S nodejs \
  && adduser -S -u 1001 -G nodejs nextjs
# Standalone server + assets. server.js sits at the root of the standalone tree;
# static and public are copied to the paths the server expects at runtime.
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
COPY --from=build --chown=nextjs:nodejs /app/public ./public
USER nextjs
EXPOSE 3000
CMD ["node", "server.js"]
