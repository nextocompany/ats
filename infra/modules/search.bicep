// Azure AI Search service for candidate search (phase-2 seam, AI_SEARCH_PROVIDER=azure).
// Free tier (no cost) suits the pilot's small roster; bump to 'basic' for headroom.
// The admin key is required by the app for both index population (push) and query.

@description('Azure region for the Search service. AI Search is broadly available; defaults to the app region.')
param location string

@description('Globally-unique Search service name.')
param serviceName string

@description('SKU: free (1 partition, 50MB, 3 indexes — pilot) or basic (production headroom).')
@allowed([
  'free'
  'basic'
])
param sku string = 'free'

resource search 'Microsoft.Search/searchServices@2024-03-01-preview' = {
  name: serviceName
  location: location
  sku: {
    name: sku
  }
  properties: {
    replicaCount: 1
    partitionCount: 1
    hostingMode: 'default'
    publicNetworkAccess: 'enabled'
    // The app authenticates with the admin api-key (index push needs write).
    disableLocalAuth: false
  }
}

@description('Search service query/admin endpoint.')
output endpoint string = 'https://${search.name}.search.windows.net'

@description('Primary admin key — read/write, used for index create + push + query.')
output adminKey string = search.listAdminKeys().primaryKey
