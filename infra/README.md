# HR·ATS — Azure go-live runbook (v1: career-portal-first)

Infrastructure-as-code + the operator steps to stand up the AI HR recruitment
platform on **Azure Container Apps** in **Southeast Asia (Singapore)**.

> **Provisioning model:** these files are reviewable IaC. **You run the `az`
> commands** (Claude does not hold your Azure credentials). Run them in this
> session with the `! <command>` prefix, or in your own terminal.

---

## What v1 provisions (career-portal-first)

Real, live services:

| Service | Azure resource | Used for |
|---|---|---|
| Postgres 16 | Flexible Server (B1ms) + `hr_db` | system of record |
| Redis | Azure Cache for Redis (Basic C0) | queue / rate-limit |
| Blob | Storage account + private `resumes` container | CV / resume storage |
| Azure OpenAI | `gpt-4o` deployment | candidate scoring |
| Document Intelligence | FormRecognizer | CV OCR / parsing |
| Registry | ACR (Basic) | container images |
| Secrets | Key Vault (RBAC) | all connection strings/keys |
| Observability | Log Analytics + App Insights | logs/metrics |
| Compute | Container Apps env + 5 apps | api, worker, scheduler, portal, dashboard |

Kept **mock** in v1 (flip on in phase 2): LINE login/notify, Azure AI Search,
**Entra SSO**. See the auth caveat below.

```
infra/
├── main.bicep          # RG-scoped orchestrator (params + 25 outputs)
├── main.bicepparam     # v1 values; secrets injected from env at deploy time
└── modules/            # monitoring, registry, keyvault, postgres, redis,
                        # storage, openai, docintel, container-env, container-app
```

---

## ⚠️ Read before you start

1. **HR dashboard login fails closed in production.** v1 ships `AUTH_PROVIDER=mock`;
   in prod the real Entra JWT path is required, so HR staff **cannot log into the
   dashboard on the prod URL** until phase 2 (set `deployEntra=true`, supply
   `AZURE_AD_TENANT_ID` / `AZURE_AD_CLIENT_ID`, redeploy). The **public career
   portal is fully functional** — which is the whole point of "career-portal-first."
   For HR UAT before Entra, run the dashboard on a separate staging env with dev auth.
2. **Azure OpenAI requires approved access** and **`gpt-4o` capacity in your region**.
   Singapore (`southeastasia`) may not have capacity — set `aiLocation` to a region
   that does (e.g. `eastus`, `australiaeast`, `swedencentral`). Apply for access at
   https://aka.ms/oai/access *before* go-live (lead time can be days).
3. **Cost:** these are billable resources. B1ms Postgres + Basic Redis + ACA
   consumption + OpenAI usage. Tear down with `az group delete -n <rg>` when piloting.

---

## Prerequisites

- `az` CLI ≥ 2.60 + the `containerapp` extension: `az extension add -n containerapp`
- Bicep: `az bicep install`
- For DB migration from your laptop: `migrate` (golang-migrate) **or** Docker
  (`migrate/migrate` image), and `psql` for seeding.
- Owner/Contributor on the target subscription.

---

## Step-by-step

### 0. Login & resource group
```bash
az login
az account set --subscription "<SUBSCRIPTION_ID>"
export RG=hrats-prod-rg
az group create -n "$RG" -l southeastasia
```

### 1. Generate the two secrets (never commit these)
`main.bicepparam` reads them from env vars:
```bash
export PG_ADMIN_PASSWORD="$(openssl rand -base64 24)"
export JWT_SECRET="$(openssl rand -base64 48)"
```

### 2. Deploy the infrastructure
```bash
# If gpt-4o isn't available in southeastasia, add: -p aiLocation=eastus
az deployment group create \
  -g "$RG" \
  -f infra/main.bicep \
  -p infra/main.bicepparam \
  -p postgresAdminPassword="$PG_ADMIN_PASSWORD" jwtSecret="$JWT_SECRET"
```
On the **first** deploy the 5 Container Apps reference images that don't exist in
ACR yet, so their first revision will fail to pull — expected. Their **FQDNs are
already assigned**, which is all the next step needs.

### 3. Build & push the five images, then roll the apps
**Option A — CI (recommended):** configure OIDC (Step 6) and run the
**“Deploy to Azure Container Apps”** GitHub Action. It builds in ACR and updates
every app.

**Option B — locally, server-side build (no Docker needed):**
```bash
ACR=$(az acr list -g "$RG" --query "[0].name" -o tsv)
ACR_LOGIN=$(az acr show -n "$ACR" --query loginServer -o tsv)
API_FQDN=$(az containerapp show -n hrats-prod-api -g "$RG" \
  --query properties.configuration.ingress.fqdn -o tsv)
TAG=v1

# backend (one Dockerfile, three services)
for SVC in api worker scheduler; do
  az acr build -r "$ACR" -t "hr-ats/$SVC:$TAG" --build-arg SVC=$SVC -f backend/Dockerfile backend
done
# web (bake the API origin into the client bundle)
az acr build -r "$ACR" -t "hr-ats/career-portal:$TAG" \
  --build-arg NEXT_PUBLIC_API_URL="https://$API_FQDN" -f career-portal/Dockerfile career-portal
az acr build -r "$ACR" -t "hr-ats/dashboard:$TAG" \
  --build-arg NEXT_PUBLIC_API_URL="https://$API_FQDN" -f frontend/Dockerfile frontend

# roll each app to the tag
for pair in api:hr-ats/api worker:hr-ats/worker scheduler:hr-ats/scheduler \
            portal:hr-ats/career-portal dashboard:hr-ats/dashboard; do
  app="hrats-prod-${pair%%:*}"; repo="${pair#*:}"
  az containerapp update -n "$app" -g "$RG" --image "$ACR_LOGIN/$repo:$TAG"
done
```

### 4. Migrate the database
The schema is applied with golang-migrate (not on boot). Open the Postgres
firewall to your IP, read `DB_URL` from Key Vault, then migrate:
```bash
KV=$(az keyvault list -g "$RG" --query "[0].name" -o tsv)
PG=$(az postgres flexible-server list -g "$RG" --query "[0].name" -o tsv)
MYIP=$(curl -s ifconfig.me)
az postgres flexible-server firewall-rule create -g "$RG" -n "$PG" \
  --rule-name operator --start-ip-address "$MYIP" --end-ip-address "$MYIP"

export DB_URL=$(az keyvault secret show --vault-name "$KV" -n db-url --query value -o tsv)

# golang-migrate (or use the migrate/migrate Docker image)
migrate -path backend/migrations -database "$DB_URL" up
# docker alternative:
# docker run --rm -v "$PWD/backend/migrations:/m" migrate/migrate \
#   -path=/m -database "$DB_URL" up
```

### 5. Seed reference data (positions/stores/vacancies)
```bash
psql "$DB_URL" -f scripts/seed_stores.sql
psql "$DB_URL" -f scripts/seed_positions.sql
psql "$DB_URL" -f scripts/seed_vacancies.sql
# (optional demo candidates)
# psql "$DB_URL" -f scripts/seed_demo_candidates.sql
```
When done, remove the operator firewall rule:
```bash
az postgres flexible-server firewall-rule delete -g "$RG" -n "$PG" --rule-name operator -y
```

### 6. (For CI) OIDC federated credentials
Create an app registration with a federated credential for this repo, then grant it
`AcrPush` + `Contributor` on the resource group and set GitHub
secrets `AZURE_CLIENT_ID` / `AZURE_TENANT_ID` / `AZURE_SUBSCRIPTION_ID` and the
variable `AZURE_RESOURCE_GROUP=hrats-prod-rg`. (`az ad app create` →
`az ad app federated-credential create --subject repo:<org>/<repo>:ref:refs/heads/main`
→ `az role assignment create`.)

### 7. Verify
```bash
az deployment group show -g "$RG" -n main --query properties.outputs
# portal:    https://<portalFqdn>      (public — should fully work)
# api:       https://<apiFqdn>/health  (expect {"success":true,...})
# dashboard: https://<dashboardFqdn>   (reachable; login pending Entra — see caveat)
```

---

## Phase 2 (later)
- **Entra SSO:** `deployEntra=true` + `AZURE_AD_*` → real HR auth (unblocks the dashboard in prod).
- **Azure AI Search:** `deploySearch=true` → switches `AI_SEARCH_PROVIDER=azure`; provision + populate the index.
- **LINE:** real `LINE_PROVIDER`/`NOTIFY_PROVIDER` + channel secrets.
- **PeopleSoft:** real `PS_PROVIDER` + IB credentials.
- **Custom domains + TLS:** bind managed certs to the portal/dashboard/api apps.
- Productionize migrations as an ACA **Job** instead of an operator-run step.
