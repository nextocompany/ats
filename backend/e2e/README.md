# Cross-system E2E suite (Sprint 6b)

Go tests (`//go:build e2e`) that drive the **whole system over HTTP** against the live
docker stack, fully offline via the deterministic mocks. They use the DB only for setup
and for polling async pipeline completion (polling the public `/status` endpoint would hit
the 6a rate limiter).

## Run
```bash
make e2e        # boots stack, migrates, seeds, runs the suite
# or, against an already-running stack:
make up && make migrate-up && make seed
cd backend && go test -tags e2e ./e2e/... -count=1 -v
```
`E2E_API_URL` (default `http://localhost:8080`) and `DB_URL` override the targets.

## Flows
- **full_flow_test.go** — PS `vacancy-opened` → portal `/public/positions` → `/public/apply`
  (mock resume + stub LINE token) → poll DB until the pipeline scores → `/public/status/:token`
  → dashboard `/applications` → hire (PATCH status) → mock PeopleSoft sync.
- **extras_flow_test.go** — re-engagement (rejected applicant + trigger → `reengagement_contacts`
  row via the worker) and reports+search (funnel reflects a hire, on-demand export row, candidate
  findable via `/candidates/search`).

## Notes
- Everything runs with mock providers (no Azure/PS/LINE creds). Mock parser is deterministic
  (สมชาย ใจดี / 0812345678), so repeated applies dedup-merge.
- Async steps poll the DB with bounded deadlines (no fixed sleeps).
- Auth is mock (dev super_admin) so dashboard/reports/search endpoints are reachable.
