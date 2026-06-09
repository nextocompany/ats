// Azure Container Registry (Basic). In RBAC mode the admin user is disabled and
// image pulls use the workload's user-assigned managed identity with the AcrPull
// role, assigned here so the registry owns its own access grant. In no-RBAC mode
// (Contributor-only / MPN credit subs) the admin user is enabled instead and the
// container apps pull with the ACR admin username + password — no role assignment
// is created.

@description('Azure region.')
param location string

@description('Globally-unique ACR name (alphanumeric, 5-50 chars).')
param registryName string

@description('Principal ID of the user-assigned managed identity granted AcrPull. Ignored when grantAcrPull is false.')
param pullIdentityPrincipalId string = ''

@description('Enable the ACR admin user (username/password pull). Set true for no-RBAC mode where role assignments cannot be created.')
param adminUserEnabled bool = false

@description('Create the AcrPull role assignment for the managed identity. Set false in no-RBAC mode (Contributor-only subscriptions).')
param grantAcrPull bool = true

resource registry 'Microsoft.ContainerRegistry/registries@2023-11-01-preview' = {
  name: registryName
  location: location
  sku: {
    name: 'Basic'
  }
  properties: {
    adminUserEnabled: adminUserEnabled
    publicNetworkAccess: 'Enabled'
  }
}

// AcrPull role definition ID (built-in).
var acrPullRoleId = '7f951dda-4ed3-4680-a7ca-43fe172d538d'

resource acrPull 'Microsoft.Authorization/roleAssignments@2022-04-01' = if (grantAcrPull) {
  name: guid(registry.id, pullIdentityPrincipalId, acrPullRoleId)
  scope: registry
  properties: {
    roleDefinitionId: subscriptionResourceId('Microsoft.Authorization/roleDefinitions', acrPullRoleId)
    principalId: pullIdentityPrincipalId
    principalType: 'ServicePrincipal'
  }
}

@description('ACR login server (e.g. hratsxxxx.azurecr.io).')
output loginServer string = registry.properties.loginServer

@description('ACR resource name.')
output name string = registry.name

@description('ACR resource ID.')
output id string = registry.id

// Admin credentials surfaced for no-RBAC (admin-user) pull. listCredentials()
// only returns values when adminUserEnabled is true; in RBAC mode the username
// is empty and these outputs are unused. Marked @secure() so the password value
// never appears in deployment logs/history.
@description('ACR admin username (empty unless adminUserEnabled). Used for no-RBAC container app pulls.')
#disable-next-line outputs-should-not-contain-secrets
output adminUsername string = adminUserEnabled ? registry.listCredentials().username : ''

@secure()
@description('ACR admin password (empty unless adminUserEnabled). Used for no-RBAC container app pulls.')
#disable-next-line outputs-should-not-contain-secrets
output adminPassword string = adminUserEnabled ? registry.listCredentials().passwords[0].value : ''
