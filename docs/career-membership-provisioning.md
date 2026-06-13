# Career Membership — Provisioning & Deploy Runbook

> Candidate membership (signup/login via LINE / Google / passwordless email-OTP,
> httpOnly session, saved profile + resume, account-first apply) merged to `main`
> 2026-06-13 (PR #58 → `5094fcc`). **Code is on `main` only — NOT yet deployed or
> provisioned for prod.** All three providers default to `mock`, so the live api
> behaves as before until you complete this runbook.
>
> CI/CD is billing-blocked → everything here is **operator-run `az`**. This is a
> drafted checklist: fill the `<PLACEHOLDERS>`, verify each `az communication`
> flag against your CLI version, and run steps in order.

## ⚠️ Read first

- **Cross-site cookie.** The portal and api are cross-site because
  `azurecontainerapps.io` is on the Public Suffix List. The session cookie is set
  `SameSite=None; Secure` automatically when `ENV != development`, and
  `CORS_ALLOW_ORIGINS` **must list the exact portal origin** (no wildcard);
  `AllowCredentials` is already `true`. If login "works" but doesn't persist
  across a refresh, this is almost always the cause.
- **ACS Email has no Go SDK.** `pkg/email/acs_sender.go` calls the ACS REST API and
  signs with the access key (shared-key HMAC, like Azure Storage). Send is async
  (202 Accepted). You need an ACS Email resource + a verified sender.
- **Google id_token is trusted from the direct exchange** (server-to-server over
  TLS) — no signature re-verify. Safe as-is; do not change the flow to accept
  client-supplied id_tokens without adding `aud`/`iss` verification.
- **Provider flags fail fast.** `GOOGLE_PROVIDER=real` requires
  `GOOGLE_CLIENT_ID/SECRET/CALLBACK_URL`; `EMAIL_PROVIDER=real` requires
  `ACS_EMAIL_ENDPOINT/ACCESS_KEY/SENDER`. A missing value aborts api startup.
- Worker does **not** need any of this (membership is api + portal only).

## Environment reference

| Resource | Value |
|---|---|
| Resource group | `hrats-prod-rg` |
| ACR | `hratsacr7qmhyxfjdyyl2` (login `hratsacr7qmhyxfjdyyl2.azurecr.io`) |
| Postgres flexible server | `hrats-prod-pg-7qmhyxfjdyyl2` |
| ACA domain | `yellowmoss-b9b985f7.southeastasia.azurecontainerapps.io` |
| Container apps | `hrats-prod-api`, `hrats-prod-portal`, `hrats-prod-worker`, `hrats-prod-dashboard` |

New env (api): `GOOGLE_PROVIDER`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`,
`GOOGLE_CALLBACK_URL`, `EMAIL_PROVIDER`, `ACS_EMAIL_ENDPOINT`,
`ACS_EMAIL_ACCESS_KEY`, `ACS_EMAIL_SENDER`, `CANDIDATE_SESSION_TTL` (720h),
`CANDIDATE_SESSION_COOKIE` (cp_session), `EMAIL_OTP_TTL` (10m). Migration **000013**.

---

## 0) Preflight — confirm names & set shell vars

```bash
RG=hrats-prod-rg
ACR=hratsacr7qmhyxfjdyyl2
ACR_LOGIN=hratsacr7qmhyxfjdyyl2.azurecr.io
PG=hrats-prod-pg-7qmhyxfjdyyl2
TAG=$(git rev-parse HEAD | cut -c1-12)        # on main @ 5094fcc or later
az account show -o table                        # correct MPN sub?

# Resolve the EXACT public FQDNs — the cookie/CORS/callback depend on them.
API_FQDN=$(az containerapp show -n hrats-prod-api     -g $RG --query properties.configuration.ingress.fqdn -o tsv)
PORTAL_FQDN=$(az containerapp show -n hrats-prod-portal -g $RG --query properties.configuration.ingress.fqdn -o tsv)
echo "api=https://$API_FQDN"
echo "portal=https://$PORTAL_FQDN"
# Confirm the portal app name if the above errors:
# az containerapp list -g $RG -o table
```

**Obtain before proceeding:** `GOOGLE_CLIENT_ID/SECRET` (step 1); ACS
`ACS_EMAIL_ENDPOINT/ACCESS_KEY/SENDER` (step 2); your public IP for the temp PG
firewall rule (step 3): `MYIP=$(curl -s ifconfig.me)`.

---

## 1) Google OAuth client (Google Cloud Console — not az)

1. Console → **APIs & Services → Credentials → Create OAuth client ID** → **Web application**.
2. **Authorized redirect URI** (must match EXACTLY):
   `https://<API_FQDN>/api/v1/public/auth/google/callback`
3. OAuth consent screen: scopes `openid`, `email`, `profile`; publish (or add test users).
4. Copy **Client ID** + **Client secret** → `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`.

---

## 2) Provision Azure Communication Services Email

> Needs `az extension add --name communication`. Flags vary by version — verify with `--help`.

```bash
ACS=hrats-prod-acs
ACS_EMAIL=hrats-prod-email
DATA_LOC="Asia Pacific"     # data residency; ACS resources are location=global

# 2a. Communication Services + Email resources
az communication create        --name $ACS       -g $RG --location global --data-location "$DATA_LOC"
az communication email create  --name $ACS_EMAIL -g $RG --location global --data-location "$DATA_LOC"

# 2b. Azure-managed sender domain (auto-verified, no DNS; rate-limited — fine for OTP)
az communication email domain create \
  --domain-name AzureManagedDomain --email-service-name $ACS_EMAIL -g $RG \
  --location global --domain-management AzureManaged

# 2c. Verified sender address (DoNotReply@<guid>.azurecomm.net)
SENDER_DOMAIN=$(az communication email domain show \
  --domain-name AzureManagedDomain --email-service-name $ACS_EMAIL -g $RG \
  --query fromSenderDomain -o tsv)
ACS_EMAIL_SENDER="DoNotReply@$SENDER_DOMAIN"; echo "sender=$ACS_EMAIL_SENDER"

# 2d. LINK the domain to the Communication Services resource (required to send)
DOMAIN_ID=$(az communication email domain show \
  --domain-name AzureManagedDomain --email-service-name $ACS_EMAIL -g $RG --query id -o tsv)
az communication update --name $ACS -g $RG --linked-domains "$DOMAIN_ID"

# 2e. Endpoint + base64 access key (from the connection string)
ACS_CONN=$(az communication list-key --name $ACS -g $RG --query primaryConnectionString -o tsv)
ACS_EMAIL_ENDPOINT=$(echo "$ACS_CONN" | sed -n 's/.*endpoint=\([^;]*\).*/\1/p' | sed 's:/*$::')
ACS_EMAIL_ACCESS_KEY=$(echo "$ACS_CONN" | sed -n 's/.*accesskey=\([^;]*\).*/\1/p')
echo "endpoint=$ACS_EMAIL_ENDPOINT"
```

> The code wants `ACS_EMAIL_ENDPOINT` (host URL, trailing slash optional) and the
> **base64 access key** — NOT the full connection string.

---

## 3) Apply migration 000013 to prod `hr_db`

```bash
DBURL=$(az containerapp secret show -n hrats-prod-api -g $RG --secret-name db-url --query value -o tsv)

MYIP=$(curl -s ifconfig.me)
az postgres flexible-server firewall-rule create -g $RG --server-name $PG \
  --name tmp-migrate-membership --start-ip-address "$MYIP" --end-ip-address "$MYIP"

~/go/bin/migrate -path backend/migrations -database "$DBURL" version   # expect 12
~/go/bin/migrate -path backend/migrations -database "$DBURL" up        # → 13
~/go/bin/migrate -path backend/migrations -database "$DBURL" version   # confirm 13

# ALWAYS remove the rule afterwards
az postgres flexible-server firewall-rule delete -g $RG --server-name $PG \
  --name tmp-migrate-membership --yes
```

> If `schema_migrations` is dirty/drifted (it has been before), `migrate force <v>`
> then `up` — 000013 is idempotent (`CREATE TABLE IF NOT EXISTS` / `ADD COLUMN IF NOT EXISTS`).

---

## 4) Set ACA secrets + env on the api

```bash
# 4a. Secrets
az containerapp secret set -n hrats-prod-api -g $RG --secrets \
  google-client-secret="$GOOGLE_CLIENT_SECRET" \
  acs-email-access-key="$ACS_EMAIL_ACCESS_KEY"

# 4b. Env (cookie auto SameSite=None;Secure because ENV != development)
az containerapp update -n hrats-prod-api -g $RG --set-env-vars \
  GOOGLE_PROVIDER=real \
  GOOGLE_CLIENT_ID="$GOOGLE_CLIENT_ID" \
  GOOGLE_CLIENT_SECRET=secretref:google-client-secret \
  GOOGLE_CALLBACK_URL="https://$API_FQDN/api/v1/public/auth/google/callback" \
  EMAIL_PROVIDER=real \
  ACS_EMAIL_ENDPOINT="$ACS_EMAIL_ENDPOINT" \
  ACS_EMAIL_ACCESS_KEY=secretref:acs-email-access-key \
  ACS_EMAIL_SENDER="$ACS_EMAIL_SENDER" \
  CANDIDATE_SESSION_TTL=720h \
  CANDIDATE_SESSION_COOKIE=cp_session \
  EMAIL_OTP_TTL=10m

# 4c. CRITICAL: CORS must include the exact portal origin (no wildcard)
az containerapp show -n hrats-prod-api -g $RG \
  --query "properties.template.containers[0].env[?name=='CORS_ALLOW_ORIGINS'].value" -o tsv
# If https://$PORTAL_FQDN is missing, append it (keep existing origins):
# az containerapp update -n hrats-prod-api -g $RG --set-env-vars \
#   CORS_ALLOW_ORIGINS="https://$PORTAL_FQDN,<existing-origins>"
```

> Setting env spawns a new revision running the **old image** — fine; deploy is next.

---

## 5) Build + deploy the api image

```bash
az acr build -r $ACR -t hr-ats/api:$TAG -t hr-ats/api:latest \
  --build-arg SVC=api -f backend/Dockerfile backend
az containerapp update -n hrats-prod-api -g $RG --image $ACR_LOGIN/hr-ats/api:$TAG
```

---

## 6) Build + deploy the portal image

> Portal carries the new signup/login/account pages → must be rebuilt.
> Build-arg `NEXT_PUBLIC_API_URL` only (no Entra args — those belong to the dashboard).

```bash
az acr build -r $ACR -t hr-ats/portal:$TAG -t hr-ats/portal:latest \
  --build-arg NEXT_PUBLIC_API_URL="https://$API_FQDN" \
  -f career-portal/Dockerfile career-portal
az containerapp update -n hrats-prod-portal -g $RG --image $ACR_LOGIN/hr-ats/portal:$TAG
```

---

## 7) Verify live

```bash
curl -s https://$API_FQDN/health | head -c 200; echo

# email-OTP start (enumeration-safe 200) — then check the inbox for the code
curl -s -X POST https://$API_FQDN/api/v1/public/auth/email/start \
  -H 'Content-Type: application/json' -H "Origin: https://$PORTAL_FQDN" \
  -d '{"email":"you@yourdomain.com"}'; echo

# OAuth login should 302 to the provider
curl -s -o /dev/null -w "google=%{http_code}\n" "https://$API_FQDN/api/v1/public/auth/google/login?return=https://$PORTAL_FQDN/jobs"
curl -s -o /dev/null -w "line=%{http_code}\n"   "https://$API_FQDN/api/v1/public/line/login?return=https://$PORTAL_FQDN/jobs"

# CSRF guard: cross-origin POST rejected
curl -s -o /dev/null -w "csrf=%{http_code} (expect 403)\n" -X POST \
  -H 'Origin: https://evil.example.com' -H 'Content-Type: application/json' \
  https://$API_FQDN/api/v1/public/auth/profile -d '{}'
```

**Browser smoke (the real test):** `https://<PORTAL_FQDN>/signup` → complete LINE,
Google, and email-OTP signups → confirm you stay logged in after a refresh (cookie
persists) → apply to a job with the saved resume. If login doesn't persist, recheck
CORS origin + that the cookie shows `SameSite=None; Secure` in devtools.

---

## 8) Rollback

```bash
# Code: pin api/portal to the previous image tag
az containerapp update -n hrats-prod-api    -g $RG --image $ACR_LOGIN/hr-ats/api:<PREV_TAG>
az containerapp update -n hrats-prod-portal -g $RG --image $ACR_LOGIN/hr-ats/portal:<PREV_TAG>
# Providers: flip back to mock without a redeploy
az containerapp update -n hrats-prod-api -g $RG --set-env-vars GOOGLE_PROVIDER=mock EMAIL_PROVIDER=mock
# DB: migration is additive — only roll back if truly necessary (drops the 3 tables)
# ~/go/bin/migrate -path backend/migrations -database "$DBURL" down 1
```

---

## 9) Security follow-through
- **Rotate the still-exposed LINE channel secret** in the LINE console, then re-set
  the `line-channel-secret` ACA secret (see `docs`/memory `line-login-notify-live`).
  It does not auto-redeploy.
- `unset GOOGLE_CLIENT_SECRET ACS_EMAIL_ACCESS_KEY` after the run — treat the shell
  vars as sensitive.
- Open hardening follow-ups: **#59** per-email OTP brute-force throttle, **#60**
  sweep expired `email_otps` / `candidate_sessions` rows.

## Caveats
- `az communication` sub-commands/flags differ across extension versions — the
  managed-domain linking + `fromSenderDomain` query are the most likely to need
  adjustment. Verify each with `--help`.
- ACS Azure-managed domains have **low send limits** (fine for current OTP volume;
  move to a custom verified domain before scaling).
- Long-term: put api + portal under one **custom registrable domain** so the cookie
  can drop to `SameSite=Lax` (simpler + safer than `None`).
