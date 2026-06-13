# Plan: Career Portal Candidate Membership (Signup / Login / Saved Profile + Resume → Fast Apply)

## Summary
Add a persistent **candidate account** to the public career portal. A visitor signs up via **LINE / Google / Email-OTP**, fills a short profile (name, phone, LINE id, province, …), uploads a resume **once**, and gets a logged-in session that persists across pages and days (httpOnly cookie). Applying to any position becomes **account-first**: the apply form is **prefilled** from the saved profile and the candidate can submit with the **saved resume in one tap**. Email/Google signups can later **link LINE** so push notifications reach them.

## User Story
As a **job seeker on the CP Axtra career portal**,
I want to **register once (with LINE, Google, or email), save my profile and resume, and stay logged in**,
So that I can **apply to any job in seconds without re-entering my details or re-uploading my resume every time**.

## Problem → Solution
**Current:** The portal is fully **stateless for candidates**. Every apply runs a one-time LINE OAuth gate (id-token in the URL fragment), collects name/phone/resume from scratch, and creates a **new** `candidates` row per application. Nothing is remembered; nothing is reusable.
**Desired:** A persistent **candidate identity** (account + httpOnly session) with multi-provider signup, a saved profile + a saved resume, and an account-first apply flow that prefills and supports one-tap "apply with saved resume."

## Metadata
- **Complexity**: **XL** (new auth subsystem, sessions, 3 identity providers, email infra, account-first apply rewrite). ~30–40 files. **Recommend slicing into the 5 build phases (A–E) defined below** — each phase is independently shippable behind config defaults (mock).
- **Source PRD**: N/A (free-form request, Thai)
- **PRD Phase**: N/A
- **Estimated Files**: ~22 new, ~10 modified

### Locked product decisions (from requester, 2026-06-13)
1. **Email auth = OTP code via email (passwordless).** No passwords anywhere.
2. **Session = httpOnly cookie** (persists across pages/days; revocable).
3. **Apply flow = account-first.** Login/signup is required before applying; the form prefills from the account.
4. **Email delivery = Azure Communication Services (ACS) Email.**

---

## UX Design

### Before
```
Jobs ─▶ Job detail ─▶ Apply page
                         │
                         ├─ LineGate (verify with LINE)  ◀── per-apply, every time
                         └─ ApplyStepper: consent ▸ details (typed fresh) ▸ resume (uploaded fresh) ▸ submit
                                                                       │
                                                                 status_token (paste to /status)
```

### After
```
First visit:  Signup ─▶ choose method ─▶ profile (prefilled where possible) ─▶ upload resume ─▶ ✅ account + session cookie
              [LINE]  ▶ name/phone/province (LINE id auto)            ┐
              [Google]▶ name/phone/LINE id?/province (+ "Link LINE")  ├─▶ save resume ─▶ logged in
              [Email] ▶ enter email ▶ OTP ▶ name/phone/LINE id?/...   ┘

Returning:    Login ─▶ method ─▶ (LINE/Google one-click | Email→OTP) ─▶ logged in (cookie restored)

Apply (account-first):
  Jobs ─▶ Job detail ─▶ Apply
                         │  not logged in ─▶ redirect /login?return=/jobs/:id/apply
                         └  logged in ─▶ Review prefilled details
                                          ├─ [Apply with saved resume]  ← one tap (quick apply)
                                          └─ [Edit details / upload different resume] ─▶ submit
                                                                       │
                                                                 status_token
```

### Interaction Changes
| Touchpoint | Before | After | Notes |
|---|---|---|---|
| Auth | LINE id-token in URL fragment, per apply | httpOnly session cookie, persistent | `credentials:'include'` on all portal fetches |
| Identity providers | LINE only | LINE + Google + Email-OTP | All behind mock-default config seams |
| Apply entry | Anyone reaches the form | Redirect to `/login?return=…` if not authed | Account-first |
| Details step | Typed every time | Prefilled from account; editable | |
| Resume | Uploaded every apply | Uploaded once at signup; reused on apply | "Apply with saved resume" |
| LINE for push | Captured at apply | Captured at LINE signup, or via "Link LINE" for email/Google accounts | Keeps push working |
| Header | No account UI | Login / account menu / logout | |

---

## Mandatory Reading

| Priority | File | Lines | Why |
|---|---|---|---|
| P0 | `backend/internal/lineauth/lineauth.go` | 1–217 | **The exact pattern to mirror** for Google OAuth + the LINE→session change: authorize/callback, code→token `exchange`, CSRF state cookie, `safeReturn` open-redirect guard, mock bounce, secure-cookie selection. |
| P0 | `backend/pkg/config/config.go` | 15–268 | Config struct + `Load()` + provider validation pattern. New keys (Google, ACS email, session, OTP) go here following `UsesRealLINE()`-style gates. |
| P0 | `backend/cmd/api/main.go` | 162–251 | Wiring: repositories, integration seams (`auth.NewVerifier`, `notify.NewNotifier`), route registration order, the `/api/v1/public` rate limiter, CORS `AllowCredentials:true`. |
| P0 | `backend/internal/interview/repository.go` | 1–208 | **Token + optimistic-lock + sentinel-error + `pgconn` unique-violation** patterns. Mirror for `candidate_sessions` / `email_otps` repositories and opaque-token generation. |
| P0 | `backend/internal/applications/service.go` | 1–138 | `Intake` is the existing pipeline (candidate+application+blob+enqueue). Quick-apply reuses this verbatim — just feeds it the saved resume bytes + account profile. |
| P0 | `backend/internal/public/handler.go` | 1–185 | Public apply handler + `newPublicToken()` + PDPA consent recording. Account-first apply and quick-apply mirror this. |
| P1 | `backend/internal/candidates/repository.go` | 1–137 | Repository pattern: interface, `pgRepository`, `nullable()`, `COALESCE` reads, error wrapping. Mirror for `candidateauth` repo. |
| P1 | `backend/internal/candidates/model.go` | 1–37 | Struct/JSON-tag conventions. |
| P1 | `backend/internal/auth/line.go` | 1–78 | `Verifier` seam (mock/real). Reused to verify LINE id_token inside the new session-issuing callback. Mirror its mock/real shape for Google. |
| P1 | `backend/internal/middleware/auth.go` | 1–60 | HR auth middleware + `isUnauthedPath`. The new **candidate** session middleware is separate (cookie-based, on `/api/v1/public/auth` + quick-apply) and must NOT collide with HR Entra auth. |
| P1 | `backend/migrations/000012_interview_sessions.up.sql` | 1–29 | **Migration DDL convention** (numbering `000013_…`, `CREATE TABLE IF NOT EXISTS`, `gen_random_uuid()`, `TIMESTAMPTZ`, indexes, paired `.down.sql`). |
| P1 | `backend/pkg/blob/blob.go` | (Upload/Download/SignedURLForStored) | Saved-resume storage + `Download` for quick-apply; **shared-key HMAC signing** here is the template for ACS REST auth signing. |
| P0 | `career-portal/lib/api.ts` | 1–56 | Envelope client. **Must add `credentials:'include'`** to every fetch for the session cookie. |
| P0 | `career-portal/lib/queries.ts` | 1–92 | React Query hook conventions + `buildApplyForm`. New auth hooks mirror these. |
| P0 | `career-portal/components/ApplyStepper.tsx` | 1–254 | The apply form to convert to account-first + prefill + saved-resume. |
| P0 | `career-portal/app/jobs/[id]/apply/page.tsx` | 1–61 | The `useSyncExternalStore` hash pattern + LineGate gating to replace with a session gate. |
| P1 | `career-portal/components/LineGate.tsx` | 1–41 | LINE button styling/markup to reuse on the signup method chooser. |
| P1 | `career-portal/lib/line.ts` | 1–14 | `lineLoginUrl` builder; extend with `mode=link`/`mode=signup` and add `googleLoginUrl`. |
| P1 | `career-portal/app/providers.tsx` | 1–11 | Where to mount the `CandidateSessionProvider`. |
| P2 | `career-portal/app/globals.css` | (tokens) | OKLch tokens (`--primary`, `--gold`, dot motif) to keep new pages on-brand. |
| P2 | `career-portal/e2e/portal.spec.ts` | 1–73 | E2E apply flow to update for account-first + the new signup/login specs. |

## External Documentation

| Topic | Source | Key Takeaway |
|---|---|---|
| ACS Email (no Go SDK) | https://learn.microsoft.com/en-us/answers/questions/2263209/ | **No official Go SDK.** Call the REST API directly. |
| ACS Email REST send | https://learn.microsoft.com/en-us/azure/communication-services/quickstarts/email/send-email | `POST {endpoint}/emails:send?api-version=2023-03-31`, JSON body `{senderAddress, content:{subject,plainText,html}, recipients:{to:[{address}]}}`. Returns `202` + `Operation-Location` (async). Auth: `Communication-Services-Access-Key` HMAC-SHA256 over `date` + host + `x-ms-content-sha256` (same shared-key scheme as Azure Storage — mirror `pkg/blob`'s key signing), **or** AAD bearer scoped `https://communication.azure.com/.default`. |
| Google OAuth web-server flow | https://developers.google.com/identity/protocols/oauth2/web-server | Auth endpoint `https://accounts.google.com/o/oauth2/v2/auth`; token endpoint `https://oauth2.googleapis.com/token`; scope `openid email profile`; exchange returns `id_token` (JWT). Because the `id_token` is received **directly** from Google's token endpoint over TLS in the server-to-server exchange, its payload may be **decoded without re-verifying the signature** (Google's documented exception) — extract `email`, `email_verified`, `sub`, `name`. |

```
KEY_INSIGHT: ACS Email has no Go SDK → REST + Communication-Services-Access-Key HMAC.
APPLIES_TO: pkg/email/acs_sender.go
GOTCHA: send is async (202 + Operation-Location). For OTP we only need "accepted"; do NOT block on delivery polling. Treat 202 as success.

KEY_INSIGHT: Google OAuth is structurally identical to internal/lineauth.
APPLIES_TO: internal/candidateauth/google.go
GOTCHA: id_token from a direct token-endpoint exchange is trusted; decode payload, don't call tokeninfo per request. Require email_verified=true.

KEY_INSIGHT: portal (…apps.io subdomain) and api (different …apps.io subdomain) are CROSS-SITE because azurecontainerapps.io is on the Public Suffix List.
APPLIES_TO: candidate session cookie attributes + lib/api.ts
GOTCHA: a Lax/Strict cookie set by the api will NOT be sent on portal-originated fetches. The session cookie MUST be SameSite=None; Secure in prod, and every portal fetch MUST use credentials:'include'. (Long-term: move api+portal under one registrable custom domain to allow SameSite=Lax.)
```

---

## Patterns to Mirror

### NAMING_CONVENTION (Go package + DI constructor + small interface)
```go
// SOURCE: backend/internal/candidates/repository.go:14-36
type Repository interface {
	Create(ctx context.Context, c Candidate) (Candidate, error)
	FindByID(ctx context.Context, id uuid.UUID) (*Candidate, error)
	// ...
}
type pgRepository struct{ pool *pgxpool.Pool }
func NewRepository(pool *pgxpool.Pool) Repository { return &pgRepository{pool: pool} }
```

### CONFIG_SEAM (mock-default provider flag + fail-fast validation)
```go
// SOURCE: backend/pkg/config/config.go:167-171, 241-251, 273-282
LINEProvider:         getenv("LINE_PROVIDER", "mock"),
LINEChannelID:        os.Getenv("LINE_CHANNEL_ID"),
// ...
if c.UsesRealLINE() {
	if c.LINEChannelID == "" { return nil, fmt.Errorf("config: LINE_CHANNEL_ID is required when LINE_PROVIDER=real") }
	if c.LINEChannelSecret == "" || c.LINELoginCallbackURL == "" { return nil, fmt.Errorf("config: ... required when LINE_PROVIDER=real") }
}
func (c *Config) UsesRealLINE() bool { return c.LINEProvider == ProviderReal }
```

### OAUTH_HANDLER (authorize → callback → server-side exchange; CSRF state cookie; mock bounce; open-redirect guard)
```go
// SOURCE: backend/internal/lineauth/lineauth.go:73-139, 179-208
func (h *Handler) Login(c *fiber.Ctx) error {
	ret := h.safeReturn(c.Query("return"))
	if !h.real { return c.Redirect(ret+"#line_id_token="+devLineStub, fiber.StatusFound) }
	state, _ := randToken()
	c.Cookie(&fiber.Cookie{Name: stateCookie, Value: base64.RawURLEncoding.EncodeToString([]byte(state+"\x00"+ret)),
		HTTPOnly: true, Secure: h.secureCookie, SameSite: "Lax", MaxAge: int(stateTTL.Seconds()), Path: "/"})
	q := url.Values{"response_type": {"code"}, "client_id": {h.channelID}, "redirect_uri": {h.callbackURL}, "state": {state}, "scope": {"openid profile"}}
	return c.Redirect(authorizeURL+"?"+q.Encode(), fiber.StatusFound)
}
// safeReturn rejects any return URL not on the portal origin (open-redirect guard).
```

### OPAQUE_TOKEN (URL-safe, never a UUID)
```go
// SOURCE: backend/internal/public/handler.go:177-184
func newPublicToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil { return "", fiber.NewError(fiber.StatusInternalServerError, "token generation failed") }
	return base64.RawURLEncoding.EncodeToString(b), nil
}
```

### REPOSITORY_PATTERN (nullable inserts, COALESCE reads, sentinel errors, pgconn unique-violation, error wrap)
```go
// SOURCE: backend/internal/candidates/repository.go:38-61 + interview/repository.go:29-30,67-81
func nullable(s string) *string { if s == "" { return nil }; return &s }
// ...
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation { return nil, ErrAlreadyExists }
if err != nil { return Candidate{}, fmt.Errorf("candidates: create: %w", err) }
```

### SERVICE_PATTERN (constructor DI, validate-first, structured logging)
```go
// SOURCE: backend/internal/applications/service.go:57-86, 131-135
func NewService(c candidates.Repository, a Repository, b BlobUploader, q Enqueuer) *Service { return &Service{...} }
func (s *Service) Intake(ctx context.Context, in IntakeInput) (IntakeResult, error) {
	if in.CandidateName == "" { return IntakeResult{}, fmt.Errorf("intake: candidate name is required") }
	cand, err := s.candidates.Create(ctx, candidates.Candidate{ FullName: in.CandidateName, Phone: in.Phone, /*...*/ })
	// ... create app, upload blob, enqueue, log.Info()...
}
```

### NOTIFY_SEAM (mock/real selected by config — the template for pkg/email)
```go
// SOURCE: backend/internal/auth/line.go:29-45 (mirror exactly for EmailSender + GoogleVerifier)
func NewVerifier(cfg *config.Config) Verifier {
	if cfg.UsesRealLINE() { return realVerifier{channelID: cfg.LINEChannelID, http: &http.Client{Timeout: 10 * time.Second}} }
	return mockVerifier{}
}
type mockVerifier struct{}
func (mockVerifier) Verify(_ context.Context, idToken string) (LineUser, error) { /* deterministic */ }
```

### ROUTE_REGISTRATION (group + RegisterRoutes; wired in main.go after the public rate limiter)
```go
// SOURCE: backend/internal/lineauth/lineauth.go:65-70 + cmd/api/main.go:196-209
func RegisterRoutes(app *fiber.App, h *Handler) {
	g := app.Group("/api/v1/public/line")
	g.Get("/login", h.Login); g.Get("/callback", h.Callback)
}
// main.go: app.Use("/api/v1/public", limiter.New(...)); then RegisterRoutes(app, ...)
```

### MIGRATION_DDL
```sql
-- SOURCE: backend/migrations/000012_interview_sessions.up.sql:7-28
CREATE TABLE IF NOT EXISTS interview_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  UUID NOT NULL UNIQUE REFERENCES applications(id) ON DELETE CASCADE,
    access_token    TEXT NOT NULL UNIQUE,
    status          TEXT NOT NULL DEFAULT 'invited',
    version         INT  NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_interview_sessions_app ON interview_sessions (application_id);
```

### FRONTEND_API_CLIENT (envelope unwrap; add credentials for cookie)
```ts
// SOURCE: career-portal/lib/api.ts:31-55  (CHANGE: add credentials:'include' to each fetch)
export const api = {
  get: async <T>(path: string) => unwrap<T>(await fetch(`${BASE}${path}`, { credentials: "include" })),
  post: async <T>(path: string, body?: unknown) => unwrap<T>(await fetch(`${BASE}${path}`, {
    method: "POST", credentials: "include", headers: { "Content-Type": "application/json" },
    body: body !== undefined ? JSON.stringify(body) : undefined })),
  postForm: async <T>(path: string, form: FormData, headers?) => unwrap<T>(await fetch(`${BASE}${path}`, {
    method: "POST", credentials: "include", body: form, headers })),
};
```

### FRONTEND_QUERY_HOOK
```ts
// SOURCE: career-portal/lib/queries.ts:15-28, 46-55
export function usePublicPositions() {
  return useQuery({ queryKey: ["public-positions"], queryFn: () => api.get<PublicPosition[]>("/api/v1/public/positions").then(r => r.data) });
}
export function useApplyMutation() {
  return useMutation<ApplyResult, Error, ApplyInput>({ mutationFn: (input) => api.postForm<ApplyResult>("/api/v1/public/apply", buildApplyForm(input)).then(r => r.data) });
}
```

### FRONTEND_FORM_UI (shadcn Input/Label/Button size="tap", Thai copy, stepper, inline validation)
```tsx
// SOURCE: career-portal/components/ApplyStepper.tsx:146-192, 236-250
<ol className="flex items-center gap-2" aria-label="ขั้นตอนการสมัคร"> ... </ol>
<div className="space-y-2">
  <Label htmlFor="full_name">ชื่อ-นามสกุล <span className="text-destructive">*</span></Label>
  <Input id="full_name" value={fullName} onChange={(e) => setFullName(e.target.value)} autoComplete="name" />
</div>
<Button type="button" size="tap" onClick={...} disabled={...} className="flex-1">ถัดไป</Button>
```

---

## Files to Change

### Backend — new
| File | Action | Justification |
|---|---|---|
| `backend/pkg/email/email.go` | CREATE | `Sender` seam (`Send(ctx, EmailMessage) error`) + `NewSender(cfg)` mock/real. |
| `backend/pkg/email/acs_sender.go` | CREATE | ACS REST sender (HMAC shared-key signing mirrored from `pkg/blob`). |
| `backend/pkg/email/mock_sender.go` | CREATE | Logs the email (incl. OTP in dev) — local/CI need no ACS creds. |
| `backend/pkg/email/email_test.go` | CREATE | Table tests for message building + signer. |
| `backend/internal/candidateauth/model.go` | CREATE | `Account`, `Session`, `OTPChallenge` structs + status consts + sentinel errors. |
| `backend/internal/candidateauth/repository.go` | CREATE | Accounts/sessions/OTP data access (find-or-create by email/line_sub/google_sub; session issue/find/revoke; OTP store/consume). |
| `backend/internal/candidateauth/service.go` | CREATE | Orchestration: email-OTP start/verify, identity find-or-create, session issuance, profile update, resume save, identity linking. |
| `backend/internal/candidateauth/google.go` | CREATE | Google OAuth handler (mirror `lineauth`): login/callback/exchange → issue session. |
| `backend/internal/candidateauth/handler.go` | CREATE | HTTP: `POST email/start`, `POST email/verify`, `GET me`, `POST logout`, `PATCH profile`, `POST resume`, `POST link/line`(callback hook). |
| `backend/internal/candidateauth/middleware.go` | CREATE | `RequireCandidate` / `OptionalCandidate` Fiber middleware (reads session cookie → `c.Locals`). |
| `backend/internal/candidateauth/routes.go` | CREATE | `RegisterRoutes` under `/api/v1/public/auth` + the session-gated quick-apply. |
| `backend/internal/candidateauth/*_test.go` | CREATE | service/repo/otp/google-mock/middleware tests. |
| `backend/migrations/000013_candidate_accounts.up.sql` / `.down.sql` | CREATE | `candidate_accounts`, `candidate_sessions`, `email_otps`; `candidates.account_id` FK. |

### Backend — modify
| File | Action | Justification |
|---|---|---|
| `backend/pkg/config/config.go` | UPDATE | Add Google, ACS-email, session, OTP config + provider validation + `UsesRealGoogle()/UsesRealEmail()`. |
| `backend/internal/lineauth/lineauth.go` | UPDATE | Optional `SessionIssuer` (account-first): on callback verify id_token → find-or-create account → set session cookie → redirect clean (no fragment). Supports `mode=link` for adding LINE to an existing session. Legacy fragment path retained when issuer is nil. |
| `backend/internal/applications/service.go` | UPDATE | Add `BlobDownloader` + `IntakeFromAccount` (download saved resume bytes, reuse `Intake`) and stamp `account_id`. |
| `backend/internal/applications/repository.go` | UPDATE | Persist/scan `account_id` (nullable) on candidate/application link. |
| `backend/internal/candidates/{model,repository}.go` | UPDATE | Add nullable `AccountID` to `Create` so member applies link back to the account. |
| `backend/cmd/api/main.go` | UPDATE | Wire `pkg/email`, `candidateauth` (repo+service+handler+middleware+routes), pass `SessionIssuer` into `lineauth`, register quick-apply. |

### Frontend — new
| File | Action | Justification |
|---|---|---|
| `career-portal/lib/auth.ts` | CREATE | Auth API helpers: `emailStart/emailVerify`, `googleLoginUrl`, `lineLoginUrl(mode)`, `logout`, `updateProfile`, `uploadResume`, `quickApply`. |
| `career-portal/lib/session.tsx` | CREATE | `CandidateSessionProvider` + `useCandidate()` (React Query `useMe()` over `GET /auth/me`). |
| `career-portal/app/signup/page.tsx` | CREATE | Signup flow: method → profile → resume. |
| `career-portal/app/login/page.tsx` | CREATE | Login: method chooser + email-OTP, honors `?return=`. |
| `career-portal/app/account/page.tsx` | CREATE | View/edit saved profile + resume + "Link LINE" (session-gated). |
| `career-portal/components/auth/AuthMethods.tsx` | CREATE | LINE / Google / Email buttons (reuse LINE styling). |
| `career-portal/components/auth/EmailOtpForm.tsx` | CREATE | Email → 6-digit OTP entry. |
| `career-portal/components/auth/ProfileForm.tsx` | CREATE | Shared profile fields (name/phone/LINE id/province); reused in signup + account + apply prefill. |
| `career-portal/components/auth/ResumeUploadStep.tsx` | CREATE | Resume upload (extract from ApplyStepper step 2). |
| `career-portal/components/auth/LinkLineButton.tsx` | CREATE | "เชื่อมบัญชี LINE" for email/Google accounts. |
| `career-portal/e2e/signup.spec.ts`, `login.spec.ts` | CREATE | Mock-provider E2E for the three signup paths + login + link. |

### Frontend — modify
| File | Action | Justification |
|---|---|---|
| `career-portal/lib/api.ts` | UPDATE | `credentials:'include'` on every fetch (cookie). |
| `career-portal/lib/line.ts` | UPDATE | `lineLoginUrl(returnUrl, mode?)` + `googleLoginUrl(returnUrl)`. |
| `career-portal/app/providers.tsx` | UPDATE | Mount `CandidateSessionProvider`. |
| `career-portal/app/jobs/[id]/apply/page.tsx` | UPDATE | Account-first: redirect to `/login?return=…` when unauthenticated; render prefilled ApplyStepper otherwise. Remove the LineGate-before-apply. |
| `career-portal/components/ApplyStepper.tsx` | UPDATE | Prefill from account; drop the LINE-token step; add "Apply with saved resume" (quick apply) + "upload different resume". |
| `career-portal/components/SiteHeader.tsx` | UPDATE | Account menu (login / profile / logout). |
| `career-portal/lib/queries.ts` | UPDATE | `useApplyMutation` no longer needs the LINE header; add `useQuickApply`. |
| `career-portal/lib/types.ts` | UPDATE | `Account`, `MeResponse`, `EmailOtp*`, `QuickApply*` types. |
| `career-portal/e2e/portal.spec.ts` | UPDATE | Apply flow now signs up/logs in first. |
| `career-portal/next.config.ts` | UPDATE (if needed) | CSP `connect-src` already allows the API origin; confirm Google/LINE redirects are top-level navigations (no CSP change expected). |

## NOT Building
- **Passwords / username login** — passwordless (OTP) only, per decision.
- **HR-dashboard changes** beyond storing `account_id` — no new HR screens. (Members may show a small "registered" hint later; out of scope.)
- **Backfill** linking historical per-apply `candidates` rows to new accounts.
- **Account deletion / full PDPA self-service portal** — only signup-time consent + a saved profile. (Existing retention sweep still applies.)
- **SMS OTP**, social providers beyond LINE/Google, multi-resume libraries (one saved resume per account for v1).
- **Refresh-token rotation / device management** — single opaque session token with TTL + logout revoke.
- **Changing the per-application processing pipeline** (OCR/parse/score stays exactly as-is; quick-apply feeds the same `Intake`).

---

## Step-by-Step Tasks

> Build order = Phases **A → E**. Each phase keeps `go test -race ./...` green and ships behind mock defaults.

### PHASE A — Backend account + session + email-OTP

#### Task A1: Migration `000013_candidate_accounts`
- **ACTION**: Create `backend/migrations/000013_candidate_accounts.up.sql` and `.down.sql`.
- **IMPLEMENT**:
  ```sql
  CREATE TABLE IF NOT EXISTS candidate_accounts (
      id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      full_name       VARCHAR(255) NOT NULL DEFAULT '',
      email           VARCHAR(255) UNIQUE,            -- nullable; unique when present
      email_verified  BOOLEAN NOT NULL DEFAULT FALSE,
      phone           VARCHAR(20),
      line_user_id    TEXT UNIQUE,                    -- nullable; unique when present
      line_display_id VARCHAR(100),                   -- the @line id the user types (optional)
      google_sub      TEXT UNIQUE,                    -- nullable; unique when present
      province        VARCHAR(100),
      resume_blob_url TEXT,
      resume_file_type VARCHAR(10),                   -- pdf | docx | image
      pdpa_consent    BOOLEAN NOT NULL DEFAULT FALSE,
      pdpa_version    VARCHAR(10),
      pdpa_consent_at TIMESTAMPTZ,
      created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE TABLE IF NOT EXISTS candidate_sessions (
      id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      account_id  UUID NOT NULL REFERENCES candidate_accounts(id) ON DELETE CASCADE,
      token       TEXT NOT NULL UNIQUE,               -- opaque, hashed at rest (see A3)
      expires_at  TIMESTAMPTZ NOT NULL,
      revoked_at  TIMESTAMPTZ,
      created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX IF NOT EXISTS idx_candidate_sessions_account ON candidate_sessions (account_id);
  CREATE TABLE IF NOT EXISTS email_otps (
      id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
      email       VARCHAR(255) NOT NULL,
      code_hash   TEXT NOT NULL,                      -- sha256(code), never plaintext
      expires_at  TIMESTAMPTZ NOT NULL,
      consumed_at TIMESTAMPTZ,
      attempts    INT NOT NULL DEFAULT 0,
      created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  CREATE INDEX IF NOT EXISTS idx_email_otps_email ON email_otps (email);
  ALTER TABLE candidates ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES candidate_accounts(id);
  ```
  `.down.sql`: drop the `account_id` column, then the three tables (reverse order).
- **MIRROR**: MIGRATION_DDL.
- **GOTCHA**: Numbering is **strictly sequential** (`000013`). Prod `schema_migrations` has drifted before — keep up/down idempotent (`IF NOT EXISTS`). UNIQUE on a nullable column allows many NULLs (correct for accounts with only one identity provider).
- **VALIDATE**: `~/go/bin/migrate -path backend/migrations -database "$DBURL" up` locally against the docker Postgres; `\d candidate_accounts`.

#### Task A2: `pkg/email` seam (mock + ACS REST)
- **ACTION**: Create `pkg/email/email.go`, `mock_sender.go`, `acs_sender.go`, `email_test.go`.
- **IMPLEMENT**: `type EmailMessage struct{ To, Subject, PlainText, HTML string }`; `type Sender interface { Send(ctx, EmailMessage) error }`; `func NewSender(cfg *config.Config) Sender` → `acsSender` when `cfg.UsesRealEmail()` else `mockSender`. `acsSender.Send` POSTs to `{endpoint}/emails:send?api-version=2023-03-31`, signs with `Communication-Services-Access-Key` HMAC-SHA256 (port the shared-key signing from `pkg/blob`), treats **202** as success.
- **MIRROR**: NOTIFY_SEAM (mock/real), and `pkg/blob` shared-key signer for HMAC.
- **IMPORTS**: `crypto/hmac`, `crypto/sha256`, `encoding/base64`, `net/http`, `time`, `github.com/nexto/hr-ats/pkg/config`.
- **GOTCHA**: No Go SDK — REST only. Send is **async (202 + Operation-Location)**; do NOT poll for OTP. `mockSender` must log the OTP at debug so dev/E2E can read it (never in prod: gate the code log on `cfg.IsDevelopment()`).
- **VALIDATE**: `go test ./pkg/email/...`; signer unit test against a known vector.

#### Task A3: `candidateauth` model + repository
- **ACTION**: Create `internal/candidateauth/model.go` + `repository.go` (+ test).
- **IMPLEMENT**: structs `Account`, `Session`; sentinel errors `ErrNotFound`, `ErrAlreadyExists`, `ErrSessionExpired`, `ErrOTPInvalid`, `ErrOTPExpired`. Repo methods: `FindOrCreateByEmail`, `FindOrCreateByLineSub`, `FindOrCreateByGoogleSub`, `LinkLine(accountID, lineSub, displayID)`, `UpdateProfile`, `SetResume`, `GetByID`; sessions: `CreateSession(accountID, hash, expiresAt)`, `FindAccountBySessionHash`, `RevokeSession`; OTP: `CreateOTP`, `ConsumeOTP(email, codeHash)` (atomic: select valid+unconsumed, bump `attempts`, set `consumed_at`).
- **MIRROR**: REPOSITORY_PATTERN (nullable, COALESCE, `pgconn` unique-violation → `ErrAlreadyExists`, `fmt.Errorf("candidateauth: …: %w", err)`).
- **GOTCHA**: **Store only `sha256(token)`** in `candidate_sessions.token` and `sha256(code)` in `email_otps.code_hash` — never plaintext (DB-leak containment). Find-or-create must be race-safe: `INSERT … ON CONFLICT (email/line_user_id/google_sub) DO UPDATE … RETURNING` or insert-then-select-on-unique-violation.
- **VALIDATE**: `go test ./internal/candidateauth/...` (repo against docker PG, or in-memory fake per existing test style).

#### Task A4: `candidateauth` service (OTP + session) + email-OTP handler
- **ACTION**: Add `service.go`, `otp.go`, `handler.go`, `routes.go`, `middleware.go`.
- **IMPLEMENT**:
  - `otp.go`: `genCode()` → crypto/rand 6-digit; `hashCode/hashToken` → `sha256` hex.
  - `service.StartEmailOTP(ctx, email)`: validate email, gen code, store `sha256(code)` with `cfg.EmailOTPTTL`, `email.Send` (subject/body Thai+EN). Rate-limit per email (reuse the public limiter + a per-email attempt cap).
  - `service.VerifyEmailOTP(ctx, email, code)`: `ConsumeOTP`; on success `FindOrCreateByEmail` (set `email_verified=true`), `issueSession` → opaque token; return token + account.
  - `issueSession`: `randToken()` (24-byte, MIRROR OPAQUE_TOKEN), store `sha256`, `expires = now + cfg.CandidateSessionTTL`.
  - `handler.go`: `POST /api/v1/public/auth/email/start`, `POST /email/verify` (sets cookie via `setSessionCookie`), `GET /auth/me`, `POST /auth/logout` (revoke + clear cookie).
  - `setSessionCookie`: name `cfg.SessionCookieName`, `HTTPOnly:true`, `Secure: !dev`, `SameSite: "None" if !dev else "Lax"`, `MaxAge`, `Path:"/"`. **(see cross-site gotcha)**
  - `middleware.go`: `RequireCandidate` reads the cookie → `FindAccountBySessionHash` → `c.Locals(CandidateKey, account)` or 401; `OptionalCandidate` sets locals if present, never blocks.
  - `routes.go`: `RegisterRoutes(app, h, mw)`.
- **MIRROR**: SERVICE_PATTERN, OAUTH_HANDLER (cookie attrs), ROUTE_REGISTRATION.
- **GOTCHA**: These routes live under `/api/v1/public/auth` so they ride the existing public rate limiter and **bypass HR Entra auth** (`isUnauthedPath` already covers `/api/v1/public`). Cookie must be `SameSite=None; Secure` in prod (cross-site). Don't leak whether an email exists — `email/start` always returns 200.
- **VALIDATE**: `go test -race ./internal/candidateauth/...`; manual: `curl -i -X POST …/email/start` then read mock log code, `…/email/verify` → `Set-Cookie`, `…/auth/me` with the cookie returns the account.

#### Task A5: Wire Phase A in `main.go`
- **ACTION**: Construct `emailSender := email.NewSender(cfg)`, `caRepo := candidateauth.NewRepository(pool)`, `caSvc := candidateauth.NewService(caRepo, emailSender, blobClient, cfg)`, `caMW := candidateauth.RequireCandidate(caRepo, cfg)`, register routes after the public rate limiter.
- **MIRROR**: cmd/api/main.go:196-209 ordering.
- **VALIDATE**: `go build ./...`; `/health` 200; existing tests green.

### PHASE B — Google OAuth + LINE-as-account + Link LINE

#### Task B1: Config — Google + ACS email + session + OTP keys
- **ACTION**: Extend `config.go`.
- **IMPLEMENT**: add fields + `Load()` entries + validation:
  - `GoogleProvider` (`GOOGLE_PROVIDER`, mock), `GoogleClientID/Secret/CallbackURL`; `UsesRealGoogle()`; validate trio when real.
  - `EmailProvider` (`EMAIL_PROVIDER`, mock), `ACSEmailEndpoint`, `ACSEmailAccessKey`, `ACSEmailSender`; `UsesRealEmail()`; validate when real.
  - `CandidateSessionTTL` (`CANDIDATE_SESSION_TTL`, default `720h`), `SessionCookieName` (`CANDIDATE_SESSION_COOKIE`, default `cp_session`), `EmailOTPTTL` (`EMAIL_OTP_TTL`, default `10m`).
  - Add `GOOGLE_PROVIDER`/`EMAIL_PROVIDER` to the provider-allowlist loop (`{"mock", ProviderReal}`).
- **MIRROR**: CONFIG_SEAM.
- **GOTCHA**: Durations via a `getenvDuration` helper (add next to `getenvInt`). Keep validation fail-fast.
- **VALIDATE**: `go test ./pkg/config/...`; booting with `EMAIL_PROVIDER=real` and no key must error clearly.

#### Task B2: Google OAuth handler
- **ACTION**: `internal/candidateauth/google.go` + test.
- **IMPLEMENT**: Mirror `lineauth` precisely — `Login` (mock bounce → stub account; real → redirect to `accounts.google.com/o/oauth2/v2/auth`, scope `openid email profile`, state cookie), `Callback` (validate state, `exchange` code at `oauth2.googleapis.com/token`, **decode** id_token payload for `email`/`email_verified`/`sub`/`name`), then `svc.LoginWithGoogle(sub, email, name)` → `FindOrCreateByGoogleSub` → issue session cookie → redirect to clean `ret`. Errors → `ret#auth_error=…`.
- **MIRROR**: OAUTH_HANDLER, `auth/line.go` mock/real shape.
- **GOTCHA**: Require `email_verified == true`. id_token from the direct exchange is trusted (no per-request tokeninfo). Reuse `safeReturn` open-redirect guard.
- **VALIDATE**: `go test ./internal/candidateauth/...`; mock `…/auth/google/login` 302s back with a session cookie.

#### Task B3: LINE → account session + Link mode (modify `lineauth`)
- **ACTION**: Update `internal/lineauth/lineauth.go`; add a `SessionIssuer` interface + optional `verifier auth.Verifier`.
- **IMPLEMENT**: `NewHandler(cfg, issuer SessionIssuer, verifier auth.Verifier)`. When `issuer != nil` (account-first): in `Callback`, after `exchange`→idToken, `verifier.Verify`→`LineUser{Subject,Name,Email}`. If a valid session cookie is present and `mode=link` → `issuer.LinkLine(accountID, sub, displayID)`; else → `issuer.LoginWithLine(sub, name, email)` → set session cookie. Redirect to clean `ret` (no `#line_id_token`). When `issuer == nil`, behavior is unchanged (legacy fragment). Thread `mode` through the state cookie like `ret`.
- **MIRROR**: existing lineauth structure (don't duplicate exchange/secret handling).
- **IMPORTS**: `github.com/nexto/hr-ats/internal/auth`.
- **GOTCHA**: Keep the legacy fragment path compiling/passing (other code/tests may reference it). The `SessionIssuer` interface is **defined in lineauth** (consumer side) and satisfied by `candidateauth.Service` to avoid an import cycle.
- **VALIDATE**: `go test -race ./internal/lineauth/... ./internal/candidateauth/...`; mock LINE login sets a `cp_session` cookie.

#### Task B4: Link-LINE + profile + resume endpoints
- **ACTION**: Extend `candidateauth/handler.go` + `routes.go`.
- **IMPLEMENT** (all `RequireCandidate`): `PATCH /api/v1/public/auth/profile` (name/phone/line_display_id/province → `UpdateProfile`); `POST /api/v1/public/auth/resume` (multipart, same validation as apply: ≤10MB, pdf/docx/jpeg/png → blob `accounts/{accountID}/{filename}` → `SetResume`); the LINE `mode=link` entrypoint is just `lineLoginUrl(ret, "link")` reusing B3.
- **MIRROR**: public/handler.go:85-96 (file validation), blob Upload key convention.
- **GOTCHA**: Resume blob key namespace `accounts/…` (NOT `{appID}/…`) so it's account-scoped and reusable.
- **VALIDATE**: `go test ./internal/candidateauth/...`; upload + `auth/me` shows `resume_file_type`.

### PHASE C — Frontend session + signup/login

#### Task C1: Cookie-credentialed client + session context
- **ACTION**: Update `lib/api.ts` (credentials), create `lib/auth.ts`, `lib/session.tsx`, update `app/providers.tsx`, `lib/types.ts`.
- **IMPLEMENT**: add `credentials:'include'` (FRONTEND_API_CLIENT). `lib/auth.ts`: `emailStart`, `emailVerify`, `googleLoginUrl(ret)`, `lineLoginUrl(ret, mode?)`, `logout`, `updateProfile`, `uploadResume`, `quickApply`. `lib/session.tsx`: `useMe()` query over `GET /api/v1/public/auth/me` (`retry:false`, `staleTime:30_000`); `CandidateSessionProvider` + `useCandidate()` → `{ candidate, isLoading, isAuthenticated, refetch }`. Mount provider in `providers.tsx` inside `QueryClientProvider`.
- **MIRROR**: FRONTEND_QUERY_HOOK, providers.tsx.
- **GOTCHA**: `me` 401 is a normal "logged-out" state, not an error toast — map to `isAuthenticated:false`.
- **VALIDATE**: `npm run build` (or `pnpm`/`yarn` per repo); `useCandidate()` returns unauthenticated before login.

#### Task C2: Signup + Login pages + auth components
- **ACTION**: Create `app/signup/page.tsx`, `app/login/page.tsx`, `components/auth/{AuthMethods,EmailOtpForm,ProfileForm,ResumeUploadStep,LinkLineButton}.tsx`.
- **IMPLEMENT**: Signup stepper: **method** (`AuthMethods`: LINE/Google → full-page nav to backend login URL with `return=/signup/continue` or back to `/account`; Email → `EmailOtpForm`) → **profile** (`ProfileForm`) → **resume** (`ResumeUploadStep`) → done (redirect to `?return` or `/jobs`). Login: `AuthMethods` + `EmailOtpForm`; honor `?return=`. `LinkLineButton` (account page) → `lineLoginUrl(currentUrl, "link")`.
- **MIRROR**: FRONTEND_FORM_UI (shadcn `Input/Label/Button size="tap"`, Thai copy, stepper bar), LineGate markup for the LINE button.
- **GOTCHA**: LINE/Google are **top-level navigations** (`window.location.href`), not fetches — the session cookie is set by the backend redirect; on return, `useMe()` refetch picks it up. After email OTP verify (a fetch that sets the cookie), call `me` refetch.
- **VALIDATE**: dev server; complete all three signup paths against mock providers; cookie present; `/account` shows profile.

#### Task C3: Header account menu
- **ACTION**: Update `components/SiteHeader.tsx`.
- **IMPLEMENT**: when `isAuthenticated` show name + menu (โปรไฟล์ `/account`, ออกจากระบบ → `logout()` + refetch); else "เข้าสู่ระบบ" → `/login`.
- **MIRROR**: existing SiteHeader nav/CTA.
- **VALIDATE**: visual at 320/768/1440; logout clears state.

### PHASE D — Account-first apply + quick apply + prefill

#### Task D1: Backend quick-apply + account-linked intake
- **ACTION**: Update `applications/service.go` (+repo, candidates repo) and add the quick-apply handler/route (in `candidateauth` or `public`, session-gated).
- **IMPLEMENT**: add `BlobDownloader` (`Download(ctx, name) ([]byte, error)`) to the service; `IntakeFromAccount(ctx, acct, positionID)` → download saved resume bytes from `acct.ResumeBlobURL` (via `blob.SignedURLForStored`/`Download` by key), build `IntakeInput` from the account profile (`SourceChannel:"career_portal"`, `LineUserID: acct.LineUserID`, `AccountID: acct.ID`), call `Intake`. Stamp `account_id` on the candidate. New route `POST /api/v1/public/apply/quick` (`RequireCandidate`, body `{position_id}`) → record PDPA from the account's stored consent → return `{status_token}`. Keep the existing `POST /api/v1/public/apply` but allow session-cookie identity (account-first) in addition to the legacy LINE header during transition.
- **MIRROR**: applications/service.go Intake, public/handler.go Apply + `newPublicToken` + consent recording.
- **GOTCHA**: Reuse `Intake` wholesale so OCR/parse/score/dedup are unchanged. Account must have a saved resume + prior PDPA consent before quick-apply (else 400 → prompt to complete profile).
- **VALIDATE**: `go test -race ./internal/applications/... ./internal/candidateauth/...`; quick-apply returns a status_token and the worker scores it as today.

#### Task D2: Account-first apply page + prefilled stepper
- **ACTION**: Update `app/jobs/[id]/apply/page.tsx`, `components/ApplyStepper.tsx`, `lib/queries.ts`.
- **IMPLEMENT**: apply page: if `!isAuthenticated` → `router.replace('/login?return=' + encodeURIComponent(path))`; else render `ApplyStepper` seeded from `useCandidate()`. Stepper: remove the LINE-token step; **prefill** name/phone/province; show two CTAs — **[สมัครด้วยเรซูเม่ที่บันทึกไว้]** → `useQuickApply(positionId)`, and **[แก้ไขข้อมูล / อัปโหลดเรซูเม่ใหม่]** → existing form `POST /apply` (now cookie-authed, no LINE header). Success screen unchanged (status_token).
- **MIRROR**: ApplyStepper success/stepper UI; FRONTEND_QUERY_HOOK for `useQuickApply`.
- **GOTCHA**: Remove `lineIdToken` from `ApplyInput`/`buildApplyForm`/`useApplyMutation` (cookie now carries identity). Keep `buildApplyForm` pure + its unit test (drop the line field).
- **VALIDATE**: dev server; logged-out apply redirects to login; logged-in apply prefills; quick-apply works in one tap.

### PHASE E — Tests, deploy config, polish

#### Task E1: E2E specs
- **ACTION**: Create `e2e/signup.spec.ts`, `e2e/login.spec.ts`; update `e2e/portal.spec.ts`, `e2e/apply-form.spec.ts`.
- **IMPLEMENT**: mock-provider flows — email signup (read OTP from a test seam/mock log or a dev-only `__test/last-otp`), LINE/Google mock bounce, login, account-first apply redirect, quick apply, prefill assertions; screenshots at 320/768/1440 (per web testing rules).
- **MIRROR**: e2e/portal.spec.ts structure + `__screens__` breakpoints.
- **GOTCHA**: For OTP in E2E, add a **dev-only** endpoint or rely on `mockSender` logging + a test hook; never expose in prod.
- **VALIDATE**: `npx playwright test` green.

#### Task E2: Deploy config + docs
- **ACTION**: Document new env in `backend/.env.example`, compose, and the ACA deploy recipe (memory: operator-run `az`).
- **IMPLEMENT**: list `GOOGLE_PROVIDER/CLIENT_ID/SECRET/CALLBACK_URL`, `EMAIL_PROVIDER`, `ACS_EMAIL_ENDPOINT/ACCESS_KEY/SENDER`, `CANDIDATE_SESSION_TTL/COOKIE`, `EMAIL_OTP_TTL`. Note prod cookie `SameSite=None; Secure` + the cross-site/PSL caveat. Provision the ACS Email resource + verified sender; Google OAuth client (authorized redirect = prod callback).
- **VALIDATE**: doc review; secrets via ACA `secretref:` (never in chat — see security note below).

---

## Testing Strategy

### Unit Tests
| Test | Input | Expected | Edge? |
|---|---|---|---|
| `email` ACS signer | known key+payload | expected HMAC header | — |
| `mockSender.Send` | message | logs (dev), no error | — |
| `otp.genCode` | — | 6 digits, crypto/rand | yes (no `Math.rand`) |
| `ConsumeOTP` valid | email+code | success, `consumed_at` set | — |
| `ConsumeOTP` reuse | same code twice | 2nd → `ErrOTPInvalid` | yes |
| `ConsumeOTP` expired | code past TTL | `ErrOTPExpired` | yes |
| `FindOrCreateByEmail` race | concurrent same email | one account, no dup | yes (concurrent) |
| `issue/find session` | token | account; wrong/expired → nil/expired | yes |
| Google callback (mock) | stub | account + session cookie | — |
| LINE callback `mode=link` | session + sub | account.line_user_id set | yes |
| `IntakeFromAccount` | account w/ saved resume | application enqueued, `account_id` stamped | — |
| `IntakeFromAccount` no resume | account w/o resume | 400 / error | yes |
| `buildApplyForm` | input (no LINE) | correct FormData | — |

### Edge Cases Checklist
- [ ] OTP reuse / expiry / wrong code / too many attempts
- [ ] Email enumeration (start always 200)
- [ ] Expired / revoked / missing session cookie → 401 on gated routes
- [ ] Cross-site cookie actually sent (SameSite=None; Secure; credentials:'include')
- [ ] Open-redirect on every OAuth `return` (`safeReturn`)
- [ ] Account with only one identity (email-only, line-only, google-only) — UNIQUE-nullable holds
- [ ] Link LINE to an account that already has a different LINE sub
- [ ] Quick-apply without saved resume / without prior consent
- [ ] Concurrent signup with same email/sub (find-or-create race)

---

## Validation Commands

### Static Analysis
```bash
cd backend && gofmt -l . && go vet ./...
cd backend && golangci-lint run ./... && gosec ./...   # gotcha: gosec G101 fires on var NAMES with "token"/"secret" — rename, don't annotate
cd career-portal && npm run lint && npx tsc --noEmit
```
EXPECT: clean (gofmt no files, golangci 0, gosec 0, tsc 0).

### Unit / Race Tests
```bash
cd backend && go test -race ./...
cd career-portal && npm test   # if unit runner present
```
EXPECT: all pass.

### Database Validation
```bash
~/go/bin/migrate -path backend/migrations -database "$DBURL" up
~/go/bin/migrate -path backend/migrations -database "$DBURL" down 1 && ~/go/bin/migrate ... up   # down/up round-trip
```
EXPECT: 000013 applies + reverts cleanly.

### Browser / E2E
```bash
docker compose up -d            # api/postgres/redis/azurite; EMAIL/GOOGLE/LINE default mock
cd career-portal && npm run dev # http://localhost:3001
cd career-portal && npx playwright test
```
EXPECT: signup (×3), login, account-first apply, quick-apply all green; screenshots captured.

### Manual Validation
- [ ] Email signup: enter email → read OTP from mock log → verify → cookie set → profile → resume → logged in.
- [ ] Refresh page / new tab → still logged in (cookie persists).
- [ ] Google + LINE mock signups create accounts + sessions.
- [ ] Email/Google account → Link LINE → `auth/me` shows `line_user_id`.
- [ ] Logged-out → `/jobs/:id/apply` redirects to `/login?return=…`; after login returns to apply.
- [ ] Apply prefilled; "apply with saved resume" → status_token; worker scores it.
- [ ] Logout → `auth/me` 401; gated routes blocked.

---

## Acceptance Criteria
- [ ] Visitor can sign up via LINE, Google, and Email-OTP; an account + httpOnly session is created.
- [ ] Signup collects profile (name/phone/LINE id/province) — LINE id auto for LINE signups; email/Google can Link LINE.
- [ ] Resume uploaded once at signup is saved to the account.
- [ ] Session persists across pages and reloads; logout revokes it.
- [ ] Applying is account-first (redirect to login when logged out) and **prefilled**.
- [ ] "Apply with saved resume" submits in one tap and runs the existing scoring pipeline.
- [ ] All providers default to **mock** (local/CI need no real credentials).

## Completion Checklist
- [ ] Code follows discovered patterns (seams, repo, opaque tokens, envelope client)
- [ ] Errors wrapped (`fmt.Errorf("…: %w")`); no swallowed errors
- [ ] Secrets only via env / ACA `secretref:`; OTP & session tokens hashed at rest
- [ ] Tests follow table-driven + `-race`; 80%+ on new packages
- [ ] No hardcoded values (TTLs/cookie names/endpoints in config)
- [ ] `credentials:'include'` everywhere; cookie attrs correct per env
- [ ] No unnecessary scope beyond the 5 phases

## Risks
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Cross-site cookie blocked (azurecontainerapps.io is a Public Suffix) | High | High (login silently fails in prod) | `SameSite=None; Secure` + `credentials:'include'`; verify in prod early; plan a custom-domain follow-up for `SameSite=Lax`. |
| ACS Email REST signing wrong (no Go SDK) | Med | Med (OTP undeliverable) | Port the proven `pkg/blob` shared-key signer; unit-test the signature; ship behind `EMAIL_PROVIDER=mock` and flip last. |
| Scope (XL) overruns one pass | High | Med | Ship phases A→E independently; each is mock-default and reversible. |
| `lineauth` dual-mode regresses legacy fragment apply | Med | Med | Keep `issuer==nil` path byte-identical; cover both with tests before switching the portal. |
| Account-first locks out users mid-migration | Med | Med | Land backend (A–B) + frontend session (C) before flipping apply to account-first (D); feature can stay guest-capable until D. |
| GitHub Actions CI billing blocked (per memory) | High | Low | Deploy via operator-run `az` (documented recipe); admin-merge with red CI after local green. |
| OTP abuse / email bombing | Med | Med | Per-email + per-IP rate limits (reuse public limiter), attempt cap, short TTL, enumeration-safe responses. |

## Notes
- **Security (per project memory):** Do NOT paste real secrets in chat. The LINE channel secret was previously exposed and still needs rotation. Provision Google client secret + ACS access key directly into ACA secrets (`secretref:`), and add `connect-src`/redirect origins as needed.
- **Why reuse `Intake` for quick-apply:** keeps OCR→parse→score→dedup and the worker pipeline untouched; the only new thing is *where the bytes/profile come from* (saved account vs. fresh form).
- **Identity model choice:** a dedicated `candidate_accounts` table (login identity + saved profile/resume) sits *above* the existing per-application `candidates` rows (linked via `account_id`). This avoids disturbing the dedup/scoring pipeline that operates on `candidates`/`applications`.
- **`SessionIssuer` interface lives in `lineauth`** (the consumer) and is implemented by `candidateauth.Service` — prevents an import cycle while letting one OAuth implementation serve both legacy and account-first modes.
- **Phasing for safety:** the portal can keep guest apply working through Phases A–C; the account-first switch (D) is the single user-visible cutover.
