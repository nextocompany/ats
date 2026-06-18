# Implementation Report: ATS Slice 3 — Offer Management (3.6)

## Summary
Full offer-management slice on branch `feat/ats-offer-management` (stacked on PR #82). HR composes/edits/sends an offer (salary, start date, terms, expiry) from an `offer`-stage application; the candidate accepts/declines via career-portal membership login. Accept → application `hired` + best-effort PeopleSoft push (first auto-firing of the existing `SyncHired` seam); decline → `rejected` with reason. Offer lifecycle (`draft→sent→accepted/declined/expired`) on a new `offers` table; expiry enforced lazily at respond time.

## Assessment vs Reality
| Metric | Plan | Actual |
|---|---|---|
| Complexity | Large (~24 files) | 26 files, +1607 |
| Confidence | single-pass | implemented single-pass, no design change |
| Import cycle risk (applications→candidateauth) | flagged | none — candidateauth only imports pkg/* |

## Tasks Completed
| # | Task | Status |
|---|---|---|
| 1 | Migration 000023 (offers table) | ✅ |
| 2 | offer.go (domain, consts, sentinels, validators, role gate) | ✅ |
| 3 | offer_repository.go (CRUD + RespondOffer tx + ListByAccount) + Repository iface | ✅ |
| 4 | offer_handler.go (HR: create/update/send/get; notify on send) | ✅ |
| 5 | offer_candidate_handler.go (membership: list/respond; PS push on accept) | ✅ |
| 6 | transitions.go doc (offer→hired endpoint-owned) | ✅ |
| 7 | notify.statusBody `offer` case | ✅ |
| 8 | main.go wiring (HR after authMW; candidate under RequireCandidate, origin-guarded) | ✅ |
| 9 | offer_test.go (11 tests) | ✅ |
| 10 | Dashboard: types/roles/queries/OfferPanel/detail wire/i18n | ✅ |
| 11 | career-portal: types/auth/queries/offers page/StatusCard/i18n | ✅ |

## Validation
| Check | Result |
|---|---|
| go build ./... + go vet | ✅ |
| go test ./... | ✅ (+11 offer tests, no regressions) |
| gofmt (my files) | ✅ |
| dashboard tsc / eslint / next build | ✅ |
| career-portal tsc / next build | ✅ |
| i18n parity | ✅ frontend 105, career-portal 30 (both th/en) |
| migration 000023 round-trip | ⚠️ operator (local Docker PG disk-full env) |

## Key implementation choices
- **Offer lifecycle on the `offers` table**, NOT new application statuses — keeps the funnel clean (`offer → hired/rejected`). Accept reuses the legacy-but-now-meaningful `hired` + `hired_at` (what `SyncHired` expects).
- **`RespondOffer` is one tx** with `FOR UPDATE OF o`, verifies `candidates.account_id == acct.ID` inside the tx (account-scope leak prevention → `ErrOfferNotFound`), checks `sent` + not-expired (`ErrOfferConflict`).
- **PS push best-effort on accept**: `h.hired.SyncHired` after the tx commits; failure logged, never fails the accept (matches `peoplesoft/service.go`).
- **Conflict status codes**: duplicate offer / not-editable / not-respondable → 409; not-owned → 404 (carried the 3.5 review lesson on clean status codes).
- **Frontend**: `mutate` not `mutateAsync`; OfferForm re-seeds via `key` remount (no setState-in-effect); career-portal has no sonner → inline `role="alert"` feedback.

## Deviations
- OfferPanel uses a `key`-remount to re-init the form when the draft is first created, instead of a sync `useEffect` (avoids the repo's `react-hooks/set-state-in-effect` error — same lesson as 3.5's reject fix).
- Added `interviewed` + `pending_approval` cases to career-portal `StatusCard` (gap surfaced by the survey) alongside `offer`.

## Next Steps
- [ ] `/code-review` (Go + TS reviewers)
- [ ] Commit + PR (stacked on #82); note: first auto-fire of `SyncHired`; migration 000023
- [ ] Operator: apply 000022 then 000023; both PRs merge in order
- [ ] Next ATS slice: 3.3 Interview/Offer Letter (PDF)
