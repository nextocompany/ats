// Azure Cache for Redis (Basic C0). TLS-only (port 6380). The primary key is
// read back to compose the rediss:// URL stored in Key Vault by main.bicep.

@description('Azure region.')
param location string

@description('Redis cache name (globally-unique).')
param redisName string

resource redis 'Microsoft.Cache/redis@2024-11-01' = {
  name: redisName
  location: location
  properties: {
    sku: {
      name: 'Basic'
      family: 'C'
      capacity: 0
    }
    enableNonSslPort: false
    minimumTlsVersion: '1.2'
    redisVersion: '6'
  }
}

@description('Redis hostname.')
output hostName string = redis.properties.hostName

@description('Redis SSL port (6380).')
output sslPort int = redis.properties.sslPort

@description('Redis cache resource name.')
output name string = redis.name

@description('Redis primary access key.')
@secure()
output primaryKey string = redis.listKeys().primaryKey
