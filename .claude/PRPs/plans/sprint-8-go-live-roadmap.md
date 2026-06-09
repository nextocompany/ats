# Sprint 8 — UAT / Go-Live Roadmap

> Milestone roadmap (not a single implementation plan). Each numbered **slice** below is sized to later run through `/prp-plan` → `/prp-implement` → `/code-review` → `/prp-pr`, exactly like the Sprint 7 slices. Decisions captured here unblock those plans.

## Target & Scope (confirmed 2026-06-04)

- **Deploy target:** Azure Container Apps (ACA) — Azure-native, container-based, Key Vault + Managed Identity for secrets, native single-replica pinning for the scheduler.
- **v1 go-live real seams:** Azure OpenAI + Document Intelligence (AI) · real Azure Blob · **LINE Login + Notifications** · **Azure AI Search**.
- **Deferred to phase 2 (stay mock):** **Entra SSO** (`AUTH_PROVIDER`) · **PeopleSoft live** (`PS_PROVIDER`, CSV fallback in the interim).
- **UAT:** dedicated **staging** environment with real Azure/integrations; UAT passes there *before* prod cutover.

### ⚠️ Top-level consequence of deferring Entra (must confirm)
With `AUTH_PROVIDER=mock` and `ENV != development`, the HR API auth middleware injects **no user** and **fails closed** — the **HR dashboard is unusable in prod** (`backend/internal/middleware/auth.go:19-20`, `mock_jwt.go:21-32`). Therefore v1 go-live is effectively **career-portal-first**: candidates apply through the public portal (real AI screening + LINE + notifications), while the **HR console and PeopleSoft sync land in phase 2 with Entra**. If HR needs to use the console at v1, **Entra must move back into v1 scope** (Phase 2 slice 2.5 below). → **DECISION D2.**

---

## Current-State Findings (from codebase audit)

**Seams (7):** all follow `PROVIDER` env (default `mock`) → fail-fast validation → `New*(cfg)` factory.
- **AI is bundled:** `AI_PROVIDER=azure` flips OCR (Doc Intelligence) + parser (OpenAI) + the scoring LLM together (`internal/ai/factory.go`, `internal/scoring/scorer.go:36`). All 4 Azure keys required at once.
- **Flag-value gotcha:** AI + Search use the value `azure`; Auth/PS/LINE/Notify use `real`. `AI_PROVIDER=real` would silently fall back to mock.
- **Blob has NO flag** — Azurite vs real Azure differ only by `AZURE_BLOB_CONNECTION_STRING`; nothing prevents shipping prod with the dev Azurite string. (Add a guard — slice 2.2.)
- **Runbooks:** only `docs/azure-openai-provisioning.md` exists; 6 of 7 seams lack a provisioning runbook.

**Prod readiness:** Dockerfile is prod-grade (multi-stage, non-root, distroless-ish). Frontend security headers/CSP are **READY** (`unsafe-inline` CSP is deliberate/documented). GAPs: no prod deploy artifacts, CORS defaults to localhost, no metrics/tracing/error-tracking, migrations are manual (`migrate@latest` unpinned), no secret manager, scheduler **single-replica is load-bearing** (multi-instance double-enqueues report export + retention sweep).

**Product:** F01–F17 delivered (career portal, AI pipeline, HR dashboard, analytics, re-engagement, reports, search, PDPA, PWA). Descoped: F11 custom per-req criteria, manual merge UI, profile-edit, full users CRUD. Real-provider paths (Azure AI accuracy + ≤60s SLA, LINE LIFF, notifications, AI Search) have **no automated coverage** → manual UAT.

---

## Open Decisions (resolve in Phase 0 — cheap, unblock everything)

| ID | Decision | Why it blocks | Owner |
|----|----------|---------------|-------|
| **D1** | **Hired-candidate retention** — does hired PII stay in the ATS beyond 1 year? (carried from #17) | Sweep currently retains hired; client sign-off needed for compliance | Client/DPO |
| **D2** | **HR console at v1?** Entra is deferred → HR console fails closed in prod. Confirm career-portal-first, or pull Entra (2.5) into v1. | Reshapes go-live scope + cutover | Client + you |
| **D3** | **Descoped features** (F11 custom criteria, manual merge, profile-edit) — confirmed OUT of v1 UAT scope? | Sets UAT acceptance boundary | Client |
| **D4** | **Optional security hardening** for v1: nonce CSP? PS-webhook replay protection? | In/out of Phase 3 security pass | You |
| **D5** | **Data seeding** — which stores/JDs/positions load to prod at cutover (the 169-store / ~70-JD CSV import)? | Cutover runbook input | Client + you |

---

## Phase 1 — Production Infrastructure on ACA *(mock-compatible; start now, parallel to provisioning)*

Goal: a deployable, observable, secret-managed prod-shaped environment that runs the current (mock) app on ACA. Everything here is independent of flipping seams.

| Slice | Deliverable | Notes / gotchas |
|-------|-------------|-----------------|
| **1.1 Container images → ACR** | Build + push `api`/`worker`/`scheduler` (one image, `SVC` arg) and **two frontend images** to Azure Container Registry; GitHub Actions deploy workflow | Backend Dockerfile exists. Add `frontend`/`career-portal` Dockerfiles with `output: 'standalone'` (decide) and **build-time `NEXT_PUBLIC_API_URL`** = real API origin (it's baked, not runtime). |
| **1.2 ACA apps + ingress** | 3 backend Container Apps + 2 frontend apps; external ingress on api + both frontends; internal-only worker; **scheduler pinned `minReplicas=maxReplicas=1`** | The single-replica scheduler constraint is **load-bearing** (double-enqueue otherwise). ACA scale rules must not scale it. Wire `/health` to ACA liveness/readiness probes (api + worker). |
| **1.3 Managed data plane** | Azure DB for PostgreSQL Flexible Server + Azure Cache for Redis; **TLS (`sslmode=require`)** | Replace `sslmode=disable`. Confirm Redis is shared by asynq + rate limiter; size accordingly. |
| **1.4 Secrets via Key Vault + Managed Identity** | All secrets (JWT, Azure keys, LINE token, search key, blob conn) from Key Vault; **fail-fast hardening**: `JWT_SECRET` must FAIL (not warn) when `ENV!=development`; CORS must reject the localhost default in non-dev | Currently `JWT_SECRET` only warns (`config.go:198-200`); CORS defaults localhost (`config.go:158`). Small Go change + Key Vault refs in ACA. |
| **1.5 Automated migrations** | Pre-deploy **ACA Job** (or release step) running `migrate up` against `DB_URL`, **pinned migrate version** (not `@latest`); ordered migrate-then-rollout | App binaries don't self-migrate (good). No automated story today. |
| **1.6 Observability** | Error tracking + metrics/log pipeline — **Azure Application Insights** (Azure-native) or Sentry; ship zerolog → Log Analytics | `/health` already solid. Add at least error tracking + request metrics before go-live. |
| **1.7 Trusted-proxy for rate limiter** | Fiber `EnableTrustedProxyCheck: true` + `TrustedProxies` = ACA ingress CIDR + `ProxyHeader: X-Forwarded-For` so `c.IP()` = real client, not the ingress | **Resolves the HIGH follow-up from #18.** Do NOT trust `X-Forwarded-For` without the allowlist (spoofable bypass). ACA ingress provides a known proxy hop. |
| **1.8 Domains / TLS / CORS / CSP** | Custom domains + managed certs for HR console + portal + API; set prod `CORS_ALLOW_ORIGINS`; frontend CSP `connect-src` → real API origin | Headers/CSP already prod-shaped; just wire real origins. |

**Phase 1 exit:** the mock app runs on ACA staging at real domains over HTTPS, migrated, secret-managed, observable, with correct client-IP rate limiting. No real integrations yet.

---

## Phase 2 — Provision + Flip Real Seams *(each slice: provision → write runbook → flip flag in staging → smoke test)*

Pattern per slice mirrors `docs/azure-openai-provisioning.md`: provision the Azure/3rd-party resource, document it, set the env flag in **staging**, run an independent smoke test, then the relevant UAT journey.

| Slice | Seam | Flag(s) → value | Required env | Validation |
|-------|------|-----------------|--------------|------------|
| **2.1 Azure OpenAI + Doc Intelligence** | AI (bundled) | `AI_PROVIDER=azure` | `AZURE_OPENAI_ENDPOINT/KEY` (+ `DEPLOYMENT=hr-screening-gpt4o`) **and** `AZURE_DOC_INTEL_ENDPOINT/KEY` | OpenAI runbook exists; **write the Doc Intelligence runbook** (prebuilt-layout, api-version `2024-11-30`). Validate parse+OCR+score accuracy and the **≤60s pipeline SLA** on real CVs (pdf/docx/image). |
| **2.2 Real Azure Blob** | Blob (no flag) | swap `AZURE_BLOB_CONNECTION_STRING` to a real Storage Account | conn string | **Add a guard:** fail-fast if the Azurite dev account string is set when `ENV!=development`. Verify resume upload + 15-min SAS signed URLs. |
| **2.3 LINE Login + Notifications** | LINE + Notify | `LINE_PROVIDER=real` + `NOTIFY_PROVIDER=real` | `LINE_CHANNEL_ID`, `NOTIFY_LINE_TOKEN` (+ `NOTIFY_EMAIL_FROM`) | **Write a LINE/Notify runbook.** Validate LIFF id-token verification in the **LINE in-app browser**, real LINE push + email delivery (re-engagement, report links). Note: `NOTIFY_EMAIL_FROM` is read but not fail-fast — verify manually. |
| **2.4 Azure AI Search** | Search | `AI_SEARCH_PROVIDER=azure` | `AZURE_SEARCH_ENDPOINT/KEY` (+ `INDEX=candidates`) | Search code is **query-only** — this slice must also build an **index-population path** (backfill existing candidates + keep it updated on intake). Validate relevance vs the trigram baseline. **Largest slice in Phase 2.** |

**Phase 2 exit:** staging runs all v1 real seams; each smoke test green; runbooks written for DocIntel, Blob, LINE/Notify, AI Search.

---

## Phase 3 — Staging UAT

| Slice | Deliverable |
|-------|-------------|
| **3.1 Staging environment** | Phase 1 ACA shape + Phase 2 real seams, isolated from prod; seeded reference data |
| **3.2 Scripted UAT** | Test scripts covering the 15 critical journeys, **prioritising the real-provider paths with no automated coverage**: real OCR/parse/scoring accuracy + variety (pdf/docx/image, low-confidence routing), LINE LIFF apply, notification delivery, AI Search relevance, PWA install/offline on real devices, public status flow, rate-limit behaviour behind ingress |
| **3.3 Performance + security pass** | Pipeline ≤60s SLA under real load; Lighthouse/PWA on devices; cross-browser/responsive (320–1920); resolve **D4** optional hardening (nonce CSP / PS replay) if in scope |
| **3.4 UAT sign-off** | Client acceptance against agreed criteria (D3 boundary); defect triage → fix slices |

**Phase 3 exit:** signed UAT, no open blockers, go/no-go inputs ready.

---

## Phase 4 — Go-Live Cutover

| Slice | Deliverable |
|-------|-------------|
| **4.1 Prod env config checklist** | All v1 flags real (`AI_PROVIDER=azure`, `AI_SEARCH_PROVIDER=azure`, `LINE_PROVIDER=real`, `NOTIFY_PROVIDER=real`, real Blob), `AUTH_PROVIDER`/`PS_PROVIDER` = mock (phase 2), all secrets in Key Vault, prod CORS, trusted-proxy on, retention policy per D1 |
| **4.2 Cutover runbook + rollback** | Migrate → deploy → data seed (D5: stores/JDs/positions import) → smoke → DNS cutover; documented rollback (image pin + DB restore point) |
| **4.3 Go/no-go + soft-launch window** | Final gate; monitored soft-launch (career-portal-first per D2); error-budget watch |

**Phase 4 exit:** v1 live (public career portal + real AI screening + LINE + notifications + AI Search); HR console + PeopleSoft tracked as phase 2.

---

## Phase 2-prime (post-v1) — deferred scope
- **2.5 Entra SSO** (`AUTH_PROVIDER=real`) — Azure AD login + 7-role claim mapping → unlocks the **HR console** in prod.
- **2.6 PeopleSoft live** (`PS_PROVIDER=real`) — real IB push/webhooks + HMAC + CSV-fallback validation.
- Descoped product features (F11 custom criteria, manual merge UI, profile-edit, users CRUD) as prioritised.

---

## Dependency Order (critical path)

```
Phase 0 decisions ─┬─> Phase 1 (ACA infra, mock)  ──┐
                   └─> Phase 2 provisioning (parallel)─┴─> Phase 3 staging UAT ─> Phase 4 cutover
```
Phase 1 and the Phase 2 Azure provisioning (resource creation has lead time — request quota early) run in **parallel**. Flipping a seam (2.x) only needs its resource + Phase 1's secret plumbing. AI (2.1) has the longest provisioning lead time → start first.

## Suggested slice sequence for `/prp-plan`
1. **1.4 + 1.7** (secrets fail-fast + trusted-proxy + CORS/JWT guards) — small Go changes, land first; resolves the #18 HIGH follow-up.
2. **2.2** (real Blob + Azurite guard) — smallest seam, builds the flip-and-validate muscle.
3. **1.1–1.3, 1.5, 1.6, 1.8** (ACA images/apps/data/migrations/observability/domains) — the infra bulk.
4. **2.1** (Azure OpenAI + DocIntel) — longest lead time, start provisioning in parallel with #3.
5. **2.3** (LINE + Notifications), **2.4** (Azure AI Search + index population).
6. **3.x** UAT, then **4.x** cutover.

## Risks
| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| HR console unusable at v1 (Entra deferred, mock fails closed) | High (by design) | High if HR needed at v1 | D2: confirm career-portal-first, or pull 2.5 into v1 |
| Scheduler scaled >1 on ACA → double-enqueue (report export + retention sweep) | Medium | High | Pin `minReplicas=maxReplicas=1`; document; alert on duplicate exports |
| Prod ships with Azurite blob conn string or localhost CORS/empty JWT | Medium | High | Slice 1.4 + 2.2 fail-fast guards in non-dev |
| Real Azure OpenAI misses ≤60s SLA or accuracy bar on noisy Thai CVs | Medium | High | 2.1 UAT with real CV variety; tune deployment/throughput; keep mock fallback path |
| Azure AI Search index population is bigger than a flag flip | High | Medium | Scope 2.4 explicitly as build-the-index, not just query; backfill + on-intake update |
| `AI_PROVIDER=real` typo silently runs mock in prod | Low | High | Slice 1.4: validate provider flag values at startup (azure vs real) |
| Manual/unpinned migrations cause drift | Medium | Medium | 1.5 automated ACA Job, pinned migrate version |
| Quota/lead time for Azure OpenAI access | Medium | High | Request access/quota in Phase 0; runbook already notes this |

## Notes
- Carries forward the merged Sprint 7 work (#16 HMAC, #17 PDPA sweep, #18 Redis rate limiter, #19 Playwright-CI). The trusted-proxy follow-up from #18 is **slice 1.7**; the hired-policy question from #17 is **D1**.
- `docs/azure-openai-provisioning.md` (untracked) should be committed as part of slice 2.1; Phase 2 adds runbooks for DocIntel, Blob, LINE/Notify, AI Search.
- This roadmap is the milestone map; **do not `/prp-implement` it directly** — run `/prp-plan` per slice in the suggested sequence.
