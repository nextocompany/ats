# Implementation Report: ATS Slice 4 — Letter Generation (3.3)

## Summary
Bilingual (Thai) PDF letter generation on branch `feat/ats-offer-letter` (stacked on #83→#82). HR generates an **interview invitation** or **offer letter** as a PDF (pure-Go `go-pdf/fpdf` + embedded Sarabun TTF), stored in blob; both HR (dashboard) and the candidate (career-portal membership) download via signed URLs.

## Assessment vs Reality
| Metric | Plan | Actual |
|---|---|---|
| Complexity | Large (~24 files) | 31 files (incl. 2 fonts + OFL), +1409 lines |
| Confidence | single-pass | implemented single-pass |
| fpdf offline add | risk flagged | ✅ resolved from GOMODCACHE (GOPROXY=off) |
| Thai font sourcing | risk flagged | ✅ fetched Sarabun (OFL) via curl, committed |

## Tasks Completed
| # | Task | Status |
|---|---|---|
| 1 | Add `go-pdf/fpdf@v0.9.0` (offline) | ✅ |
| 2 | `internal/letters/pdf.go` renderer + embedded Sarabun + bilingual template | ✅ |
| 3 | Migration 000024 (letters table) | ✅ |
| 4 | `letter.go` domain (consts/roles/structs/sentinel) | ✅ |
| 5 | `letter_repository.go` (GatherLetterData + Upsert + lists) + Repository iface | ✅ |
| 6 | `letter_handler.go` (HR generate/list/download) | ✅ |
| 7 | `letter_candidate_handler.go` (account-scoped list) | ✅ |
| 8 | config `CompanyName` | ✅ |
| 9 | main.go wiring (HR + candidate, origin-guarded) | ✅ |
| 10 | backend tests (8 handler + 4 renderer) | ✅ |
| 11 | Dashboard: types/roles/queries/LettersPanel/wire/i18n | ✅ |
| 12 | career-portal: types/auth/queries/documents section/i18n | ✅ |

## Validation
| Check | Result |
|---|---|
| go build / vet / test | ✅ (+12 tests; renderer produces valid `%PDF` for Thai content) |
| gofmt | ✅ |
| dashboard tsc / eslint / next build | ✅ |
| career-portal tsc / next build | ✅ |
| i18n parity | ✅ frontend 115, career-portal 35 (both th/en) |
| migration 000024 round-trip | ⚠️ operator (local Docker PG disk-full) |

## Key choices
- **Pure-Go fpdf + embedded Sarabun (SIL OFL)** — no headless Chrome, works with the CGO-disabled minimal Alpine image; `AddUTF8FontFromBytes` registers the `go:embed`-ed TTF so Thai glyphs render. `OFL.txt` committed alongside the fonts.
- **Stored-in-blob + persisted record** (mirrors `reports.RecordExport`): generate once → `blob.Upload("letters/<id>-<type>.pdf")` (reuses the `resumes` container with a `letters/` prefix) → upsert a `letters` row (UNIQUE per application+type, regenerate overwrites). Re-download serves the same artifact via `SignedURLForStored`.
- **Both HR + candidate** use the existing `{url}` JSON + signed-URL contract (the dashboard already renders/opens blob SAS URLs; candidate list is account-scoped via `candidateauth`).
- **Preconditions enforced server-side**: interview letter needs an `Appointment`, offer letter needs a non-draft `Offer` → 400 (`ErrLetterPreconditions`).
- **Thai B.E. date formatting** + thousands-separated salary in the letter body.

## Deviations from Plan
- None of substance. `applications` imports `internal/letters` (one-directional; letters only imports fpdf — no cycle), so `GatherLetterData` returns `letters.LetterData` directly (avoids a redundant mapping struct).

## Issues Encountered
- `go get fpdf` needs network — resolved: the version was already in GOMODCACHE, `GOPROXY=off go get` succeeded.
- Thai TTF not present anywhere — fetched Sarabun Regular/Bold + OFL from the google/fonts repo (`curl -L`) and committed under `internal/letters/fonts/`.

## Next Steps
- [ ] `/code-review`
- [ ] Commit + PR (stacked on #83)
- [ ] Operator: apply migrations 000022 → 000023 → 000024, roll images (new fpdf dep picked up on rebuild)
- [ ] Next ATS slice: 3.8 Document/Onboarding, then 3.9 ATS Reports
