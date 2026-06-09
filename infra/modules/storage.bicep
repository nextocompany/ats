// Storage account + private blob container for resumes. The connection string
// is composed from the account key and returned (secure) for Key Vault storage.

@description('Azure region.')
param location string

@description('Globally-unique storage account name (3-24 chars, lowercase alphanumeric).')
param storageAccountName string

@description('Private blob container name for resume uploads.')
param containerName string = 'resumes'

resource storage 'Microsoft.Storage/storageAccounts@2023-05-01' = {
  name: storageAccountName
  location: location
  sku: {
    name: 'Standard_LRS'
  }
  kind: 'StorageV2'
  properties: {
    accessTier: 'Hot'
    allowBlobPublicAccess: false
    minimumTlsVersion: 'TLS1_2'
    supportsHttpsTrafficOnly: true
    publicNetworkAccess: 'Enabled'
  }
}

resource blobService 'Microsoft.Storage/storageAccounts/blobServices@2023-05-01' = {
  parent: storage
  name: 'default'
}

resource container 'Microsoft.Storage/storageAccounts/blobServices/containers@2023-05-01' = {
  parent: blobService
  name: containerName
  properties: {
    publicAccess: 'None'
  }
}

@description('Storage account resource name.')
output name string = storage.name

@description('Blob container name.')
output containerName string = container.name

@description('Storage account connection string (account-key based).')
@secure()
output connectionString string = 'DefaultEndpointsProtocol=https;AccountName=${storage.name};AccountKey=${storage.listKeys().keys[0].value};EndpointSuffix=${environment().suffixes.storage}'
