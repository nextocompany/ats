// v1 (career-portal-first) parameter values for main.bicep.
//
// Secrets (postgresAdminPassword, jwtSecret) are intentionally NOT hardcoded
// here. Supply them at deploy time, e.g.:
//   az deployment group create -g <rg> -f infra/main.bicep -p infra/main.bicepparam \
//     -p postgresAdminPassword=$(openssl rand -base64 24) \
//     -p jwtSecret=$(openssl rand -base64 32)
// or use getSecret() from a bootstrap Key Vault. Never commit real secret values.

using './main.bicep'

param location = 'southeastasia'

// Azure OpenAI / Document Intelligence region. gpt-4o capacity in southeastasia
// is not guaranteed — change this (e.g. to 'eastus' or 'australiaeast') if a
// deployment fails on model availability.
param aiLocation = 'southeastasia'

param resourcePrefix = 'hrats'
param environmentName = 'prod'

param imageTag = 'latest'

param openAiDeploymentName = 'gpt-4o'
param openAiModelName = 'gpt-4o'

param postgresAdminLogin = 'hratsadmin'

// Supplied at deploy time — see header. Placeholder env-var reads so the param
// file is self-documenting; replace with your secret injection mechanism.
param postgresAdminPassword = readEnvironmentVariable('PG_ADMIN_PASSWORD', '')
param jwtSecret = readEnvironmentVariable('JWT_SECRET', '')

// PDPA retention window left at in-app default; sweep stays disabled in v1.
param retentionDays = ''

// Phase-2 toggles — OFF for v1 (career-portal-first).
param deploySearch = false
param deployEntra = false
