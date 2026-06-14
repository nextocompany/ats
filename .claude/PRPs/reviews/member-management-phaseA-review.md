# Code Review: Member Management — Phase A (local)

**Reviewed**: 2026-06-15
**Branch**: `feat/member-management-directory`
**Mode**: Local (uncommitted) — 3 reviewer agents (go/typescript/security) + validation
**Decision**: ✅ APPROVE (after remediation)

## Summary
No CRITICAL or real-bug HIGH findings. HIGH items were perf/structure/hardening; all actionable HIGH+MEDIUM fixed in this pass. Remaining notes are pre-existing (inherited from inbox) or deferred to later phases.

## Findings & Resolution
### Fixed
- **ILIKE wildcard injection / over-match** (go) — `escapeLike` + `ESCAPE '\'`; search capped 200 chars.
- **Missing index on `candidates.account_id`** (go) — added `idx_candidates_account_id` in migration 000016 (perf for apps-per-member subquery).
- **`Stats()` not atomic** (go) — consolidated into one point-in-time query.
- **Input validation** (go/security) — `status` allowlist + malformed `from/to` → 400 (`parseFilter` now returns error); search length cap.
- **`last_seen_at` mislabeled** (go) — renamed to `last_login_at` (it's session-created time) across model/repo/types/UI.
- **`authorized()` zero-value role** (security) — now fails closed on missing auth context (`u.ID == ""`).
- **Audit hardening** (go/security) — resume + detail PII access logged via `h.audit` (logs error, records actor email); new `member_view_detail` action.
- **`window.open` missing noreferrer** (ts/security) — now `noopener,noreferrer`.
- **Unconditional queries before auth known** (ts) — `useMembers`/`useMemberStats` take `enabled`, gated on `allowed`.
- **`Pagination` imported from a page module** (ts) — extracted to `components/ui/pagination.tsx`; repointed applications/candidates/search/members.
- **Duplicated `StatusBadge` + `MEMBER_ADMIN_ROLES`** (ts) — extracted to `components/people/MemberStatusBadge.tsx` + `lib/roles.ts`.
- **a11y** — `scope="col"` on table headers; search input `key` remounts on URL change.
- **Tests** — added Resume handler tests (403/400/404/200) + invalid-status/date 400; `errors.Is` in integration test.

### Deferred / Noted (not blocking)
- List access not audit-logged (high-volume; policy decision) — Detail + Resume are. Note for PDPA policy.
- `Me.role` is `string` not a union — broader change, out of scope.
- Pre-existing eslint warning in `applications/page.tsx` (`items` useMemo dep) — inherited, not introduced.
- `candidate_sessions` has no real last-activity column — `last_login_at` is login time (renamed to be honest); add activity tracking later if needed.

## Validation
| Check | Result |
|---|---|
| go vet / build | ✅ Pass |
| go test -race (members) + integration | ✅ Pass (14 tests) |
| go full suite | ✅ 24 pkgs, no FAIL |
| tsc | ✅ Pass |
| eslint | ✅ Pass (1 pre-existing warning in inbox, not introduced) |
| pnpm build | ✅ Pass (/members routes present) |

## Files Reviewed
Backend: `internal/members/*` (added), `migrations/000016_*` (added), `cmd/api/main.go` (mod).
Frontend: `members/page.tsx` + `[id]/page.tsx` (added), `components/ui/pagination.tsx` + `components/people/MemberStatusBadge.tsx` + `lib/roles.ts` (added), `lib/{queries,types}.ts` + `nav-config.tsx` + `applications/candidates/search/page.tsx` (mod).
