# Code Review: ATS 3.8 — Document / Onboarding

**Reviewed**: 2026-06-19
**Branch**: feat/ats-document-onboarding → main
**Mode**: Local (uncommitted changes, pre-PR)
**Decision**: APPROVE (2 review findings fixed during review)

## Summary
Clean, additive slice that faithfully mirrors the offer/letter precedent (domain consts/sentinels, one-tx guarded UPDATE, ON CONFLICT upsert, narrow candidate store, account-scoped server-side resolution). Security posture is solid: no IDOR (candidate never passes an application id), validated doc types + file size/type, blob keys built only from server-resolved uuid + enum doc_type, signed-URL reads. Two minor findings were fixed in-review; no CRITICAL/HIGH remain.

## Findings

### CRITICAL
None.

### HIGH
None.

### MEDIUM
- **Off-palette color** — `OnboardingPanel.statusClass` used `text-emerald-600 dark:text-emerald-400`; emerald appears nowhere else in the dashboard (CP Axtra blue/yellow palette deliberately drops green). **Fixed** → `text-brand` (the positive accent ApprovalPanel already uses).

### LOW
- **Stray review reason on approve** — the HR Review handler passed the request `reason` to the repo regardless of decision, so an `{decision:"approve", reason:"…"}` payload would persist a `review_reason` on an approved row (the notify body already ignored it). **Fixed** → handler clears `reason` when approving.
- **File input not visually reset** after a successful candidate upload (uncontrolled `<input type=file>` keeps the chosen filename). Cosmetic only; the checklist row re-renders with the new status. Left as-is (no controlled-value churn).
- **Empty `ONBOARDING_REQUIRED_DOCS`** would make onboarding never "complete" (0/0). Operator misconfiguration only; default is 7. Not guarded (YAGNI).

## Notable strengths
- Account-scope enforced server-side via `FindHiredApplicationByAccount` — candidate endpoints take no application id (no IDOR), mirroring the offer respond-tx ownership check.
- Re-upload cycle correct: `ON CONFLICT DO UPDATE` resets status→pending and clears review fields (pending → rejected → re-upload → pending → approved).
- Review transition is a single tx with `FOR UPDATE` + guarded UPDATE + `RowsAffected()==0 → 409`, consistent with RespondOffer.
- Notifications are inline best-effort and nil-safe (no asynq, no new worker job); failures logged, never block the action. Verified by a "nil notifier still 200" test.
- Blob key uses server uuid + validated enum only; original filename stored as display metadata, never in the key (no path traversal).
- DRY: shared `lib/upload.ts` constants now back both resume and onboarding uploads.

## Validation Results

| Check | Result |
|---|---|
| Go build / vet / gofmt | Pass |
| Go tests (full suite + 26 new) | Pass |
| Dashboard tsc / eslint / next build | Pass |
| Career-portal tsc / eslint / next build | Pass |
| i18n parity (138 + 59 keys, th/en) | Pass |
| DB migration apply | Skipped (local Docker disk-full; SQL validated by inspection — operator applies on staging/prod) |

## Files Reviewed
Backend (created): onboarding.go, onboarding_repository.go, onboarding_handler.go, onboarding_candidate_handler.go, onboarding_test.go, notify/onboarding_message_test.go, migrations 000025 up/down.
Backend (modified): cmd/api/main.go, applications/notify.go, applications/repository.go, notify/hr_message.go, notify/message.go, pkg/config/config.go, pkg/config/config_test.go.
Dashboard: OnboardingPanel.tsx (created), applications/[id]/page.tsx, lib/{queries,roles,types}.ts, messages/{en,th}.json.
Career-portal: OnboardingSection.tsx + lib/upload.ts (created), account/page.tsx, components/auth/ResumeUploadStep.tsx, lib/{auth,queries,types}.ts, messages/{en,th}.json.
