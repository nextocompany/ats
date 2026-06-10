// Reusable Azure Container App module.
//
// Encapsulates a single Container App bound to a shared managed environment.
// Supports two wiring styles:
//   - RBAC mode (default): pulls its image from ACR using a user-assigned managed
//     identity (AcrPull) and reads secrets directly from Key Vault via the ACA
//     keyVaultUrl secret syntax — no secret values flow through this template.
//   - No-RBAC mode (Contributor-only subs): pulls from ACR with the admin
//     username + password (stored as the 'acr-password' app secret) and reads
//     secrets from inline literal Container App secrets. No managed identity is
//     attached and no role assignments are needed.
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

@description('''
Resource ID of the user-assigned managed identity used for ACR pull and Key Vault
access. Empty in no-RBAC mode — the app then attaches no identity and pulls with
the ACR admin username/password instead.
''')
param identityId string = ''

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

@description('KEDA scale rules (e.g. a Redis queue trigger). Empty = replica-count bounds only.')
param scaleRules array = []

@description('Plain (non-secret) environment variables as name/value pairs.')
param envVars array = []

@description('''
Secret definitions sourced from Key Vault. Each item:
  { name: '<aca-secret-name>', keyVaultUrl: '<kv secret uri>' }
The managed identity (identityId) must have Key Vault Secrets User on the vault.
''')
param keyVaultSecrets array = []

@description('''
Inline literal secret definitions (no-RBAC mode). Shape:
  { items: [ { name: '<aca-secret-name>', value: '<literal secret value>' } ] }
Wrapped in an object so the @secure() decorator (object-only) can protect the
literal values. They are stored as Container App secrets and never surfaced as
plain env or outputs. Mutually exclusive in practice with keyVaultSecrets.
''')
@secure()
param inlineSecrets object = {}

@description('''
Secret-backed environment variables. Each item:
  { name: '<ENV_NAME>', secretRef: '<aca-secret-name>' }
''')
param secretEnvVars array = []

@description('ACR admin username for no-RBAC (admin-user) image pull. Empty in RBAC mode.')
param registryUsername string = ''

@description('ACR admin password for no-RBAC (admin-user) image pull. Stored as the acr-password app secret. Empty in RBAC mode.')
@secure()
param registryPasswordSecretValue string = ''

@description('CPU cores per replica.')
param cpu string = '0.5'

@description('Memory per replica.')
param memory string = '1Gi'

// Build the merged env array: plain vars + secret-backed vars.
var mergedEnv = concat(envVars, secretEnvVars)

// --- Identity / registry wiring -------------------------------------------
// RBAC mode is signalled by a non-empty identityId. No-RBAC mode (empty
// identityId) attaches no managed identity and pulls with the ACR admin
// username + the acr-password app secret.
var useManagedIdentity = !empty(identityId)

var identityConfig = useManagedIdentity
  ? {
      type: 'UserAssigned'
      userAssignedIdentities: {
        '${identityId}': {}
      }
    }
  : null

var registriesConfig = useManagedIdentity
  ? [
      {
        server: acrLoginServer
        identity: identityId
      }
    ]
  : [
      {
        server: acrLoginServer
        username: registryUsername
        passwordSecretRef: 'acr-password'
      }
    ]

// --- Secret composition ----------------------------------------------------
// (a) Key Vault-backed secrets (RBAC mode), (b) inline literal secrets
// (no-RBAC mode), (c) the acr-password secret (no-RBAC admin pull only).
var keyVaultSecretDefs = [
  for s in keyVaultSecrets: {
    name: s.name
    keyVaultUrl: s.keyVaultUrl
    identity: identityId
  }
]

var inlineSecretItems = inlineSecrets.?items ?? []
var inlineSecretDefs = [
  for s in inlineSecretItems: {
    name: s.name
    value: s.value
  }
]

var acrPasswordSecretDefs = useManagedIdentity
  ? []
  : [
      {
        name: 'acr-password'
        value: registryPasswordSecretValue
      }
    ]

var allSecrets = concat(keyVaultSecretDefs, inlineSecretDefs, acrPasswordSecretDefs)

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
  identity: identityConfig
  properties: {
    managedEnvironmentId: environmentId
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: ingressConfig
      registries: registriesConfig
      secrets: allSecrets
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
        rules: scaleRules
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
