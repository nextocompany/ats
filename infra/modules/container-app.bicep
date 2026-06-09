// Reusable Azure Container App module.
//
// Encapsulates a single Container App bound to a shared managed environment.
// Each app pulls its image from ACR using a user-assigned managed identity
// (AcrPull) and reads its secrets directly from Key Vault using the ACA
// keyVaultUrl secret syntax — no secret values ever flow through this template.
//
// Ingress is optional and configurable:
//   - external (public FQDN), internal (env-internal FQDN), or none.
// Scale is bounded by minReplicas/maxReplicas; the scheduler pins both to 1.

@description('Container App name (e.g. hrats-prod-api).')
param name string

@description('Azure region.')
param location string

@description('Resource ID of the shared Container Apps managed environment.')
param environmentId string

@description('Resource ID of the user-assigned managed identity used for ACR pull and Key Vault access.')
param identityId string

@description('ACR login server (e.g. hratsxxxx.azurecr.io). Used as the registry server for image pulls.')
param acrLoginServer string

@description('Fully-qualified container image reference (e.g. hratsxxxx.azurecr.io/hr-ats/api:latest).')
param image string

@description('Container target port. Ignored when ingressMode == none.')
param targetPort int = 8080

@description('Ingress exposure: external (public), internal (env-only), or none.')
@allowed([
  'external'
  'internal'
  'none'
])
param ingressMode string = 'none'

@description('Minimum replica count. The scheduler must pin this to 1.')
@minValue(0)
param minReplicas int = 1

@description('Maximum replica count. The scheduler must pin this to 1 (single-replica, load-bearing).')
@minValue(1)
param maxReplicas int = 3

@description('Plain (non-secret) environment variables as name/value pairs.')
param envVars array = []

@description('''
Secret definitions sourced from Key Vault. Each item:
  { name: '<aca-secret-name>', keyVaultUrl: '<kv secret uri>' }
The managed identity (identityId) must have Key Vault Secrets User on the vault.
''')
param keyVaultSecrets array = []

@description('''
Secret-backed environment variables. Each item:
  { name: '<ENV_NAME>', secretRef: '<aca-secret-name>' }
''')
param secretEnvVars array = []

@description('CPU cores per replica.')
param cpu string = '0.5'

@description('Memory per replica.')
param memory string = '1Gi'

// Build the merged env array: plain vars + secret-backed vars.
var mergedEnv = concat(envVars, secretEnvVars)

var ingressConfig = ingressMode == 'none'
  ? null
  : {
      external: ingressMode == 'external'
      targetPort: targetPort
      transport: 'auto'
      allowInsecure: false
      traffic: [
        {
          latestRevision: true
          weight: 100
        }
      ]
    }

resource containerApp 'Microsoft.App/containerApps@2024-03-01' = {
  name: name
  location: location
  identity: {
    type: 'UserAssigned'
    userAssignedIdentities: {
      '${identityId}': {}
    }
  }
  properties: {
    managedEnvironmentId: environmentId
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: ingressConfig
      registries: [
        {
          server: acrLoginServer
          identity: identityId
        }
      ]
      secrets: [
        for s in keyVaultSecrets: {
          name: s.name
          keyVaultUrl: s.keyVaultUrl
          identity: identityId
        }
      ]
    }
    template: {
      containers: [
        {
          name: name
          image: image
          resources: {
            cpu: json(cpu)
            memory: memory
          }
          env: mergedEnv
        }
      ]
      scale: {
        minReplicas: minReplicas
        maxReplicas: maxReplicas
      }
    }
  }
}

@description('The Container App resource name.')
output name string = containerApp.name

@description('The Container App fully-qualified domain name (empty when ingress is none).')
output fqdn string = ingressMode == 'none' ? '' : containerApp.properties.configuration.ingress.fqdn

@description('The Container App resource ID.')
output id string = containerApp.id
