# Delivery Scope Roadmap — 7-area client scope (2026-06-16)

Split of the added client scope into prioritized, per-area PRPs (per user decision:
"แยกเป็น PRP รายด้าน + จัดลำดับ"). This index is the single source of truth for
sequencing; each PRP gets its own `*.plan.md` to implement independently.

## Current-state → gap matrix (verified against code)

| # | Scope item | Current state (file evidence) | Gap → which PRP |
|---|---|---|---|
| 1 | AI Parsing CV (20–30 real, varieties) | REAL: Azure DocIntel OCR + GPT-4o parser, `AI_PROVIDER=azure` live. Single-file intake only (`internal/applications/handler.go:62`). Thai/English parser prompt (`internal/ai/azure_parser.go:16`). | No bulk path + needs real-CV accuracy proof → **PRP-2** (bulk) + **PRP-3** (UAT) |
| 2 | AI Scoring vs JD | REAL: hybrid rule+LLM vs Master JD responsibilities+qualifications (`internal/scoring/scorer.go`, `internal/pipeline/process.go:196`). | Mostly done → **PRP-3** (accuracy UAT) |
| 3 | Branch Assignment | REAL: rule-based geospatial province→subregion→nearest store (`internal/branch/assigner.go:35`). | Done (rule-based) → **PRP-3** (validate; confirm rule-based accepted) |
| 4 | User Access Control | REAL: RBAC 7 roles (`internal/rbac/scope.go:29`), Entra SSO + password login (`internal/middleware/auth.go`), admin user CRUD, per-record `ExistsInScope`. | Mostly done → **PRP-3** (validate matrix) |
| 5 | Load / Bulk | Bulk status-change ≤100 (`internal/applications/dashboard_handler.go:135`), asynq queue, rate limit. No bulk CV intake, no load harness. | **PRP-2** (bulk CV upload + load test) |
| 6 | Notification Email / MS Teams | LINE push REAL+live (`internal/notify/rest.go`). Email channel = placeholder error (`rest.go:40`); ACS email exists for OTP only (`pkg/email`). Teams = calendar only, no notifications. | **PRP-1** (email candidate+HR, Teams HR) |
| 7 | Thai/English | No i18n framework; portal Thai, dashboard English (hardcoded). Fonts + LLM prompts bilingual. | **PRP-4** (bilingual UI + switcher, both apps) |

## PRP breakdown + priority

| PRP | Title | Priority | Depends on | Complexity | Plan file |
|---|---|---|---|---|---|
| **PRP-1** | Notifications: Email (candidate + HR) + MS Teams (HR) | **P0** | none (extends notify seam) | Medium | ✅ **DONE + LIVE** (PR #74) |
| **PRP-2** | Bulk CV intake + load testing | **P0** | none | Large | ✅ **IMPLEMENTED** `completed/bulk-cv-intake-load.plan.md` (load test = operator-run on staging) |
| **PRP-3** | Validation & UAT hardening (parsing/scoring/branch/RBAC) | **P1** | PRP-2 (feeds 20–30 CVs + load) | Medium (test-heavy) | `validation-uat-hardening.plan.md` (to generate) |
| **PRP-4** | Bilingual UI (i18n TH/EN with switcher) | **P1–P2** | none (independent) | Large | `bilingual-ui-i18n.plan.md` (to generate) |

### Sequencing rationale
1. **PRP-1 Notifications** first — self-contained, high client visibility, reuses the existing `notify.Notifier` + `pkg/email.Sender` seams (low risk, fast win).
2. **PRP-2 Bulk + Load** next — unblocks PRP-3 (you need bulk intake to feed 20–30 real CVs and to drive load).
3. **PRP-3 Validation/UAT** after PRP-2 — proves parsing accuracy on real varied CVs, scoring-vs-JD quality, branch-assignment correctness, and the RBAC matrix. Mostly a test/measurement effort, minimal new code.
4. **PRP-4 i18n** any time (independent) — large UI refactor; schedule when notification/bulk are stable. Can run in parallel by a separate contributor.

### Notes on decisions captured
- **i18n** = full bilingual UI + language switcher on **both** dashboard and career-portal (next-intl).
- **Notifications** = all of: Email→candidate, Email→HR/manager, MS Teams→HR; keep LINE as-is.
- **Bulk/Load** = build bulk CV upload **and** a real load test.
- Branch assignment stays **rule-based** unless PRP-3 validation shows it needs AI/manual override (open question to confirm with stakeholder during UAT).

> Generate the next plan with `/prp-plan` referencing this roadmap, or implement PRP-1 now with
> `/prp-implement .claude/PRPs/plans/notifications-email-teams.plan.md`.
