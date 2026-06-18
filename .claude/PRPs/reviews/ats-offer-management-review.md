# Code Review: ATS Offer Management (feat/ats-offer-management)

**Reviewed**: 2026-06-18
**Branch**: feat/ats-offer-management → main (stacked on PR #82)
**Mode**: Local (uncommitted) · go-reviewer + typescript-reviewer + synthesis
**Decision**: ✅ APPROVE (all CRITICAL/HIGH + MEDIUM addressed; remaining LOWs fixed or accepted)

## Summary
Both reviewers initially returned BLOCK/WARN. Three HIGH issues (Go: terms NULL scan, RespondOffer app-row divergence; TS: date off-by-one) plus all MEDIUMs are fixed and re-validated. Lower items fixed or accepted with rationale.

## Backend (Go)
| Sev | Finding | Resolution |
|---|---|---|
| HIGH | `offerColumns` selects `terms` without COALESCE → pgx NULL-scan crash on any offer with empty terms (every read path) | **Fixed**: `COALESCE(terms,'')` in `offerColumns` + `ListOffersByAccount` |
| HIGH | `RespondOffer` locks only the offer row; concurrent HR reject + candidate accept diverge app/offer status | **Fixed**: `UPDATE applications … WHERE id=$1 AND status='offer'` + `RowsAffected()==0 → ErrOfferConflict` (rollback) on both accept & decline |
| MED | `'expired'` in `ListOffersByAccount` IN-list is dead code (never persisted) | **Fixed**: removed + comment (sent-past-expiry computed client-side) |
| MED | Send TOCTOU: validate on stale read | **Fixed**: `SendOffer` UPDATE backstop `AND salary IS NOT NULL AND salary>0 AND start_date IS NOT NULL` (handler keeps friendly 400) |
| MED | `start_date` DATE column ← `*time.Time`, timezone truncation risk | **Accepted** + doc: frontend sends UTC-midnight `…T00:00:00Z`, so the DATE stores the intended day |
| LOW | post-commit `GetOfferByID` re-read | **Accepted**: matches the approval-slice pattern (re-read after commit) |
| LOW | HR GET offer open to any authed HR role | **Accepted**: uniform RBAC; flagged for a future read-only role |
| LOW | decline reason is user text → downstream escaping | **Accepted**: React escapes by default (no dangerouslySetInnerHTML) |
| LOW | test style; missing Update gate test | **Fixed**: added `TestUpdateOffer_RoleGate` |

## Frontend (TS/React)
| Sev | Finding | Resolution |
|---|---|---|
| HIGH | `toDateInput` / display convert through UTC → off-by-one date for non-UTC viewers | **Fixed**: `iso.slice(0,10)` for inputs; `fmtDate`/`formatThaiDate` render with `timeZone:"UTC"` (matches UTC-midnight storage) |
| MED | unsound `t(\`status_${s}\` as "status_sent")` cast (both files) | **Fixed**: typed `STATUS_KEY: Record<OfferStatus,…>` map + fallback (OfferPanel + offers page) |
| MED | career-portal accept/decline buttons: no pending indicator | **Fixed**: `<Loader2 animate-spin>` on accept + confirm-decline |
| MED | `useMyOffers` error state not surfaced → infinite spinner on fetch failure | **Fixed**: destructure `isError`, render `offers.loadFailed` (both locales) |
| LOW | hardcoded Thai salary placeholder | **Fixed**: `offer.salaryPlaceholder` key (both locales) |
| LOW | number input missing `step` | **Fixed**: `step={1}` |
| LOW | `expires_at` stored UTC-midnight = 07:00 Bangkok | **Accepted**: product note (respond enforces via server time check) |
| LOW | `toLocaleString()` no locale | **Fixed**: `toLocaleString("th-TH",{maximumFractionDigits:0})` both surfaces |

i18n key-presence audit (both reviewers): **PASS** — all `offer.*` (dashboard) and `offers.*` (career-portal) keys present in both locales; both apps use `useTranslations` (client). React Query invalidation chain sound.

## Validation (post-fix)
| Check | Result |
|---|---|
| go build / vet / test | ✅ (+1 test; no regressions) |
| gofmt (my files) | ✅ |
| dashboard tsc / eslint / next build | ✅ |
| career-portal tsc / next build | ✅ |
| i18n parity | ✅ frontend 106, career-portal 31 (both th/en) |
| migration 000023 round-trip | ⚠️ operator (local Docker PG disk-full) |

## Files Reviewed
All 26 changed files (backend 11, dashboard 6, career-portal 6, + 3 .claude artifacts).
