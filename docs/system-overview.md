# ระบบ AI HR Recruitment & Screening — System Overview

> สรุปสถาปัตยกรรมและฟีเจอร์ทั้งระบบ แบ่งตาม 3 surface หลัก: **Pool** (ชั้นข้อมูลผู้สมัครที่แชร์กัน), **Portal** (แอปฝั่งผู้สมัคร), และ **ATS** (แดชบอร์ดฝั่ง HR/recruiter)
>
> อัปเดต: 2026-06-28 · prod schema v49 · main `108c9c3`

ระบบเป็นแพลตฟอร์มสรรหาบุคลากรแบบ end-to-end: **intake → AI screening/scoring → HR approval → offer → onboarding → PeopleSoft HCM sync** ออกแบบมาสำหรับ CP Axtra (retail หลายสาขา) รองรับภาษาไทยเป็นหลัก

---

## 0. ภาพรวมสถาปัตยกรรม (Shared / Cross-cutting)

### 0.1 Surface ทั้ง 3 ส่วน

| Surface | คืออะไร | Deploy เป็น app |
|---|---|---|
| **Pool** | ชั้นข้อมูล "คน" ที่แชร์ระหว่างทั้งสองแอป — candidate accounts, resume library, dedup, talent pool, search, intake pipeline | ไม่ใช่ app แยก (เป็น domain/data layer ใน backend) |
| **Portal** | แอปฝั่งผู้สมัคร (candidate-facing) — สมัครงาน, ดูสถานะ, AI pre-interview, ตอบ offer | `career-portal/` (Next.js PWA) → `hrats-prod-portal` |
| **ATS** | แดชบอร์ดฝั่ง HR/recruiter — pipeline, approval, interview, offer, analytics, admin | `frontend/` (Next.js) → `hrats-prod-dashboard` |

ทั้งสาม surface วิ่งอยู่บน **backend Go เดียวกัน** (Fiber) ที่ deploy เป็น 3 process: `api`, `worker` (asynq), `scheduler` (cron)

### 0.2 Tech stack

- **Backend**: Go 1.26 + **Fiber v2** · **pgx/v5** + PostgreSQL · **asynq** (Redis-backed) สำหรับ async pipeline · zerolog
  - cmd: `api`, `worker`, `scheduler`, `reindex`, `importref`, `seedresumes`
- **Frontend (ทั้ง 2 แอป)**: Next.js 16.2.6 (App Router) · React 19.2.4 · TypeScript · **TanStack Query v5** · **next-intl** (TH/EN) · Tailwind v4 + shadcn-style UI · lucide-react
  - ATS dashboard เพิ่ม **MSAL** (`@azure/msal-browser/react`) สำหรับ Entra SSO
  - Portal เพิ่ม **Serwist** (`@serwist/next`) สำหรับ PWA
- **Infra**: Azure — **Bicep** IaC (`infra/`) deploy **Azure Container Apps** + ACR + Postgres Flexible Server + Redis + Key Vault + monitoring

### 0.3 Deployment topology (prod, southeastasia)

| Container App | บทบาท |
|---|---|
| `hrats-prod-api` | HTTP API handlers ทั้งหมด |
| `hrats-prod-worker` | asynq worker — รัน intake pipeline + sweep tasks |
| `hrats-prod-scheduler` | cron registration (interview reminder, SLA escalation ฯลฯ) |
| `hrats-prod-dashboard` | ATS HR dashboard (ต้อง build ด้วย 5 Entra build-args ไม่งั้น SSO ตกไป DEV mode) |
| `hrats-prod-portal` | Career portal (candidate) |

> Deploy แบบ manual `az` (OIDC ยัง broken) ตาม `docs/module-3-deploy-runbook.md` — roll เฉพาะ service ที่เปลี่ยน

### 0.4 Provider seam — "mock-by-default" (สำคัญที่สุด)

pattern ที่ใช้ซ้ำมากที่สุดทั้งระบบ: **external dependency ทุกตัวอยู่หลัง factory ที่ default เป็น mock** (log-only, ไม่ต้องใช้ credential) เพื่อให้ local/CI รันได้โดยไม่ต้องมี secret; เลือก provider จริงผ่าน env config

| Integration | Seam | Provider จริง |
|---|---|---|
| CV OCR + parse | `ai/factory.go` | Azure Document Intelligence + Azure OpenAI (หรือ Gemini) |
| Screening LLM | `scoring/scorer.go` | Azure OpenAI / Gemini |
| AI pre-interview | `interview/factory.go` | Azure OpenAI |
| Calendar | `calendar/calendar.go` | MS Graph (`GRAPH_PROVIDER=real`) |
| Notify | `notify/rest.go` ↔ `mock.go` | LINE push + email (ACS) + Teams webhook |
| PeopleSoft | `peoplesoft/{mock,rest}.go` | PeopleSoft REST |
| Search | `search/search.go` | Postgres trigram (default) / Azure AI Search |
| Executive ROI | `executive/service.go` | live aggregation (`EXECUTIVE_PROVIDER`, default mock) |

### 0.5 Status lifecycle (กระดูกสันหลังของระบบ)

candidate application status นิยามครั้งเดียวที่ `backend/internal/applications/model.go` และ mirror ที่ `frontend/lib/statusMachine.ts`:

```
pending → (pipeline) → parsed/scored → [must-have gate] → rejected
                                      ↘ scored → ai_interview → ai_interviewed
   → shortlisted → interview → interviewed → pending_approval
   → (4-level approval) → offer → hired → (onboarding) → closed
สถานะ recoverable: invalid_resume, name_mismatch (ผู้สมัคร re-upload ได้)
```

manual transition ฝั่ง HR ถูก gate ด้วย `applications/transitions.go` (`allowedTransitions`) — เช่น `interview` ต้องมี schedule payload, `rejected` ต้องมีเหตุผล

---

## 1. POOL — ชั้นข้อมูลผู้สมัคร (Candidate / Talent Pool)

ชั้นข้อมูล "คน" ที่แชร์กันทั้งระบบ มี **identity 3 tier** ที่เชื่อมกันผ่าน `candidates.account_id`:

- **`candidateauth`** — identity ฝั่ง portal (session cookie, self-service)
- **`members`** — projection ฝั่ง HR เหนือตาราง `candidate_accounts` เดียวกัน (role-gated, ไม่เห็น raw PII)
- **`candidates`** — row ต่อใบสมัครที่ pipeline ทำงานด้วย (1 account เป็นเจ้าของได้หลาย row)

### 1.1 Candidate accounts

`candidate_accounts` = identity ถาวรของ portal — มี email / `line_user_id` / `google_sub` อย่างใดอย่างหนึ่งก็พอ ส่วนที่เหลือเติมทีหลังเมื่อ link provider เพิ่ม

ความต่างของฟิลด์ชื่อ (สำคัญ):
- `display_name` — cosmetic จาก LINE/Google login, **ไม่เคยใช้ matching**
- `name_th` / `name_en` — required ตอนสมัคร, **ใช้ match กับชื่อใน resume ที่ parse ได้**
- `line_user_id` / `google_sub` / `resume_blob_url` เป็น `json:"-"` — ไม่ serialize ออกไปหา client
- `status`: `active | suspended | anonymized` (เฉพาะ `active` ออก session ได้)

วิธีเกิด account:
- **Self-provision (portal)**: email-OTP / LINE login / Google login
- **Provisioned ตอน intake (เงียบ)**: worker เรียก `EnsureAccountByEmail` สร้าง account แบบ *unverified* โดย key ด้วย email ที่ parse จาก resume แล้ว `SetAccountID` ผูก canonical candidate

**Unify**: `members` คือ admin projection ของ `candidate_accounts` — หน้า "person page" รวมใบสมัครจากทุก candidate row ที่ผูก account เดียวกัน; `candidates.List` ตัด orphan ที่ `is_duplicate_of IS NOT NULL` ออก

### 1.2 Resume library

`candidate_account_resumes` = ประวัติ CV ต่อ account, **cap ที่ 5 ใบ**; มี 1 ใบเป็น `is_default = TRUE` เท่านั้น (`SetDefaultResume` เคลียร์ทั้งหมดก่อนแล้วตั้ง 1 ใบ) มี denormalized pointer `candidate_accounts.resume_blob_url` sync ทุกครั้งที่ insert/set-default/delete เพื่อให้ QuickApply อ่านเร็ว

**QuickApply** (`POST /api/v1/public/apply/quick`) — ใช้ profile + default resume ที่บันทึกไว้ ป้อนเข้า pipeline เดียวกับ apply ปกติ

### 1.3 Dedup (ตรวจผู้สมัครซ้ำ) — 2 ชั้น

- **Reconciliation** (`dedup/dedup.go`): หลังสร้าง candidate, `Reconcile` โหลดคนที่ใช้ contact ร่วม (id_card/phone/email) แล้วให้คะแนน
  - national id ตรง = 1.0 · contact+name = 0.95 · contact-only = 0.85 · name-only = 0.5
  - name match ใช้ Levenshtein ≤ 2
  - **≥ 0.9** → `auto_merged` (ตั้ง `is_duplicate_of`) · **0.7-0.9** → `pending_review` (HR ตัดสิน) · ที่เหลือ `none`
  - เก็บผลที่ `applications.dedup_state`
- **Resume-name gate** (`dedup/namematch.go`): `NameMatchesAny(resume, name_th, name_en)` เช็คชื่อใน resume กับชื่อ account แบบ lenient (ตัด honorific, shared-token, ทน OCR) — จงใจหลวมเพื่อไม่ reject คนจริงผิด ๆ จากชื่อไทย/ชื่อเล่น

### 1.4 Talent pool

`talent_pool` = boolean บน **applications** = ผู้สมัครอยู่ใน pool กลางแทนที่จะถูก assign ให้สาขาใดสาขาหนึ่ง

- HR ตั้งเองตอน assignment (`TalentPool || StoreNo == nil` → pool); repo ตั้ง `assigned_store_id = NULL, talent_pool = TRUE`
- **Pool release** (`pool_release.go`): sweep คืนผู้สมัครเฉพาะสาขาที่ HR ไม่ดำเนินการภายใน grace window (default 3 วัน) กลับ pool กลาง — **ปิดอยู่ default** (`POOL_RELEASE_ENABLED=false`) รอ wiring `picked_up_at`
- **Re-engagement** (`reengage/`): หาคนใน pool/เคยถูก reject (`talent_pool IS TRUE OR status='rejected'`, ตัดที่ hired แล้ว) แล้วแจ้งเรื่องตำแหน่งที่ (re-)เปิด — channel: LINE push → email; บันทึก contact ก่อนส่งเพื่อ at-most-once

### 1.5 Search

seam เลือกด้วย config: **Postgres trigram/ILIKE** (default, zero-credential) หรือ **Azure AI Search** เมื่อ `UsesAzureSearch()` มี optional `Embedder` (จาก `internal/ai`) เปิด **hybrid keyword + vector ranking**; ไม่มี = keyword-only (degrade graceful) ผลลัพธ์ scope ด้วย RBAC เสมอ index source = `candidates JOIN applications` (ตัด duplicate)

### 1.6 Intake pipeline

front door 2 ทาง converge ที่ `applications.Service.Intake`:
- **Portal**: `POST /api/v1/public/apply` + `/apply/quick`
- **External webhook** (`intake/`): `POST /api/v1/intake/:source` สำหรับ `ms_forms | seek | jobsdb` — resume เป็น base64 inline (ไม่ fetch URL → กัน SSRF), HMAC-signed + replay protection, ปิดถ้าไม่มี `INTAKE_WEBHOOK_SECRET`

worker `Processor.run` รัน 7 step (ดู §2.1 ฝั่ง ATS):
**OCR → parse → not-resume gate → name-mismatch gate → dedup → account link → score → branch assign → index**

### 1.7 ตาราง DB + endpoint หลัก

**Tables**: `candidate_accounts`, `candidate_account_resumes`, `candidates` (`account_id`, `is_duplicate_of`, `line_user_id`), `applications` (`talent_pool`, `dedup_state`, `assigned_store_id`, `released_to_pool_at`, `picked_up_at`, `ai_score`), `candidate_locks`, `member_notes`/`member_tags`, reengage contacts, `activity_logs`

**Endpoints**:
- Portal auth/resume: `/api/v1/public/auth/{email/start, email/verify, logout, me, profile, resume, resumes, resumes/:id/file, resumes/:id/default, resumes/:id, consent/withdraw, consent/accept}` + `/auth/google`
- Apply: `/api/v1/public/{apply, apply/quick, positions}`
- Intake webhook: `/api/v1/intake/:source`
- HR candidate read: `/api/v1/candidates`, `/:id`, `/:id/timeline`, `/search`, `/:id/lock`
- Re-engage: `/api/v1/positions/:id/reengage`
- Members admin: `/api/v1/admin/members/*` (`/`, `/stats`, `/export.csv`, `/bulk`, `/:id`, `/:id/resume`, `/:id/status`, `/:id/force-logout`, `/:id/anonymize`, `/:id/notes`, `/:id/tags`)

---

## 2. PORTAL — แอปฝั่งผู้สมัคร (Career Portal)

Next.js PWA ที่ `career-portal/` — audience ส่วนใหญ่เปิดใน LINE in-app browser, ภาษาไทยเป็นหลัก

### 2.1 Route / page

| Route | ผู้สมัครทำอะไร |
|---|---|
| `/` | Landing (CP Axtra brand): Hero + sections |
| `/jobs` | เปิดดูตำแหน่งที่เปิดรับ; client-side search + level facets; เป็น PWA `start_url` |
| `/jobs/[id]` | รายละเอียดตำแหน่ง (Master JD: responsibilities/qualifications/benefits + fallback generic), ปุ่มแชร์, ปุ่ม Apply |
| `/jobs/[id]/apply` | **Account-first** — ยังไม่ login เด้งไป `/login?return=…`; login แล้ว `ApplyStepper` prefill จาก account; รองรับ deep-link "Apply with SEEK" (`seek_name/email/phone/province`) |
| `/login` | LINE/Google OAuth + Email OTP; เคารพ `?return=` |
| `/signup` | one-time setup: เลือก method → `ProfileForm` (+ PDPA consent) → `ResumeUploadStep`; เสร็จเมื่อ `has_resume` |
| `/status` | ติดตามสถานะด้วย opaque token (prefill จาก `?token=`); public = status card สั้น; ถ้า login + เป็นเจ้าของ = timeline แบบ curated |
| `/account` | self-service: identity + completeness %, ProfileForm, ApplicationHistory, ResumeLibrary (≤5), LINE-link, OnboardingSection, PDPA DataRights (export/erase), re-consent banner |
| `/offers` + `/offers/[id]` | รายการ offer (OfferCard) + documents (offer/interview letters); `[id]` = application id เป็น target ของ notification CTA |
| `/interview` | AI pre-interview chat; token อ่านจาก **URL fragment** (`#token=…`) ผ่าน `useSyncExternalStore` เพื่อไม่ให้ขึ้น server/log |
| `/privacy` | PDPA privacy notice (ไทย) |
| `/offline` | offline fallback shell (precached) |

### 2.2 Auth

- **LINE Login (OAuth 2.1)** — `/api/v1/public/line/login` + `/callback`; ตั้ง httpOnly CSRF `line_oauth` state cookie, แลก code → `id_token` ฝั่ง server
  - **Account-first mode**: verify id_token → `LoginWithLine` find-or-create → ตั้ง session cookie
  - **mode=link**: ผูก LINE เข้า session ปัจจุบัน (`ErrLineLinkedToOther` → `line_in_use`)
  - email scope เปิดเฉพาะเมื่อ `LINE_REQUEST_EMAIL_SCOPE`; mock mode คืน `dev-line-id-token`; `safeReturn` กัน open redirect
- **Google OAuth** — `/api/v1/public/auth/google`
- **Email OTP** — `/api/v1/public/auth/email/{start,verify}`
- **Session** — opaque token ใน httpOnly cookie; cross-site portal↔API จึง `SameSite=None; Secure` (prod) / `Lax` (dev); `RequireCandidate` gate route ที่ต้อง auth
- **Email capture** — LINE account มักไม่มี email; ฟอร์มสมัครเก็บ phone+email แล้ว `BackfillContact`/`BackfillNames` เขียนลง account (set-once)
- **PDPA consent** — บังคับ, ไม่เคยปลอม, บันทึกตอนสมัคร/signup

### 2.3 Public API ที่ portal เรียก

- **ไม่ต้อง auth**: `GET /positions`, `GET /positions/:id` (JD projection), `POST /apply` (multipart, ≤10MB pdf/docx/jpg/png, mint status token, คืน `{status_token}` 201), `GET /status/:token` (status สั้น)
- **ต้อง auth (`RequireCandidate`)**: `POST /apply/quick`, `GET /me/applications`, `GET /me/applications/:token/timeline` (unknown/unowned ทั้งคู่ 404 — ไม่มี IDOR oracle), membership/resume/consent, offers (`GET /auth/offers`, `POST /auth/offers/:id/respond`), letters, onboarding, DSAR (`/auth/me/erase`, `/auth/me/export`), interview (`GET/POST /api/v1/public/interview/:token[/message]`)
- ทั้ง group `/api/v1/public` มี IP rate-limit; `/auth` + `/apply` มี origin guard เพิ่ม

### 2.4 PWA

- **Manifest**: ชื่อ "ร่วมงานกับเรา", `start_url:/jobs`, `display:standalone`, `lang:th`, theme `#0B47B8`
- **Service worker** (Serwist): `NetworkFirst` สำหรับ `GET /api/v1/public/*` (cache 5s timeout, เก็บเฉพาะ 200) → offline เห็นข้อมูลล่าสุด; **apply POST ไม่ cache** (PII + LINE token); navigation ที่ uncached fallback → `/offline`

### 2.5 Status tracking

- **opaque public_token**: 24-byte base64url (ไม่ใช่ UUID), mint ก่อน apply notification เพื่อให้ deep link `/status?token=…` ติดไปด้วย
- **public view**: `{status, applied_at, position}` เท่านั้น — ไม่โชว์ AI score/internal
- **login-gated timeline**: milestone แบบ curated (`apptimeline.Build`), account-scoped

### 2.6 Offers (ฝั่งผู้สมัคร)

`POST /auth/offers/:id/respond`, decision = `accept | decline | negotiate`:
- **accept** → application → `hired` (เริ่ม onboarding); PeopleSoft push เลื่อนไปตอน onboarding approve-complete (ไม่ใช่ตอน accept)
- **decline** → ต้องมีเหตุผล → reject application
- **negotiate** → ต้องมี `counter_salary` > 0; `NegotiateOffer` พัก offer ที่ `negotiating` (app ยัง `offer`) รอ HR revise/resend; cap ด้วย `NEGOTIATION_MAX_ROUNDS` (`ErrNegotiationClosed` → 409); แจ้ง HR (email/Teams) best-effort

---

## 3. ATS — แดชบอร์ดฝั่ง HR (Recruiter Dashboard)

Next.js dashboard ที่ `frontend/app/(app)/` เหนือ backend Go เดียวกัน

### 3.1 Recruiting pipeline (auto screening) — 7 step

worker (`pipeline/process.go` `HandleProcessApplication`):

1-2. **OCR + parse** → `ai.Profile`; upload OCR text + parsed JSON ลง blob; OCR confidence < 0.7 = flag manual review (ไม่ abort)

2a/2b. **Gate**: *not-a-resume* (`invalid_resume`) + *name-mismatch* vs account holder (`name_mismatch`, lenient Thai-name match) — recoverable, ไม่ retry

3. **Dedup** (`dedup.Reconcile`) → reconcile เป็น canonical candidate

4. **Scoring** (`scoring.Scorer.Score`): 5 มิติ มี cap คงที่ — Experience 30, Skills 20, Education 10, Language 10, Location 20
   - rule แบบ deterministic (`scoring/rules.go`) รองรับวุฒิไทย + ภาษาไทย/อังกฤษ
   - **Skills (0-20) เป็นมิติเดียวที่มาจาก LLM**; LLM ยังให้ strengths/red-flags/suggested-positions ภาษาไทย
   - **per-position weights**: `WeightedTotal = Σ weight_i·(subscore_i/cap_i)` → 0-100, weights รวม = 100; โหลดจาก `positions.score_weights`, fallback `DefaultWeights{34,22,11,11,22}`; แก้ได้ที่หน้า **Scoring** (gated `settings.admin`)

5. **Must-have gate**: min-education + min-experience-months แบบ binary; ไม่ผ่าน → short-circuit **ก่อนเรียก LLM** (ไม่เปลือง token) → `rejected`

6. **Branch assignment** (`branch/assigner.go`): match จังหวัดผู้สมัคร → vacancies ที่เปิด → สาขาใกล้สุด (haversine) + filter store-format; ไม่ match → talent pool; ผูก `vacancy_id` ให้ scope resolve ไปหา hiring manager → `scored`

7. **Notify** best-effort: email + Teams หา store HR + email หา hiring manager; ผู้สมัครได้ LINE+email

**Inbox**: `applications/page.tsx` (filter, score badge/fit label, bulk, pagination); detail `applications/[id]/`; bulk intake `applications/bulk/`

### 3.2 Hiring workflow (manual funnel)

- **Requisitions** — JD เปิดเองเขียนลงตาราง `vacancies` `source='manual'` (vs `peoplesoft`-synced); fields: responsibilities, qualifications, benefits, employment-type, salary min/max, priority, headcount; lifecycle `pending_approval → open → closed/cancelled` (เฉพาะ `open` ผู้สมัครเห็น); แก้ JD บน PS-row ได้แต่ไม่แตะ position/store/headcount ที่ PS เป็นเจ้าของ
- **Approvals** — chain 4 ระดับคงที่: L1 Store HR (auto ตอนสร้าง) → L2 Area HR → L3 Hiring Manager → L4 Talent Acquisition; permission `approval.decide.l1..l4`; reject = terminal; ผ่าน L4 → `offer`; มี SLA escalation sweep
- **Interviews** (2 อย่างคนละชนิด):
  - *AI pre-interview*: text chat adaptive, ผู้สมัครใช้ opaque token, LLM interviewer อิง JD ~6 turn → transcript + `Evaluation{score, recommendation, strengths, concerns}` (sync, ไม่ผ่าน worker)
  - *Human interview*: HR calendar + scheduling, invite ผ่าน MS Graph (ผู้สมัครเป็น attendee), multi-round
- **Compare + ranking** — จัดอันดับ pool หลัง AI-interview ด้วย composite (screening + AI-interview score, น้ำหนักเท่ากัน) เทียบ side-by-side
- **Shortlist** — Top-5 ของ `shortlisted` ผสม screening กับ TA scorecard
- **Scorecards** — 2 มุม: TA (`scorecard.ta`) + Line-Manager (`scorecard.lm`), rating 1-5 เฉลี่ยเป็น composite
- **Offers** — lifecycle `draft → sent → negotiating → accepted/declined/expired`; **benefits** เป็น structured lines (`Benefit{label,value}`), salary, start-date, terms, expiry; ผู้สมัครตอบผ่าน portal (accept→hired+PS push ใน transaction / decline→rejected / negotiate→counter, bounded rounds); gated `offer.write`
- **Letters** — PDF interview-invite + offer letter (renderer `internal/letters/`), 1 current ต่อ (application,type), เก็บ blob, HR + ผู้สมัคร download ได้; gated `letter.write`
- **Onboarding** — หลัง hire ผู้สมัคร upload doc checklist (id_card, house_registration, education, bank_book, tax, photo, health_check; military/name-change optional); HR approve/reject ทีละใบ; **`Closed`** = onboarding ครบ **และ** `ps_synced_at` ตั้งแล้ว
- **PeopleSoft sync out** (`peoplesoft.SyncHired`) — push hired ไป PS REST + retry; **PS fail ไม่ทำให้ hire fail** — เขียน CSV fallback ลง blob, ปล่อย `ps_synced_at` ว่าง

### 3.3 Analytics

- **Executive ROI** (เพิ่งขึ้น, PR #193) — `executive/roi.go` + `executive/page.tsx`; live aggregation จาก ATS data + admin `executive_cost_config` (ไม่มี HRIS); rolling lookback (month=1/quarter=3/year=12); cards: ROI band (cost-per-hire, savings, ROI %, vacancy-cost-avoided), volume/response funnel, time-to-hire (avg/median ผ่าน `PERCENTILE_CONT`), success by branch/region/position (`Σ success.hires == headline`); period/dimension ใน URL; gated `executive.view`; tab budget/headcount เดิม retire (รอ HRIS); live/mock ใช้ `applyROIMath` ร่วมกัน
- **Reports** — recruitment-funnel metrics ตามช่วงวันที่, RBAC-scoped, CSV export; gated `reports.view`/`reports.export`

### 3.4 Admin & governance — RBAC 2 แกน (dynamic)

- **แกน Permission** — catalog คงที่ 26 key (`rbac/permissions.go`), แต่ละ key map 1:1 กับ handler call site; matrix role→permission เป็น **data-driven** (`rbac_role_permissions`, แก้ runtime ได้); `super_admin` = hard code bypass; มี migration-parity test กัน code/DB หลุดกัน
- **แกน Scope** — `rbac/scope.go`: visibility `all > subregion > area > requisition > store`; แต่ละ kind emit SQL predicate ต่อ table (`ApplicationsClause`, `VacanciesClause`, `CandidatesClause`, `AccountsClause`); role ที่ไม่รู้จัก fail-closed แคบสุด; area scope ผ่าน `user_areas`/`area_stores`; requisition scope = "ผู้สมัครในตำแหน่งที่ฉันเปิด"; talent pool กลางทุก store/area operator เห็น

หน้า: **Admin** (super-admin tenant toggle + Roles/Permissions matrix + Users, `users.admin`) · **Areas** (store grouping + area_hr, `area.admin`) · **Scoring** (per-position weights, `settings.admin`) · **Members** → redirect ไป **Candidates** ที่ unify แล้ว · Settings backend = `system_settings` key-value

### 3.5 PDPA / compliance

- **PDPA/DPO console** (`pdpa.admin`) — overview, DSAR queue ที่ hold, consent-history lookup; หน้า **Privacy** สำหรับ HR ทุกคน
- **Consent** — versioned + candidate snapshot; รองรับ consent ระดับ account (ก่อนสมัคร)
- **DSAR** — self-service export + erasure ผ่าน portal; erase ทันทีถ้าไม่มี legal hold ไม่งั้น queue ให้ HR/DPO; audit ทุก event
- **Data erasure / retention** — anonymize irreversible (`members.erase`); retention sweep ลบ candidate เกิน window (กัน zero-window)
- **Breach register** (`breach.manage`) — PDPC §37 แจ้งเหตุละเมิด 72 ชม., generate draft TH/EN พร้อม deadline/overdue tracking

### 3.6 Integrations

- **PeopleSoft in** (`peoplesoft/webhook.go`): webhook upsert vacancies; position code ที่ map ไม่ได้เก็บไว้ให้ admin ดู; trigger re-engagement sweep
- **PeopleSoft out**: hired push + CSV fallback (§3.2)
- **LINE / email (ACS) / Teams**: candidate messaging + HR alert
- **MS Graph**: calendar invite สำหรับ human interview
- ทั้งหมดตาม provider seam §0.4 (mock-by-default)

### 3.7 หมายเหตุเพิ่มเติม

- `internal/fit/` — AI cross-position fit analysis (รวม screening + AI-interview eval → แนะนำว่าเหมาะกับ Master JD ตำแหน่งไหน, เหตุผลภาษาไทย) โผล่ใน inbox `FitLabel` + candidate detail

---

## ภาคผนวก — Caveat ที่ควรรู้

- **Pool release sweep ปิดอยู่ default** (`POOL_RELEASE_ENABLED=false`) รอ wiring `picked_up_at`
- **Search** silently fallback เป็น keyword-only เมื่อไม่มี embedder/Azure config
- **Executive ROI** `EXECUTIVE_PROVIDER` ยังเป็น `mock` บน prod; cost-config เริ่มต้น unset → ROI โชว์ empty-state จนกว่า settings.admin จะตั้งค่า
- **Dashboard build** ต้องใส่ 5 Entra build-args (`NEXT_PUBLIC_AZURE_AD_*`) ไม่งั้น SSO regress เป็น DEV sign-in
- **Deploy** ยังเป็น manual `az` (OIDC SP ไม่มี subscription role) — roll เฉพาะ service ที่เปลี่ยน
