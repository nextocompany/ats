// Log Analytics workspace + workspace-based Application Insights.
// The Container Apps environment binds to this workspace for log shipping.

@description('Azure region.')
param location string

@description('Log Analytics workspace name.')
param workspaceName string

@description('Application Insights component name.')
param appInsightsName string

@description('Log retention in days.')
@minValue(30)
param retentionInDays int = 30

resource workspace 'Microsoft.OperationalInsights/workspaces@2023-09-01' = {
  name: workspaceName
  location: location
  properties: {
    sku: {
      name: 'PerGB2018'
    }
    retentionInDays: retentionInDays
    features: {
      enableLogAccessUsingOnlyResourcePermissions: true
    }
  }
}

resource appInsights 'Microsoft.Insights/components@2020-02-02' = {
  name: appInsightsName
  location: location
  kind: 'web'
  properties: {
    Application_Type: 'web'
    WorkspaceResourceId: workspace.id
    IngestionMode: 'LogAnalytics'
    publicNetworkAccessForIngestion: 'Enabled'
    publicNetworkAccessForQuery: 'Enabled'
  }
}

@description('Log Analytics workspace resource ID.')
output workspaceId string = workspace.id

@description('Log Analytics customer (workspace) ID — used by the ACA environment.')
output workspaceCustomerId string = workspace.properties.customerId

@description('Application Insights connection string.')
output appInsightsConnectionString string = appInsights.properties.ConnectionString

@description('Application Insights instrumentation key.')
output appInsightsInstrumentationKey string = appInsights.properties.InstrumentationKey
