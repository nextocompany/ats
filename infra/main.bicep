// =============================================================================
// HR ATS — Azure Container Apps infrastructure (v1, career-portal-first scope)
// =============================================================================
//
// Resource-group-scoped deployment. Provisions the full v1 platform:
//   - Log Analytics + Application Insights
//   - Azure Container Registry (Basic, admin disabled, MI/RBAC pull)
//   - Key Vault (RBAC mode) with all workload secrets
//   - Postgres Flexible Server (v16, B1ms) + hr_db
//   - Azure Cache for Redis (Basic C0)
//   - Storage account + private "resumes" blob container
//   - Azure OpenAI (gpt-4o deployment) + Document Intelligence
//   - Container Apps environment
//   - 5 Container Apps: api, worker, scheduler, portal, dashboard
//
// All apps run under one user-assigned managed identity (AcrPull + Key Vault
// Secrets User). Secrets reach the apps via the ACA keyVaultUrl secret syntax —
// no secret values are templated into the apps themselves.
//
// Phase-2 toggles (deploySearch, deployEntra) default OFF so v1 stays
// career-portal-first. Flipping them to true is the documented phase-2 path.
// =============================================================================

targetScope = 'resourceGroup'

// ---------------------------------------------------------------------------
// Parameters
// ---------------------------------------------------------------------------

@description('Primary region for app + data resources.')
param location string = 'southeastasia'

@description('''
Region for Azure OpenAI / Document Intelligence. Model availability differs by
region; southeastasia is the default but may need to change (e.g. to a region
with gpt-4o capacity) — verify model availability before deploying.
''')
param aiLocation string = 'southeastasia'

@description('Resource name prefix.')
param resourcePrefix string = 'hrats'

@description('Environment name segment (e.g. prod).')
param environmentName string = 'prod'

@description('Container image tag deployed to all apps (e.g. latest, or a git SHA).')
param imageTag string = 'latest'

@description('Azure OpenAI model deployment name (read by the app as AZURE_OPENAI_DEPLOYMENT).')
param openAiDeploymentName string = 'gpt-4o'

@description('Azure OpenAI model name.')
param openAiModelName string = 'gpt-4o'

@description('Postgres administrator login.')
param postgresAdminLogin string = 'hratsadmin'

@description('Postgres administrator password (secure). Generate/supply at deploy time.')
@secure()
param postgresAdminPassword string

@description('JWT signing secret for the API (secure). Generate/supply at deploy time.')
@secure()
param jwtSecret string

@description('PDPA retention window in days. Empty disables an explicit window (defaults apply in-app).')
param retentionDays string = ''

// --- Phase-2 toggles (default OFF; do NOT provision in v1) -----------------

@description('Phase-2: provision Azure AI Search. Keep false for v1.')
param deploySearch bool = false

@description('Phase-2: wire Entra (real AUTH_PROVIDER) inputs. Keep false for v1.')
param deployEntra bool = false

// --- Cost-lean / thin-pilot toggles ----------------------------------------

@description('Provision Azure OpenAI + Document Intelligence. Set false on subscriptions without OpenAI access (e.g. MPN/credit) — backend then runs AI_PROVIDER=mock.')
param deployAi bool = true

@description('Scale stateless apps (api/worker/portal/dashboard) to 0 when idle to conserve credit. Scheduler always stays at 1 replica.')
param scaleToZero bool = false

// ---------------------------------------------------------------------------
// Naming
// ---------------------------------------------------------------------------

var prefix = '${resourcePrefix}-${environmentName}'
var uniq = uniqueString(resourceGroup().id)

// Globally-unique names need the uniqueString suffix; ACR/storage are
// alphanumeric-only.
var acrName = toLower('${resourcePrefix}acr${uniq}')
var storageName = toLower('${resourcePrefix}st${uniq}')
var keyVaultName = take(toLower('${resourcePrefix}-kv-${uniq}'), 24)
var openAiName = toLower('${prefix}-openai-${uniq}')
var docIntelName = toLower('${prefix}-docintel-${uniq}')
var redisName = toLower('${prefix}-redis-${uniq}')
var postgresName = toLower('${prefix}-pg-${uniq}')

var workspaceName = '${prefix}-logs'
var appInsightsName = '${prefix}-appi'
var acaEnvName = '${prefix}-cae'
var identityName = '${prefix}-id'

// Container app names (consumed by the CD workflow + runbook).
var apiAppName = '${prefix}-api'
var workerAppName = '${prefix}-worker'
var schedulerAppName = '${prefix}-scheduler'
var portalAppName = '${prefix}-portal'
var dashboardAppName = '${prefix}-dashboard'

// ---------------------------------------------------------------------------
// Shared user-assigned managed identity (AcrPull + Key Vault Secrets User)
// ---------------------------------------------------------------------------

resource appIdentity 'Microsoft.ManagedIdentity/userAssignedIdentities@2023-01-31' = {
  name: identityName
  location: location
}

// ---------------------------------------------------------------------------
// Observability
// ---------------------------------------------------------------------------

module monitoring 'modules/monitoring.bicep' = {
  name: 'monitoring'
  params: {
    location: location
    workspaceName: workspaceName
    appInsightsName: appInsightsName
  }
}

// ---------------------------------------------------------------------------
// Container Registry (grants AcrPull to the app identity)
// ---------------------------------------------------------------------------

module registry 'modules/registry.bicep' = {
  name: 'registry'
  params: {
    location: location
    registryName: acrName
    pullIdentityPrincipalId: appIdentity.properties.principalId
  }
}

// ---------------------------------------------------------------------------
// Data plane
// ---------------------------------------------------------------------------

module postgres 'modules/postgres.bicep' = {
  name: 'postgres'
  params: {
    location: location
    serverName: postgresName
    databaseName: 'hr_db'
    administratorLogin: postgresAdminLogin
    administratorPassword: postgresAdminPassword
  }
}

module redis 'modules/redis.bicep' = {
  name: 'redis'
  params: {
    location: location
    redisName: redisName
  }
}

module storage 'modules/storage.bicep' = {
  name: 'storage'
  params: {
    location: location
    storageAccountName: storageName
    containerName: 'resumes'
  }
}

// ---------------------------------------------------------------------------
// AI plane (separate region — model availability differs)
// Conditional: skipped entirely when deployAi=false (MPN/credit subscriptions
// without OpenAI access). When absent these modules expose no outputs, so every
// reference below is guarded by deployAi.
// ---------------------------------------------------------------------------

module openai 'modules/openai.bicep' = if (deployAi) {
  name: 'openai'
  params: {
    location: aiLocation
    accountName: openAiName
    deploymentName: openAiDeploymentName
    modelName: openAiModelName
  }
}

module docintel 'modules/docintel.bicep' = if (deployAi) {
  name: 'docintel'
  params: {
    location: aiLocation
    accountName: docIntelName
  }
}

// ---------------------------------------------------------------------------
// Secret composition (kept local; passed securely into Key Vault module)
// ---------------------------------------------------------------------------

// libpq/URL form expected by lib/pq. sslmode=require because the flexible
// server enforces TLS.
var dbUrl = 'postgres://${postgresAdminLogin}:${postgresAdminPassword}@${postgres.outputs.fqdn}:5432/${postgres.outputs.databaseName}?sslmode=require'

// TLS Redis URL on port 6380. Password-only auth (empty username).
var redisUrl = 'rediss://:${redis.outputs.primaryKey}@${redis.outputs.hostName}:${redis.outputs.sslPort}'

module keyVault 'modules/keyvault.bicep' = {
  name: 'keyvault'
  params: {
    location: location
    keyVaultName: keyVaultName
    rbacPrincipalObjectId: appIdentity.properties.principalId
    dbUrl: dbUrl
    redisUrl: redisUrl
    jwtSecret: jwtSecret
    blobConnString: storage.outputs.connectionString
    // Conditional modules expose no outputs when deployAi=false; the safe-access
    // operator (.?) yields null, and ?? falls back to '' so no empty AI secrets
    // are written (createAiSecrets gates that). Avoids BCP318 on a possibly-null
    // conditional module.
    openAiKey: deployAi ? (openai.?outputs.key ?? '') : ''
    docIntelKey: deployAi ? (docintel.?outputs.key ?? '') : ''
    createAiSecrets: deployAi
  }
}

// ---------------------------------------------------------------------------
// Container Apps environment
// ---------------------------------------------------------------------------

module acaEnv 'modules/container-env.bicep' = {
  name: 'aca-env'
  params: {
    location: location
    environmentName: acaEnvName
    logAnalyticsCustomerId: monitoring.outputs.workspaceCustomerId
    logAnalyticsWorkspaceId: monitoring.outputs.workspaceId
  }
}

// ---------------------------------------------------------------------------
// Derived URLs / image refs
// ---------------------------------------------------------------------------

var acrLoginServer = registry.outputs.loginServer
var apiImage = '${acrLoginServer}/hr-ats/api:${imageTag}'
var workerImage = '${acrLoginServer}/hr-ats/worker:${imageTag}'
var schedulerImage = '${acrLoginServer}/hr-ats/scheduler:${imageTag}'
var portalImage = '${acrLoginServer}/hr-ats/career-portal:${imageTag}'
var dashboardImage = '${acrLoginServer}/hr-ats/dashboard:${imageTag}'

// FQDNs are predictable from the env default domain, which lets us wire
// PORTAL_BASE_URL / CORS without a deploy ordering cycle between apps.
var envDomain = acaEnv.outputs.defaultDomain
var portalFqdn = '${portalAppName}.${envDomain}'
var dashboardFqdn = '${dashboardAppName}.${envDomain}'
var portalUrl = 'https://${portalFqdn}'
var dashboardUrl = 'https://${dashboardFqdn}'

// ---------------------------------------------------------------------------
// Key Vault secret references (ACA keyVaultUrl syntax)
// ---------------------------------------------------------------------------

var kvUri = keyVault.outputs.vaultUri

// AI secrets/env are only present when deployAi — the openai-key/docintel-key
// vault secrets are not created otherwise, so referencing them would break the
// app. Concatenate conditionally to keep every array valid in both modes.
var coreKeyVaultSecrets = [
  { name: 'db-url', keyVaultUrl: '${kvUri}secrets/db-url' }
  { name: 'redis-url', keyVaultUrl: '${kvUri}secrets/redis-url' }
  { name: 'jwt-secret', keyVaultUrl: '${kvUri}secrets/jwt-secret' }
  { name: 'blob-conn-string', keyVaultUrl: '${kvUri}secrets/blob-conn-string' }
]
var aiKeyVaultSecrets = [
  { name: 'openai-key', keyVaultUrl: '${kvUri}secrets/openai-key' }
  { name: 'docintel-key', keyVaultUrl: '${kvUri}secrets/docintel-key' }
]
var backendKeyVaultSecrets = concat(coreKeyVaultSecrets, deployAi ? aiKeyVaultSecrets : [])

var coreSecretEnv = [
  { name: 'DB_URL', secretRef: 'db-url' }
  { name: 'REDIS_URL', secretRef: 'redis-url' }
  { name: 'JWT_SECRET', secretRef: 'jwt-secret' }
  { name: 'AZURE_BLOB_CONNECTION_STRING', secretRef: 'blob-conn-string' }
]
var aiSecretEnv = [
  { name: 'AZURE_OPENAI_KEY', secretRef: 'openai-key' }
  { name: 'AZURE_DOC_INTEL_KEY', secretRef: 'docintel-key' }
]
var backendSecretEnv = concat(coreSecretEnv, deployAi ? aiSecretEnv : [])

// Plain backend env shared by api/worker/scheduler. AUTH_PROVIDER stays mock in
// v1 (HR dashboard login fails closed until Entra/phase-2). AI_SEARCH_PROVIDER
// stays mock unless deploySearch flips on (phase-2).
// AI endpoint env vars reference conditional-module outputs, so they only join
// the array when deployAi. AI_PROVIDER flips to mock when AI is not provisioned.
var aiPlainEnv = [
  { name: 'AZURE_OPENAI_ENDPOINT', value: openai.?outputs.endpoint ?? '' }
  { name: 'AZURE_OPENAI_DEPLOYMENT', value: openai.?outputs.deploymentName ?? '' }
  { name: 'AZURE_DOC_INTEL_ENDPOINT', value: docintel.?outputs.endpoint ?? '' }
]

var basePlainEnv = [
  { name: 'ENV', value: 'production' }
  { name: 'HTTP_PORT', value: '8080' }
  { name: 'WORKER_PORT', value: '8081' }
  { name: 'AZURE_BLOB_CONTAINER', value: 'resumes' }
  { name: 'AI_PROVIDER', value: deployAi ? 'azure' : 'mock' }
  { name: 'AI_SEARCH_PROVIDER', value: deploySearch ? 'azure' : 'mock' }
  { name: 'AUTH_PROVIDER', value: deployEntra ? 'real' : 'mock' }
  { name: 'LINE_PROVIDER', value: 'mock' }
  { name: 'NOTIFY_PROVIDER', value: 'mock' }
  { name: 'PS_PROVIDER', value: 'mock' }
  { name: 'PORTAL_BASE_URL', value: portalUrl }
  { name: 'CORS_ALLOW_ORIGINS', value: '${portalUrl},${dashboardUrl}' }
  { name: 'RETENTION_DAYS', value: retentionDays }
  { name: 'RETENTION_SWEEP_ENABLED', value: 'false' }
  { name: 'APPLICATIONINSIGHTS_CONNECTION_STRING', value: monitoring.outputs.appInsightsConnectionString }
]

var backendPlainEnv = concat(basePlainEnv, deployAi ? aiPlainEnv : [])

// ---------------------------------------------------------------------------
// Container Apps
// ---------------------------------------------------------------------------

// API — public HTTP ingress on :8080, scale 1..3.
module apiApp 'modules/container-app.bicep' = {
  name: 'app-api'
  params: {
    name: apiAppName
    location: location
    environmentId: acaEnv.outputs.id
    identityId: appIdentity.id
    acrLoginServer: acrLoginServer
    image: apiImage
    ingressMode: 'external'
    targetPort: 8080
    minReplicas: scaleToZero ? 0 : 1
    maxReplicas: 3
    envVars: backendPlainEnv
    keyVaultSecrets: backendKeyVaultSecrets
    secretEnvVars: backendSecretEnv
  }
}

// Worker — internal ingress on :8081 (it binds WORKER_PORT), scale 1..3.
module workerApp 'modules/container-app.bicep' = {
  name: 'app-worker'
  params: {
    name: workerAppName
    location: location
    environmentId: acaEnv.outputs.id
    identityId: appIdentity.id
    acrLoginServer: acrLoginServer
    image: workerImage
    ingressMode: 'internal'
    targetPort: 8081
    minReplicas: scaleToZero ? 0 : 1
    maxReplicas: 3
    envVars: backendPlainEnv
    keyVaultSecrets: backendKeyVaultSecrets
    secretEnvVars: backendSecretEnv
  }
}

// Scheduler — NO ingress, single replica (load-bearing cron, hard 1..1).
module schedulerApp 'modules/container-app.bicep' = {
  name: 'app-scheduler'
  params: {
    name: schedulerAppName
    location: location
    environmentId: acaEnv.outputs.id
    identityId: appIdentity.id
    acrLoginServer: acrLoginServer
    image: schedulerImage
    ingressMode: 'none'
    minReplicas: 1
    maxReplicas: 1
    envVars: backendPlainEnv
    keyVaultSecrets: backendKeyVaultSecrets
    secretEnvVars: backendSecretEnv
  }
}

// Career portal — public, :3000. NEXT_PUBLIC_API_URL is baked at build time,
// so the web apps need no runtime secrets.
module portalApp 'modules/container-app.bicep' = {
  name: 'app-portal'
  params: {
    name: portalAppName
    location: location
    environmentId: acaEnv.outputs.id
    identityId: appIdentity.id
    acrLoginServer: acrLoginServer
    image: portalImage
    ingressMode: 'external'
    targetPort: 3000
    minReplicas: scaleToZero ? 0 : 1
    maxReplicas: 3
    envVars: [
      { name: 'NODE_ENV', value: 'production' }
    ]
  }
}

// Dashboard — public, :3000.
module dashboardApp 'modules/container-app.bicep' = {
  name: 'app-dashboard'
  params: {
    name: dashboardAppName
    location: location
    environmentId: acaEnv.outputs.id
    identityId: appIdentity.id
    acrLoginServer: acrLoginServer
    image: dashboardImage
    ingressMode: 'external'
    targetPort: 3000
    minReplicas: scaleToZero ? 0 : 1
    maxReplicas: 3
    envVars: [
      { name: 'NODE_ENV', value: 'production' }
    ]
  }
}

// ---------------------------------------------------------------------------
// Outputs (consumed by the CD workflow + runbook)
// ---------------------------------------------------------------------------

@description('ACR login server for docker push / image refs.')
output acrLoginServer string = acrLoginServer

@description('ACR resource name.')
output acrName string = registry.outputs.name

@description('User-assigned managed identity resource ID (apps + CI image pull).')
output appIdentityId string = appIdentity.id

@description('User-assigned managed identity client ID.')
output appIdentityClientId string = appIdentity.properties.clientId

@description('Key Vault name.')
output keyVaultName string = keyVault.outputs.name

@description('Key Vault base URI.')
output keyVaultUri string = kvUri

@description('Container Apps environment name.')
output containerAppsEnvironmentName string = acaEnv.outputs.name

@description('Container Apps environment default domain.')
output containerAppsEnvironmentDomain string = envDomain

@description('API container app name.')
output apiAppName string = apiApp.outputs.name

@description('API public FQDN.')
output apiFqdn string = apiApp.outputs.fqdn

@description('Worker container app name.')
output workerAppName string = workerApp.outputs.name

@description('Scheduler container app name (single replica, no ingress).')
output schedulerAppName string = schedulerApp.outputs.name

@description('Career portal container app name.')
output portalAppName string = portalApp.outputs.name

@description('Career portal public FQDN.')
output portalFqdn string = portalApp.outputs.fqdn

@description('Dashboard container app name.')
output dashboardAppName string = dashboardApp.outputs.name

@description('Dashboard public FQDN.')
output dashboardFqdn string = dashboardApp.outputs.fqdn

@description('Postgres server FQDN/host.')
output postgresHost string = postgres.outputs.fqdn

@description('Postgres database name.')
output postgresDatabase string = postgres.outputs.databaseName

@description('Redis hostname.')
output redisHost string = redis.outputs.hostName

@description('Storage account name.')
output storageAccountName string = storage.outputs.name

@description('Resume blob container name.')
output blobContainerName string = storage.outputs.containerName

@description('Azure OpenAI endpoint (empty when deployAi=false).')
output openAiEndpoint string = openai.?outputs.endpoint ?? ''

@description('Azure OpenAI deployment name (AZURE_OPENAI_DEPLOYMENT; empty when deployAi=false).')
output openAiDeployment string = openai.?outputs.deploymentName ?? ''

@description('Document Intelligence endpoint (empty when deployAi=false).')
output docIntelEndpoint string = docintel.?outputs.endpoint ?? ''

@description('Application Insights connection string.')
output appInsightsConnectionString string = monitoring.outputs.appInsightsConnectionString
