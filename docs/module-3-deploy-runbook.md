# Module-3 Deploy Runbook — migrations 000022–000025

Deploys the ATS **Module-3 post-interview lifecycle** (slices 3.5 approval, 3.6
offer, 3.3 letters, 3.8 onboarding, 3.9 reports) to prod. All five slices are on
`main` but **none is deployed yet**. This runbook covers the four pending DB
migrations **in order** plus rolling the container apps.

- **Target schema:** prod is at **v21** (000021, from slice 3.4/3.1). This applies
  **000022 → 000023 → 000024 → 000025** to reach **v25**.
- **`main` at time of writing:** `6cb23c2cf5d5` (PR #89). Deploy this SHA or later.
- **No data backfill, no destructive change** — every migration is additive
  (`CREATE TABLE IF NOT EXISTS`). 3.9 Reports has **no migration** (read-only).
- **Auth note:** CD (GitHub Actions OIDC) has historically been role-blocked, so
  the **manual `az` path (Option B) is the safe default**. If CD is unblocked,
  Option A is faster and bakes the Entra args for you.

---

## What each migration adds

| Version | File | Slice | Adds |
|---|---|---|---|
| 000022 | `approval_workflow` | 3.5 | `approval_requests`, `approval_steps` tables |
| 000023 | `offers` | 3.6 | `offers` table |
| 000024 | `letters` | 3.3 | `letters` table |
| 000025 | `onboarding_documents` | 3.8 | `onboarding_documents` table |
| — | (none) | 3.9 | Reports = read-only aggregation, no schema |

## Which apps each slice touches (all five must roll)

| App | Why it must roll |
|---|---|
| `hrats-prod-api` | all slice handlers (approval/offer/letter/onboarding/reports) + new config + `fpdf` dep (letters) |
| `hrats-prod-worker` | approval-SLA escalation task + `fpdf` dep (shared backend image) |
| `hrats-prod-scheduler` | approval-SLA cron registration (gated `APPROVAL_SLA_ENABLED`, default OFF) |
| `hrats-prod-dashboard` | ApprovalPanel / OfferPanel / LettersPanel / OnboardingPanel / `/reports` — **needs the 5 Entra build-args** |
| `hrats-prod-portal` | candidate `/offers` accept-decline, "My documents", onboarding upload section |

---

## Environment reference

| Resource | Value |
|---|---|
| Resource group | `hrats-prod-rg` |
| ACR | `hratsacr7qmhyxfjdyyl2` (login `hratsacr7qmhyxfjdyyl2.azurecr.io`) |
| Postgres flexible server | `hrats-prod-pg-7qmhyxfjdyyl2` (db `hr_db`) |
| Container apps | `hrats-prod-{api,worker,scheduler,dashboard,portal}` |
| ACA domain | `yellowmoss-b9b985f7.southeastasia.azurecontainerapps.io` |

### New config env (defaults are safe — usually set NOTHING)

| Env | Default | App | Notes |
|---|---|---|---|
| `COMPANY_NAME` | `CP AXTRA` | api | letterhead on generated PDFs (3.3) |
| `ONBOARDING_REQUIRED_DOCS` | `id_card,house_registration,education_certificate,bank_book,tax_document,photo,health_check` (7) | api | onboarding checklist (3.8); add `military_certificate`/`name_change` to require them |
| `APPROVAL_SLA_ENABLED` | `false` | scheduler | SLA escalation OFF by default — opt in only when wanted |
| `APPROVAL_SLA_CRON` | `0 * * * *` | scheduler | hourly when enabled |
| `APPROVAL_SLA_HOURS` | `48` | scheduler | per-step deadline when enabled |

Notifications fire on prod already (`NOTIFY_PROVIDER=real`, ACS email live; LINE
live). MS Teams onboarding/offer notifications fire only if `TEAMS_WEBHOOK_URL` is
set (otherwise silently skipped). **No new secret is required for Module-3.**

---

## 0) Preflight

```bash
RG=hrats-prod-rg
ACR=hratsacr7qmhyxfjdyyl2
ACR_LOGIN=hratsacr7qmhyxfjdyyl2.azurecr.io
PG=hrats-prod-pg-7qmhyxfjdyyl2

git checkout main && git pull --ff-only          # be on 6cb23c2cf5d5 or later
TAG=$(git rev-parse HEAD | cut -c1-12)
az account show -o table                          # correct MPN sub? (az login = nextto@ert.co.th)

API_FQDN=$(az containerapp show -n hrats-prod-api       -g $RG --query properties.configuration.ingress.fqdn -o tsv)
DASH_FQDN=$(az containerapp show -n hrats-prod-dashboard -g $RG --query properties.configuration.ingress.fqdn -o tsv)
echo "tag=$TAG  api=https://$API_FQDN  dash=https://$DASH_FQDN"
```

Have ready: your public IP for the temp PG firewall rule, and (for a manual
dashboard build) the Entra values — reuse the GitHub Actions repo vars
`AZURE_AD_CLIENT_ID`, `AZURE_AD_TENANT_ID`, `AZURE_AD_AUTHORITY`
(`https://login.microsoftonline.com/organizations` for the multi-tenant app).

> `~/go/bin/migrate` is the golang-migrate CLI. Install if missing:
> `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest`

---

## 1) Apply migrations 000022 → 000025 (ALWAYS manual, do this first)

```bash
DBURL=$(az containerapp secret show -n hrats-prod-api -g $RG --secret-name db-url --query value -o tsv)

MYIP=$(curl -s ifconfig.me)
az postgres flexible-server firewall-rule create -g $RG --server-name $PG \
  --name tmp-migrate-m3 --start-ip-address "$MYIP" --end-ip-address "$MYIP"

~/go/bin/migrate -path backend/migrations -database "$DBURL" version   # expect 21
~/go/bin/migrate -path backend/migrations -database "$DBURL" up        # applies 22,23,24,25 in order
~/go/bin/migrate -path backend/migrations -database "$DBURL" version   # confirm 25

# ALWAYS remove the temp rule afterwards
az postgres flexible-server firewall-rule delete -g $RG --server-name $PG \
  --name tmp-migrate-m3 --yes
```

- `up` applies **all** pending versions in numeric order — 22 → 23 → 24 → 25 in one go. Do **not** apply out of order.
- If `version` reports **dirty** (it has drifted before): `~/go/bin/migrate ... force <last-good-version>` then re-run `up`. All four migrations are idempotent (`CREATE TABLE IF NOT EXISTS`), so a forced re-run is safe.
- Spot-check the tables exist:
  ```bash
  psql "$DBURL" -c "\dt approval_requests approval_steps offers letters onboarding_documents"
  ```

---

## 2) Build + roll the apps

### Option A — GitHub Actions (preferred, if CD is unblocked)

Bakes the Entra build-args automatically from repo vars and rolls all five apps.
**Does not run migrations** — step 1 must already be done.

```bash
gh workflow run deploy.yml -f imageTag=$TAG          # or push a v* tag
gh run watch                                          # follow to completion
```

### Option B — manual `az` (CD blocked) — build 5 images, then roll

```bash
API_URL="https://$API_FQDN"
DASH_URL="https://$DASH_FQDN"

# 2a. Backend images (api/worker/scheduler share backend/Dockerfile; fpdf + Sarabun
#     fonts are baked in automatically from go.mod / committed TTFs).
for SVC in api worker scheduler; do
  az acr build -r "$ACR" -t "hr-ats/$SVC:$TAG" -t "hr-ats/$SVC:latest" \
    --build-arg SVC=$SVC -f backend/Dockerfile backend
done

# 2b. Career portal (only the public API origin is baked).
az acr build -r "$ACR" -t "hr-ats/career-portal:$TAG" -t "hr-ats/career-portal:latest" \
  --build-arg NEXT_PUBLIC_API_URL="$API_URL" -f career-portal/Dockerfile career-portal

# 2c. Dashboard — MUST pass the Entra args or SSO silently regresses to DEV sign-in.
#     Reuse the repo Actions vars for the values.
AD_CLIENT_ID=<AZURE_AD_CLIENT_ID>
AD_TENANT_ID=<AZURE_AD_TENANT_ID>
AD_AUTHORITY=https://login.microsoftonline.com/organizations   # multi-tenant app
az acr build -r "$ACR" -t "hr-ats/dashboard:$TAG" -t "hr-ats/dashboard:latest" \
  --build-arg NEXT_PUBLIC_API_URL="$API_URL" \
  --build-arg NEXT_PUBLIC_AZURE_AD_CLIENT_ID="$AD_CLIENT_ID" \
  --build-arg NEXT_PUBLIC_AZURE_AD_TENANT_ID="$AD_TENANT_ID" \
  --build-arg NEXT_PUBLIC_AZURE_AD_REDIRECT_URI="$DASH_URL" \
  --build-arg NEXT_PUBLIC_AZURE_AD_AUTHORITY="$AD_AUTHORITY" \
  -f frontend/Dockerfile frontend

# 2d. Roll each app to the new tag.
for pair in \
  "hrats-prod-api:api" "hrats-prod-worker:worker" "hrats-prod-scheduler:scheduler" \
  "hrats-prod-portal:career-portal" "hrats-prod-dashboard:dashboard"; do
  APP="${pair%%:*}"; REPO="${pair##*:}"
  az containerapp update -n "$APP" -g "$RG" --image "$ACR_LOGIN/hr-ats/$REPO:$TAG"
done
```

> **GOTCHA (recurring):** the manual dashboard build (2c) MUST include the four
> `NEXT_PUBLIC_AZURE_AD_*` args. Omitting them ships DEV sign-in mode and breaks
> Entra SSO. Option A avoids this by reading the repo vars.

### (Optional) enable approval-SLA escalation

Only if HR wants automatic SLA reminders (default OFF):

```bash
az containerapp update -n hrats-prod-scheduler -g $RG \
  --set-env-vars APPROVAL_SLA_ENABLED=true APPROVAL_SLA_HOURS=48
```

---

## 3) Verify

```bash
# API health
curl -s -o /dev/null -w "%{http_code}\n" "https://$API_FQDN/health"        # 200

# New routes exist + are auth-gated (401/403 without a session = wired correctly)
for p in \
  "/api/v1/applications/00000000-0000-0000-0000-000000000000/offer" \
  "/api/v1/applications/00000000-0000-0000-0000-000000000000/onboarding" \
  "/api/v1/reports/ats" ; do
  echo "$p -> $(curl -s -o /dev/null -w '%{http_code}' "https://$API_FQDN$p")"
done

# Candidate routes (origin-guarded, require cp_session) — expect 401
curl -s -o /dev/null -w "%{http_code}\n" "https://$API_FQDN/api/v1/public/auth/onboarding"  # 401

# Schema
psql "$DBURL" -c "SELECT version, dirty FROM schema_migrations;"            # 25, f
```

Then **human UAT** (logged in):
- Dashboard SSO still works (Microsoft login, not DEV mode).
- Approval chain: submit → decide across levels → app reaches `offer`.
- Offer: compose/send → candidate `/offers` accept (→ `hired`) / decline (→ rejected).
- Letters: generate interview + offer PDF → opens, Thai renders.
- Onboarding: candidate `/account` uploads each doc → HR `OnboardingPanel` approve/reject+reason → candidate sees outcome → re-upload resets to pending.
- Reports: `/reports` visible to HR roles only; date range changes numbers; a store HR sees only their store; **Export CSV** downloads `ats-report.csv` matching the page. (Validates the 3.9 aggregation SQL on real data — it could not be tested locally.)

---

## 4) Rollback

**App rollback** (no data change) — re-point to the previous image tag:

```bash
PREV=<previous-good-tag>
for pair in \
  "hrats-prod-api:api" "hrats-prod-worker:worker" "hrats-prod-scheduler:scheduler" \
  "hrats-prod-portal:career-portal" "hrats-prod-dashboard:dashboard"; do
  APP="${pair%%:*}"; REPO="${pair##*:}"
  az containerapp update -n "$APP" -g "$RG" --image "$ACR_LOGIN/hr-ats/$REPO:$PREV"
done
```

The old images tolerate the new tables (additive), so an app-only rollback is safe
and is the **preferred** rollback. The new tables simply go unused.

**Migration rollback** (DESTRUCTIVE — drops the tables + their data; only if truly
needed, via the temp firewall rule from step 1):

```bash
~/go/bin/migrate -path backend/migrations -database "$DBURL" down 4   # 25→24→23→22→21
```

---

## Notes

- Module-3 is **all on `main`** but treat this as one deploy: migrations first, then
  roll all five apps together so the api/dashboard/portal stay consistent.
- No new secret, no infra change. `fpdf` + Sarabun fonts are compiled into the
  backend image automatically — nothing to configure.
- The 3.9 reports aggregation SQL was validated by inspection + handler/unit tests
  only (local Postgres was unavailable); the `/reports` + CSV UAT step above is the
  first real-data check — do it.
