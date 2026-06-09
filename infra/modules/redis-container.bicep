// In-cluster Redis Container App (pilot / credit-limited path).
//
// A deliberately minimal, registry-less Container App that runs the public
// redis:7-alpine image from Docker Hub. It exists as its own module rather than
// going through the shared container-app.bicep because that module assumes an
// ACR registry + (optionally) a managed identity; this one needs neither — no
// registries block, no identity, no secrets, no env.
//
// Internal TCP ingress on 6379 makes it reachable only from inside the Container
// Apps environment at <name>.internal.<envDefaultDomain>:6379. Data is ephemeral
// (single replica, no volume) which is acceptable for the pilot.

@description('Container App name (e.g. hrats-prod-redis). Must be DNS-predictable — used to compose REDIS_URL.')
param name string

@description('Azure region.')
param location string

@description('Resource ID of the shared Container Apps managed environment.')
param environmentId string

resource redisApp 'Microsoft.App/containerApps@2024-03-01' = {
  name: name
  location: location
  properties: {
    managedEnvironmentId: environmentId
    configuration: {
      activeRevisionsMode: 'Single'
      ingress: {
        external: false
        transport: 'Tcp'
        targetPort: 6379
        exposedPort: 6379
      }
    }
    template: {
      containers: [
        {
          name: 'redis'
          image: 'redis:7-alpine'
          resources: {
            cpu: json('0.25')
            memory: '0.5Gi'
          }
        }
      ]
      scale: {
        minReplicas: 1
        maxReplicas: 1
      }
    }
  }
}

@description('The Redis Container App resource name.')
output name string = redisApp.name
