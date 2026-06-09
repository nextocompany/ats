// Document Intelligence account (Cognitive Services kind FormRecognizer) for
// resume OCR/parse. Provisioned in aiLocation alongside Azure OpenAI.

@description('Azure region for the Document Intelligence account.')
param location string

@description('Globally-unique Document Intelligence account name.')
param accountName string

resource docIntel 'Microsoft.CognitiveServices/accounts@2024-10-01' = {
  name: accountName
  location: location
  kind: 'FormRecognizer'
  sku: {
    name: 'S0'
  }
  properties: {
    customSubDomainName: accountName
    publicNetworkAccess: 'Enabled'
    disableLocalAuth: false
  }
}

@description('Document Intelligence endpoint.')
output endpoint string = docIntel.properties.endpoint

@description('Document Intelligence account resource name.')
output name string = docIntel.name

@description('Document Intelligence account key.')
@secure()
output key string = docIntel.listKeys().key1
