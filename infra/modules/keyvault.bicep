// Key Vault in RBAC mode. Holds all workload secrets. The container apps'
// user-assigned managed identity is granted Key Vault Secrets User here so the
// vault owns its access grant. Secret values are passed in from main.bicep
// (themselves derived from other modules' outputs / secure params).

@description('Azure region.')
param location string

@description('Globally-unique Key Vault name (3-24 chars, alphanumeric + hyphen).')
param keyVaultName string

@description('Tenant ID for the vault.')
param tenantId string = subscription().tenantId

@description('Object (principal) ID of the user-assigned managed identity granted Key Vault Secrets User.')
param rbacPrincipalObjectId string

@description('Postgres libpq URL (postgres://user:pass@host:5432/db?sslmode=require).')
@secure()
param dbUrl string

@description('Redis URL (rediss://:<key>@<host>:6380).')
@secure()
param redisUrl string

@description('JWT signing secret for the API.')
@secure()
param jwtSecret string

@description('Blob storage connection string.')
@secure()
param blobConnString string

@description('Azure OpenAI account key.')
@secure()
param openAiKey string

@description('Document Intelligence account key.')
@secure()
param docIntelKey string

@description('''
Create the AI secrets (openai-key, docintel-key). Set false on subscriptions
without OpenAI access (deployAi=false in main.bicep) — the keys are empty then
and no AI secrets are written to the vault.
''')
param createAiSecrets bool = true

resource keyVault 'Microsoft.KeyVault/vaults@2023-07-01' = {
  name: keyVaultName
  location: location
  properties: {
    tenantId: tenantId
    sku: {
      family: 'A'
      name: 'standard'
    }
    enableRbacAuthorization: true
    enableSoftDelete: true
    softDeleteRetentionInDays: 7
    enablePurgeProtection: true
    publicNetworkAccess: 'Enabled'
  }
}

// Secret names match the contract expected by the CD workflow and the
// container-app secret references. The AI secrets (openai-key, docintel-key)
// are only appended when createAiSecrets is true so we never write empty
// secrets on AI-less (MPN/credit) subscriptions.
var coreSecrets = [
  { name: 'db-url', value: dbUrl }
  { name: 'redis-url', value: redisUrl }
  { name: 'jwt-secret', value: jwtSecret }
  { name: 'blob-conn-string', value: blobConnString }
]

var aiSecrets = [
  { name: 'openai-key', value: openAiKey }
  { name: 'docintel-key', value: docIntelKey }
]

var secrets = concat(coreSecrets, createAiSecrets ? aiSecrets : [])

resource kvSecrets 'Microsoft.KeyVault/vaults/secrets@2023-07-01' = [
  for s in secrets: {
    parent: keyVault
    name: s.name
    properties: {
      value: s.value
    }
  }
]

// Key Vault Secrets User role definition ID (built-in).
var secretsUserRoleId = '4633458b-17de-408a-b874-0445c86b69e6'

resource secretsUser 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
  name: guid(keyVault.id, rbacPrincipalObjectId, secretsUserRoleId)
  scope: keyVault
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', secretsUserRoleId)
    principalId: rbacPrincipalObjectId
    principalType: 'ServicePrincipal'
  }
}

@description('Key Vault resource name.')
output name string = keyVault.name

@description('Key Vault resource ID.')
output id string = keyVault.id

@description('Key Vault base URI (e.g. https://hrats-kv-xxxx.vault.azure.net/).')
output vaultUri string = keyVault.properties.vaultUri
