// Azure OpenAI account (Cognitive Services kind OpenAI) with a single model
// deployment. Region availability for OpenAI models differs from the main app
// region, so this is provisioned in a separate aiLocation.

@description('Azure region for the OpenAI account (model availability differs — may need a region other than the app location).')
param location string

@description('Globally-unique OpenAI account name.')
param accountName string

@description('Model deployment name (referenced by the app as AZURE_OPENAI_DEPLOYMENT).')
param deploymentName string = 'gpt-4o'

@description('Underlying model name.')
param modelName string = 'gpt-4o'

@description('Model version. Leave empty to let Azure pick the default for the model.')
param modelVersion string = '2024-07-18'

@description('Deployment capacity (TPM in thousands).')
param capacity int = 10

resource openai 'Microsoft.CognitiveServices/accounts@2024-10-01' = {
  name: accountName
  location: location
  kind: 'OpenAI'
  sku: {
    name: 'S0'
  }
  properties: {
    customSubDomainName: accountName
    publicNetworkAccess: 'Enabled'
    disableLocalAuth: false
  }
}

resource deployment 'Microsoft.CognitiveServices/accounts/deployments@2024-10-01' = {
  parent: openai
  name: deploymentName
  sku: {
    name: 'GlobalStandard'
    capacity: capacity
  }
  properties: {
    model: {
      format: 'OpenAI'
      name: modelName
      version: modelVersion
    }
  }
}

@description('Azure OpenAI endpoint (e.g. https://hrats-openai-xxxx.openai.azure.com/).')
output endpoint string = openai.properties.endpoint

@description('OpenAI account resource name.')
output name string = openai.name

@description('Model deployment name.')
output deploymentName string = deployment.name

@description('Azure OpenAI account key.')
@secure()
output key string = openai.listKeys().key1
