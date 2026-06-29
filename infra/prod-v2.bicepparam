// =============================================================================
// prod-v2 — greenfield redeploy onto a NEW Pay-As-You-Go subscription (Nexto).
// =============================================================================
// Context: the original MPN credit subscription was disabled (credit exhausted)
// and its data is unrecoverable. This deploys the full stack fresh onto a new
// PAYG sub. No data migration — schema is built from migrations 000001..000049
// and reference data is re-seeded after deploy.
//
// Deploy (after the new sub exists + Azure OpenAI access is granted on it):
//   az account set --subscription <NEW_PAYG_SUB_ID>
//   az group create -n hrats-prod-rg -l southeastasia
//   PG_ADMIN_PASSWORD=$(openssl rand -base64 24) \
//   JWT_SECRET=$(openssl rand -base64 32) \
//   az deployment group create -g hrats-prod-rg \
//     -f infra/main.bicep -p infra/prod-v2.bicepparam \
//     -p imageTag=<git-sha> \
//     -p postgresAdminPassword="$PG_ADMIN_PASSWORD" -p jwtSecret="$JWT_SECRET"
//
// NOTE: main.bicep hardcodes LINE_PROVIDER / NOTIFY_PROVIDER / PS_PROVIDER = mock.
// "Real all from day 1" therefore needs a post-deploy `az containerapp update`
// on the api/worker (and portal rebuild) wiring the real LINE / email (ACS) /
// Teams / Google credentials — none of which were recoverable from the dead sub.
// =============================================================================

using './main.bicep'

param location = 'southeastasia'

// Azure OpenAI / Document Intelligence region. swedencentral has gpt-5-mini
// GlobalStandard quota (500K TPM) on this VS sub; eastus quota is 0 for all
// current models (verified via `cognitiveservices usage list`).
param aiLocation = 'swedencentral'

param resourcePrefix = 'hrats'
param environmentName = 'prod'

param imageTag = 'v1' // first greenfield build tag

// In-cluster Redis (Container App) instead of managed Azure Cache for Redis —
// faster to provision, cheaper, and avoids the retiring Basic tier. Right choice
// on a VS/MSDN credit (pilot) subscription. Queue is transient so ephemeral is OK.
param redisAsContainer = true

// Current GA mini-tier chat model (gpt-4o family is deprecating and cannot be
// deployed new). gpt-5-mini is the cost/quality successor to gpt-4o-mini.
// NOTE: gpt-5 family may need an app-side API-version/param update (verify the
// AI client after deploy; provider seam falls back to mock if incompatible).
param openAiDeploymentName = 'gpt-5-mini'
param openAiModelName = 'gpt-5-mini'
param openAiModelVersion = '2025-08-07'

param postgresAdminLogin = 'hratsadmin'

// Supplied at deploy time — never commit secret values.
param postgresAdminPassword = readEnvironmentVariable('PG_ADMIN_PASSWORD', '')
param jwtSecret = readEnvironmentVariable('JWT_SECRET', '')

param retentionDays = ''

// --- AI: Azure OpenAI gpt-5-mini in swedencentral (quota verified there).
// eastus had 0 quota; gpt-4o/4.1-mini are all deprecating so gpt-5-mini is the
// only viable current model. App's 3 Azure clients are patched for gpt-5
// (api-version + max_completion_tokens + no temperature). deployAi=true creates
// the OpenAI + DocIntel accounts and flips AI_PROVIDER=azure on the apps.
param deployAi = true

// --- Search: start on Postgres trigram (free); enable Azure AI Search later ---
param deploySearch = false

// --- Entra SSO: reuse the EXISTING app reg (survived the dead sub) ------------
// Add the NEW dashboard FQDN to this app's SPA redirect URIs after the env exists.
param deployEntra = true
param azureAdTenantId = 'aaabefb4-0433-494c-b50e-67b0f4b5f05c'
param azureAdClientId = '57c7d338-47be-4726-bec5-560853620d1f'
param azureAdAllowedTenants = ''

// rbacMode (MI + Key Vault) and managed Redis are the main.bicep defaults — a
// PAYG sub supports both, so they are intentionally left at default.
