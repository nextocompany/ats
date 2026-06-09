// Container Apps managed environment, bound to the Log Analytics workspace for
// log shipping. All five container apps share this environment.

@description('Azure region.')
param location string

@description('Container Apps environment name.')
param environmentName string

@description('Log Analytics workspace customer (workspace) ID.')
param logAnalyticsCustomerId string

@description('Log Analytics workspace resource ID (to read the shared key).')
param logAnalyticsWorkspaceId string

resource environment 'Microsoft.App/managedEnvironments@2024-03-01' = {
  name: environmentName
  location: location
  properties: {
    appLogsConfiguration: {
      destination: 'log-analytics'
      logAnalyticsConfiguration: {
        customerId: logAnalyticsCustomerId
        sharedKey: listKeys(logAnalyticsWorkspaceId, '2023-09-01').primarySharedKey
      }
    }
    zoneRedundant: false
  }
}

@description('Container Apps environment resource ID.')
output id string = environment.id

@description('Container Apps environment name.')
output name string = environment.name

@description('Environment default domain (apps get <name>.<defaultDomain>).')
output defaultDomain string = environment.properties.defaultDomain
