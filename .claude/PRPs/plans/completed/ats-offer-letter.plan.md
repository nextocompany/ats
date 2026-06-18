# Plan: ATS Slice 4 — Letter Generation (Module-3 item 3.3)

## Summary
Generate bilingual (Thai) PDF letters — an **interview invitation letter** and an **offer letter** — for a candidate, stored in blob and downloadable by both HR (dashboard) and the candidate (career-portal membership). Pure-Go PDF via `github.com/go-pdf/fpdf` (cached offline) with an embedded **Sarabun** TTF (OFL) for Thai glyphs. Mirrors the offer-management slice's structure and the reports/export "build bytes → blob.Upload → persist record → serve signed URL" pattern.

## User Story
As **HR** I want to generate an official interview-invitation or offer letter as a downloadable PDF that the candidate can also retrieve, so the hiring paper trail is consistent, bilingual, and self-service.

## Metadata
- **Complexity**: Large (~24 files; migration 000024; new `internal/letters` pkg + fpdf dep + 2 embedded fonts)
- **Branch**: `feat/ats-offer-letter` (stacked on `feat/ats-offer-management` / PR #83 → on #82)
- **Estimated Files**: ~24

## Design Decisions (LOCKED — user + survey)
1. **Both letter types**: interview invitation + offer letter.
2. **PDF = pure-Go `go-pdf/fpdf@v0.9.0`** (already in GOMODCACHE — adds offline, no network at build) + **embedded Sarabun Regular/Bold TTF** (OFL, committed under `internal/letters/fonts/` with `OFL.txt`). fpdf `AddUTF8FontFromBytes` registers the embedded bytes; core fonts are Latin-only so Thai requires this. No headless Chrome (keeps the minimal non-root Alpine image, `CGO_ENABLED=0`).
3. **Stored in blob + persisted record** (not on-demand): generate once → `blob.Upload` to `letters/{applicationId}-{type}.pdf` in the existing `resumes` container → upsert a `letters` row. Re-download serves the *same* artifact; regenerate overwrites (idempotent blob name). Audit + stable artifact (an offer letter must not silently differ from what the candidate received). Mirrors `reports.RecordExport`.
4. **Access = HR + candidate**, both via the existing `{url}` JSON + `SignedURLForStored` contract (the dashboard `ResumeViewer` already renders PDF-in-iframe; CSP `frame-src` already allows blob SAS hosts).
5. **HR write gate** = `super_admin`, `hr_manager`, `hr_staff`, `sgm` (the candidate-managing HR roles) — `letterWriteRoles` map. Candidate read = account-scoped (own applications only), via `candidateauth`.
6. **Preconditions**: interview letter needs an `Appointment` (status interview/interviewed) → 400 if none; offer letter needs an `Offer` that is `sent`/`accepted` → 400 if none.
7. **Letterhead company name** = new config `COMPANY_NAME` (default `"CP AXTRA"`) — no existing constant.

### NOT building
- Rich layout/branding (logo image), e-signature, letter versioning/history (one current letter per type, regenerate overwrites).
- Editable letter body/templates in UI (fixed bilingual template; HR sets the underlying offer/appointment data).
- Emailing the PDF as an attachment (candidate self-serves via portal; offer "sent" email already links the portal).

---

## Data Model — migration 000024
```sql
-- 000024_letters.up.sql
-- Generated PDF letters (Module-3 3.3): interview invitation + offer letter, one
-- current letter per (application, type). Stored in blob; this row is the audit
-- record + the handle for re-download. Additive.
CREATE TABLE IF NOT EXISTS letters (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    type           TEXT NOT NULL, -- interview | offer
    blob_url       TEXT NOT NULL,
    created_by     UUID REFERENCES users(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (application_id, type)
);
CREATE INDEX IF NOT EXISTS idx_letters_application ON letters (application_id);
```
```sql
-- 000024_letters.down.sql
DROP TABLE IF EXISTS letters;
```

---

## Patterns to Mirror
- **PDF-as-bytes → blob → persist → signed URL**: `reports/export_service.go` (BlobStore narrow iface = `Upload` + `SignedURLForStored`; persist before deliver; sign-on-read in list).
- **Serve `{url}` JSON for download**: `dashboard_handler.go` resume (`signer.SignedURLForStored(...)` → `httpx.OK(c, fiber.Map{"url":..., "expires_in_seconds":...})`); frontend `useResumeUrl` + `ResumeViewer` iframe; `next.config.ts` frame-src already allows SAS hosts.
- **HR handler + role allowlist + narrow store + RegisterRoutes**: `offer_handler.go`. Candidate handler + `candidateauth.CandidateFromCtx` + account-scope: `offer_candidate_handler.go` (no import cycle — candidateauth only imports pkg/*).
- **Repo tx/query + `applications: %w` wrap + account join chain**: `offer_repository.go` (`ListOffersByAccount` join `candidate_accounts→candidates→applications`).
- **go:embed**: standard; embed the two TTFs + register via fpdf `AddUTF8FontFromBytes`.
- **Frontend panel (role-gated, mutate, toasts, semantic tokens)**: `OfferPanel.tsx`; career-portal `/offers` page + `lib/auth.ts`/`queries.ts`/types + i18n parity.

---

## Files to Change

### Backend
| File | Action | Notes |
|---|---|---|
| `backend/go.mod`, `go.sum` | UPDATE | `go get github.com/go-pdf/fpdf@v0.9.0` (offline from cache) |
| `internal/letters/fonts/{Sarabun-Regular,Sarabun-Bold}.ttf`, `OFL.txt` | CREATE | already fetched; committed |
| `internal/letters/pdf.go` | CREATE | `LetterData` struct, `Render(data) ([]byte, error)`, `go:embed` fonts, fpdf bilingual template (interview vs offer body) |
| `internal/letters/pdf_test.go` | CREATE | renders non-empty `%PDF`-prefixed bytes for both types incl. Thai name |
| `backend/migrations/000024_letters.{up,down}.sql` | CREATE | letters table |
| `internal/applications/letter.go` | CREATE | `LetterInterview`/`LetterOffer` consts, `letterWriteRoles`+`canManageLetter`, `Letter`/`LetterView` structs, `validLetterType`, sentinel `ErrLetterPreconditions` |
| `internal/applications/letter_repository.go` | CREATE | `GatherLetterData(appID,type)` (join app+candidate+position+store+appointment/offer), `UpsertLetter`, `GetLettersByApplication`, `GetLetterByID`, `ListLettersByAccount` + Repository iface additions |
| `internal/applications/letter_handler.go` | CREATE | HR: `POST /applications/:id/letters` (body {type} → gather+render+upload+upsert), `GET /applications/:id/letters` (list w/ signed URLs), `GET /applications/:id/letters/:letterID` ({url}); role gate; injects letter renderer + BlobStore |
| `internal/applications/letter_candidate_handler.go` | CREATE | candidate: `GET /api/v1/public/auth/letters` (account-scoped list w/ signed URLs) |
| `internal/applications/letter_test.go` | CREATE | HR gate, missing-precondition 400, type validation, candidate account-scope |
| `backend/pkg/config/config.go` | UPDATE | `CompanyName` (env `COMPANY_NAME`, default "CP AXTRA") |
| `backend/cmd/api/main.go` | UPDATE | construct letters renderer + handlers; HR after authMW; candidate under `RequireCandidate` (origin-guarded `/auth` prefix) |

### Frontend — dashboard
| File | Action |
|---|---|
| `lib/types.ts` | `Letter`, `LetterType` |
| `lib/queries.ts` | `useLetters(appId)`, `useGenerateLetter(appId)`, `useLetterUrl()` (or list already returns urls) |
| `lib/roles.ts` | `LETTER_ROLES` + `canManageLetters` |
| `components/resume/LettersPanel.tsx` | generate interview/offer letter buttons (gated + precondition-aware) + list with "open" links |
| `app/(app)/applications/[id]/page.tsx` | render `<LettersPanel>` |
| `messages/{en,th}.json` | `letters.*` block (parity) |

### Frontend — career-portal
| File | Action |
|---|---|
| `lib/types.ts` | `Letter` |
| `lib/auth.ts` | `getMyLetters()` |
| `lib/queries.ts` | `useMyLetters()` |
| `app/offers/page.tsx` | add a "เอกสารของฉัน / My documents" section listing downloadable letters (both types) |
| `messages/{en,th}.json` | `offers.documents*` keys (parity) |

---

## Step-by-Step Tasks
1. **Add fpdf dep** — `GOPROXY=off go get github.com/go-pdf/fpdf@v0.9.0` (verified offline-OK). VALIDATE: `go build ./...`.
2. **internal/letters/pdf.go** — `LetterData{ Type, CompanyName, CandidateName, PositionTitle, StoreName, IssuedDate; Interview *struct{ ScheduledAt time.Time; DurationMin int; Mode, Location, JoinURL string }; Offer *struct{ Salary float64; StartDate time.Time; Terms string } }`. `Render` builds an A4 fpdf, `AddUTF8FontFromBytes("Sarabun","",regular)` + `("Sarabun","B",bold)`, `SetFont("Sarabun",...)`, writes letterhead (company), date, salutation (เรียน <name>), a type-specific Thai body, and a signature block. Return `pdf.Output(buf)` bytes. GOTCHA: register fonts from the embedded `[]byte` (not file path); set font BEFORE any `Cell`/`MultiCell`; use `MultiCell` for wrapping Thai. VALIDATE: pdf_test renders `%PDF` non-empty for both types.
3. **migration 000024** — letters table. VALIDATE: visual vs 000023.
4. **letter.go** — consts/roles/structs/validators/sentinel. MIRROR offer.go. VALIDATE: build.
5. **letter_repository.go** — `GatherLetterData` (one query joining applications→candidates→positions→stores, plus `FindAppointment`/`GetOfferByApplication` reuse for the type-specific block); `UpsertLetter` (INSERT … ON CONFLICT (application_id,type) DO UPDATE SET blob_url, created_by, created_at=NOW() RETURNING); `GetLettersByApplication`; `GetLetterByID`; `ListLettersByAccount` (account join chain). Add to Repository iface. MIRROR offer_repository.go. VALIDATE: build+vet.
6. **letter_handler.go (HR)** — `letterStore` narrow iface + `letterRenderer` iface (`Render(LetterData)([]byte,error)`) + `BlobStore` (`Upload`,`SignedURLForStored`). `POST`: parse id+type, scope, role gate, `GatherLetterData` (400 `ErrLetterPreconditions` if appointment/offer missing), `Render`, `Upload("letters/<id>-<type>.pdf", bytes, "application/pdf")`, `UpsertLetter`, return the record. `GET list`: sign each blob_url → `[]LetterView{...,url}`. `GET :letterID`: `{url}`. MIRROR offer_handler.go. VALIDATE: build+vet.
7. **letter_candidate_handler.go** — `GET /api/v1/public/auth/letters`: `CandidateFromCtx` → `ListLettersByAccount(acct.ID)` → sign each → list. MIRROR offer_candidate_handler.go. VALIDATE: build+vet.
8. **config** — `CompanyName: getenv("COMPANY_NAME","CP AXTRA")`. VALIDATE: build.
9. **main.go wiring** — `letterRenderer := letters.NewRenderer(cfg.CompanyName)`; `letterHandler := applications.NewLetterHandler(appRepo, letterRenderer, blobClient)`; `RegisterLetterRoutes`; candidate handler `NewLetterCandidateHandler(appRepo, blobClient)` + `RegisterCandidateLetterRoutes(app, h, RequireCandidate(...))`. GOTCHA: `blobClient` satisfies `Upload`+`SignedURLForStored` (same as reports/resume signer). VALIDATE: `go build ./cmd/...`.
10. **backend tests** — letter_test.go (fakes for letterStore + renderer + blob): HR gate 403; missing precondition 400; bad type 400; candidate account-scope (not-mine excluded); pdf_test for the renderer. VALIDATE: `go test ./internal/... -run 'Letter'`.
11. **Dashboard FE** — types/roles/queries/LettersPanel/detail wire/i18n. LettersPanel: show "Generate interview letter" enabled when status interview/interviewed; "Generate offer letter" when an offer exists (reuse `useOffer`); after generate, list letters with "เปิด/Open" linking the signed URL (new tab). Use `mutate`. VALIDATE: tsc/eslint/parity/build.
12. **career-portal FE** — types/auth/queries + documents section on /offers + i18n. List `useMyLetters()`; each → open signed URL. VALIDATE: tsc/parity/build.

## Validation Commands
```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd frontend && pnpm exec tsc --noEmit && pnpm exec eslint app components lib && pnpm exec next build
cd career-portal && pnpm exec tsc --noEmit && pnpm exec next build
node scripts/check-i18n-parity.mjs
```

## Acceptance Criteria
- [ ] HR (gated) generates interview + offer letters; missing preconditions → 400
- [ ] PDF renders Thai (Sarabun) correctly; stored in blob; record upserted
- [ ] HR + candidate download via signed URL (account-scoped for candidate)
- [ ] All validation green; i18n parity both apps; fpdf added offline

## Risks
| Risk | Mitigation |
|---|---|
| fpdf `AddUTF8FontFromBytes` API differs in v0.9.0 | verify method name at impl; fallback `AddUTF8Font` from a temp file written from embed |
| Thai line-wrapping/measure in fpdf | use `MultiCell` (auto-wrap); test renders non-empty |
| go get needs network | confirmed offline from GOMODCACHE (GOPROXY=off succeeded) |
| Font licensing | Sarabun = SIL OFL; `OFL.txt` committed alongside |
| blob container | reuse `resumes` container w/ `letters/` prefix (no new wiring), like ps-export/reports |

## Notes
- Stacked on #83 → #82. Deploy order when all merge: migrate 000022 → 000023 → 000024, then roll images. New dep means the API/worker image rebuild picks up fpdf.
- Next ATS slice after this: 3.8 Document/Onboarding, then 3.9 ATS Reports.
