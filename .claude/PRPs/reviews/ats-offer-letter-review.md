# Code Review: ATS Letter Generation (feat/ats-offer-letter)

**Reviewed**: 2026-06-19
**Branch**: feat/ats-offer-letter → (stacked on #83 → #82)
**Mode**: Local (uncommitted) · go-reviewer + typescript-reviewer + synthesis
**Decision**: ✅ APPROVE (Go BLOCK→resolved: both HIGH + all MEDIUM fixed; TS APPROVE: 2 MEDIUM fixed)

## Summary
go-reviewer returned BLOCK on two HIGH (untested Download cross-app guard; `fpdf` mis-marked indirect); typescript-reviewer returned APPROVE with two MEDIUM (shared spinner; silent docs error). All HIGH + MEDIUM fixed and re-validated; LOWs fixed or accepted.

## Backend (Go)
| Sev | Finding | Resolution |
|---|---|---|
| HIGH | Download endpoint + the cross-application guard (`letter.ApplicationID != id`) untested (fake always returned nil → 404) | **Fixed**: `fakeLetterRepo.byID` configurable + `TestDownloadLetter_HappyPath` / `_CrossApplicationDenied` / `_NotFound` |
| HIGH | `go-pdf/fpdf` marked `// indirect` in go.mod (imported directly) | **Fixed**: `go mod tidy` (offline) → promoted to direct require |
| MED | `humanizeTHB` corrupts negative input (`-,000`) | **Fixed**: sign handling + `-1000` test case |
| MED | `view()`/`ListMine` swallow `SignedURLForStored` errors into empty URL | **Fixed**: `log.Warn` on signing failure (both HR + candidate) |
| MED | No DB `CHECK` on `letters.type` | **Fixed**: `CHECK (type IN ('interview','offer'))` in migration 000024 |
| LOW | `uuid.Parse(u.ID)` silent fallback to nil UUID for `created_by` | **Fixed**: returns 400 on invalid actor id |
| LOW | `Render` accepts `Type=interview` with nil `Interview` → sparse letter | **Fixed**: `Render` errors when the type's details block is nil |
| LOW | zero `StartDate` indistinguishable from unset | **Accepted**: renderer omits via `IsZero()`; documented |

## Frontend (TS/React)
| Sev | Finding | Resolution |
|---|---|---|
| MED | Both generate buttons share `generate.isPending` → both spin | **Fixed**: spinner keyed on `generate.variables` (only the in-flight type spins) |
| MED | career-portal `DocumentsSection` silent on fetch error | **Fixed**: `isError` fallback + `offers.documentsFailed` (both locales) |
| LOW | `t` prop typed as unscoped `ReturnType<typeof useTranslations>` | **Accepted**: scoped instance passed at runtime; generic-typeof syntax risks tsc churn |
| Obs | Panel could show buttons at early stages | **Clarified**: gating hides the panel pre-interview-band when no letters exist; added a comment that the server owns precondition enforcement |

i18n key-presence audit (both reviewers): **PASS** — all `letters.*` (dashboard, 9 keys) and the new `offers.*` document keys (career-portal) present in both locales; both client components use `useTranslations` (DocumentsSection consumes the scoped `t` via prop). React Query invalidation sound; `mutate` (not `mutateAsync`) throughout. Download link uses `rel="noopener noreferrer"`.

## Validation (post-fix)
| Check | Result |
|---|---|
| go build / vet / test | ✅ (+5 tests; renderer `%PDF` for Thai content) |
| gofmt | ✅ |
| dashboard tsc / eslint / next build | ✅ |
| career-portal tsc / next build | ✅ |
| i18n parity | ✅ frontend 115, career-portal 36 |
| migration 000024 round-trip | ⚠️ operator (local Docker PG disk-full) |

## Files Reviewed
All changed files (backend 14 incl. fonts/migration, dashboard 6, career-portal 5, + .claude artifacts).
