// Azure Container Registry (Basic). Admin user is disabled — image pulls use
// the workload's user-assigned managed identity with the AcrPull role, assigned
// here so the registry owns its own access grant.

@description('Azure region.')
param location string

@description('Globally-unique ACR name (alphanumeric, 5-50 chars).')
param registryName string

@description('Principal ID of the user-assigned managed identity granted AcrPull.')
param pullIdentityPrincipalId string

resource registry 'Microsoft.ContainerRegistry/registries@2023-11-01-preview' = {
  name: registryName
  location: location
  sku: {
    name: 'Basic'
  }
  properties: {
    adminUserEnabled: false
    publicNetworkAccess: 'Enabled'
  }
}

// AcrPull role definition ID (built-in).
var acrPullRoleId = '7f951dda-4ed3-4680-a7ca-43fe172d538d'

resource acrPull 'Microsoft.Authorization/roleAssignments@2022-04-01' = {
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
