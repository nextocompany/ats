// PostgreSQL Flexible Server (v16, Burstable B1ms) + the application database.
// Public network access is enabled with an Azure-services firewall rule so the
// Container Apps environment can reach it without VNet integration (v1 scope).
// The admin password is supplied as a secure param from main.bicep.

@description('Azure region.')
param location string

@description('Flexible Server name (globally-unique within the region scope).')
param serverName string

@description('Application database name.')
param databaseName string = 'hr_db'

@description('Administrator login name.')
param administratorLogin string = 'hratsadmin'

@description('Administrator password.')
@secure()
param administratorPassword string

resource postgres 'Microsoft.DBforPostgreSQL/flexibleServers@2024-08-01' = {
  name: serverName
  location: location
  sku: {
    name: 'Standard_B1ms'
    tier: 'Burstable'
  }
  properties: {
    version: '16'
    administratorLogin: administratorLogin
    administratorLoginPassword: administratorPassword
    storage: {
      storageSizeGB: 32
    }
    backup: {
      backupRetentionDays: 7
      geoRedundantBackup: 'Disabled'
    }
    highAvailability: {
      mode: 'Disabled'
    }
    network: {
      publicNetworkAccess: 'Enabled'
    }
  }
}

resource database 'Microsoft.DBforPostgreSQL/flexibleServers/databases@2024-08-01' = {
  parent: postgres
  name: databaseName
  properties: {
    charset: 'UTF8'
    collation: 'en_US.utf8'
  }
}

// Allow-list the extensions the migrations CREATE EXTENSION (Azure Postgres
// blocks CREATE EXTENSION unless the extension is in azure.extensions). Without
// this, migration 000001 fails: 'extension "pgcrypto" is not allow-listed'.
// Dynamic parameter — no server restart required.
resource extensionsAllowlist 'Microsoft.DBforPostgreSQL/flexibleServers/configurations@2024-08-01' = {
  parent: postgres
  name: 'azure.extensions'
  properties: {
    value: 'pgcrypto,pg_trgm'
    source: 'user-override'
  }
}

// Allow access from Azure services (Container Apps egress). 0.0.0.0 is the
// well-known sentinel that scopes the rule to Azure-internal traffic.
resource allowAzure 'Microsoft.DBforPostgreSQL/flexibleServers/firewallRules@2024-08-01' = {
  parent: postgres
  name: 'AllowAllAzureServices'
  properties: {
    startIpAddress: '0.0.0.0'
    endIpAddress: '0.0.0.0'
  }
}

@description('Fully-qualified domain name of the Postgres server.')
output fqdn string = postgres.properties.fullyQualifiedDomainName

@description('Postgres server resource name.')
output name string = postgres.name

@description('Administrator login (for URL composition).')
output administratorLogin string = administratorLogin

@description('Application database name.')
output databaseName string = database.name
