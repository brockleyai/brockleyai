# -------------------------------------------------------------------
# Brockley -- Azure deployment
# Provisions: AKS + Azure Database for PostgreSQL Flexible Server + Azure Cache for Redis
# Then deploys the Brockley Helm chart into the cluster.
# -------------------------------------------------------------------

locals {
  resource_group_name = var.resource_group_name != "" ? var.resource_group_name : "${var.name}-rg"

  tags = merge({
    App       = "brockley"
    Managed   = "terraform"
    Component = "brockley-platform"
  }, var.tags)

  # Map size preset to PostgreSQL SKU
  db_sku_map = {
    starter     = "B_Standard_B1ms"
    standard    = "GP_Standard_D2s_v3"
    performance = "GP_Standard_D4s_v3"
  }
  db_sku = var.database_sku != "" ? var.database_sku : local.db_sku_map[var.size]

  # Map size preset to Redis SKU
  redis_sku_map = {
    starter     = "Basic"
    standard    = "Standard"
    performance = "Premium"
  }
  redis_sku = var.redis_sku != "" ? var.redis_sku : local.redis_sku_map[var.size]
}

# -------------------------------------------------------------------
# Resource Group
# -------------------------------------------------------------------

resource "azurerm_resource_group" "main" {
  name     = local.resource_group_name
  location = var.location
  tags     = local.tags
}

# -------------------------------------------------------------------
# Networking
# -------------------------------------------------------------------

resource "azurerm_virtual_network" "main" {
  name                = "${var.name}-vnet"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location
  address_space       = [var.vnet_address_space]
  tags                = local.tags
}

# AKS nodes subnet
resource "azurerm_subnet" "aks" {
  name                 = "${var.name}-aks-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [cidrsubnet(var.vnet_address_space, 8, 0)]
}

# PostgreSQL subnet -- delegated to Microsoft.DBforPostgreSQL/flexibleServers
resource "azurerm_subnet" "postgres" {
  name                 = "${var.name}-pg-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [cidrsubnet(var.vnet_address_space, 8, 1)]

  delegation {
    name = "postgres-delegation"
    service_delegation {
      name    = "Microsoft.DBforPostgreSQL/flexibleServers"
      actions = ["Microsoft.Network/virtualNetworks/subnets/join/action"]
    }
  }
}

# Redis subnet -- Premium SKU supports VNet injection; Basic/Standard use firewall rules
resource "azurerm_subnet" "redis" {
  count                = local.redis_sku == "Premium" ? 1 : 0
  name                 = "${var.name}-redis-subnet"
  resource_group_name  = azurerm_resource_group.main.name
  virtual_network_name = azurerm_virtual_network.main.name
  address_prefixes     = [cidrsubnet(var.vnet_address_space, 8, 2)]
}

# Private DNS zone for PostgreSQL -- required for VNet-integrated Flexible Server
resource "azurerm_private_dns_zone" "postgres" {
  name                = "${var.name}.postgres.database.azure.com"
  resource_group_name = azurerm_resource_group.main.name
  tags                = local.tags
}

resource "azurerm_private_dns_zone_virtual_network_link" "postgres" {
  name                  = "${var.name}-pg-dns-link"
  resource_group_name   = azurerm_resource_group.main.name
  private_dns_zone_name = azurerm_private_dns_zone.postgres.name
  virtual_network_id    = azurerm_virtual_network.main.id
}

# -------------------------------------------------------------------
# AKS Automatic
# -------------------------------------------------------------------

resource "azurerm_kubernetes_cluster" "main" {
  name                = "${var.name}-aks"
  location            = azurerm_resource_group.main.location
  resource_group_name = azurerm_resource_group.main.name
  dns_prefix          = var.name

  # System-managed identity -- no service principal credentials to rotate.
  identity {
    type = "SystemAssigned"
  }

  default_node_pool {
    name                = "system"
    vm_size             = "Standard_DS2_v2"
    auto_scaling_enabled = true
    min_count           = 1
    max_count           = var.size == "performance" ? 5 : 3
    vnet_subnet_id      = azurerm_subnet.aks.id

    upgrade_settings {
      max_surge = "10%"
    }
  }

  network_profile {
    network_plugin    = "azure"
    network_policy    = "calico"
    load_balancer_sku = "standard"
  }

  tags = local.tags
}

# -------------------------------------------------------------------
# Azure Database for PostgreSQL Flexible Server
# -------------------------------------------------------------------

resource "random_password" "db_password" {
  length  = 32
  special = false
}

resource "azurerm_postgresql_flexible_server" "main" {
  name                = "${var.name}-pg"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  sku_name   = local.db_sku
  version    = "16"
  storage_mb = var.database_storage_mb

  administrator_login    = "brockley"
  administrator_password = random_password.db_password.result

  # VNet integration via delegated subnet
  delegated_subnet_id = azurerm_subnet.postgres.id
  private_dns_zone_id = azurerm_private_dns_zone.postgres.id

  # High availability for non-starter sizes
  dynamic "high_availability" {
    for_each = var.size != "starter" ? [1] : []
    content {
      mode = "ZoneRedundant"
    }
  }

  backup_retention_days        = var.size == "starter" ? 7 : 35
  geo_redundant_backup_enabled = var.size == "performance"

  tags = local.tags

  depends_on = [azurerm_private_dns_zone_virtual_network_link.postgres]
}

resource "azurerm_postgresql_flexible_server_database" "brockley" {
  name      = "brockley"
  server_id = azurerm_postgresql_flexible_server.main.id
  charset   = "UTF8"
  collation = "en_US.utf8"
}

# -------------------------------------------------------------------
# Azure Cache for Redis
# -------------------------------------------------------------------

resource "azurerm_redis_cache" "main" {
  name                = "${var.name}-redis"
  resource_group_name = azurerm_resource_group.main.name
  location            = azurerm_resource_group.main.location

  sku_name = local.redis_sku
  family   = local.redis_sku == "Premium" ? "P" : "C"
  capacity = var.redis_capacity

  redis_version         = "6"
  minimum_tls_version   = "1.2"
  enable_non_ssl_port   = false

  # Premium SKU: inject into VNet for private access
  dynamic "patch_schedule" {
    for_each = local.redis_sku == "Premium" ? [1] : []
    content {
      day_of_week    = "Sunday"
      start_hour_utc = 3
    }
  }

  redis_configuration {}

  tags = local.tags
}

# -------------------------------------------------------------------
# Kubernetes + Helm providers
# -------------------------------------------------------------------

provider "kubernetes" {
  host                   = azurerm_kubernetes_cluster.main.kube_config[0].host
  client_certificate     = base64decode(azurerm_kubernetes_cluster.main.kube_config[0].client_certificate)
  client_key             = base64decode(azurerm_kubernetes_cluster.main.kube_config[0].client_key)
  cluster_ca_certificate = base64decode(azurerm_kubernetes_cluster.main.kube_config[0].cluster_ca_certificate)
}

provider "helm" {
  kubernetes {
    host                   = azurerm_kubernetes_cluster.main.kube_config[0].host
    client_certificate     = base64decode(azurerm_kubernetes_cluster.main.kube_config[0].client_certificate)
    client_key             = base64decode(azurerm_kubernetes_cluster.main.kube_config[0].client_key)
    cluster_ca_certificate = base64decode(azurerm_kubernetes_cluster.main.kube_config[0].cluster_ca_certificate)
  }
}

# -------------------------------------------------------------------
# Helm release
# -------------------------------------------------------------------

resource "helm_release" "brockley" {
  name      = var.name
  chart     = var.helm_chart_path
  namespace = "brockley"

  create_namespace = true
  wait             = true
  timeout          = 600

  # Apply the size-specific values file
  values = compact([
    file("${var.helm_chart_path}/values-${var.size}.yaml"),
    var.extra_helm_values,
  ])

  # Database connection -- uses the Flexible Server FQDN over the private VNet link
  set {
    name  = "postgresql.uri"
    value = "postgres://brockley:${random_password.db_password.result}@${azurerm_postgresql_flexible_server.main.fqdn}:5432/brockley?sslmode=require"
  }

  # Redis connection -- Azure Cache for Redis uses SSL on port 6380
  set {
    name  = "redis.url"
    value = "rediss://:${azurerm_redis_cache.main.primary_access_key}@${azurerm_redis_cache.main.hostname}:${azurerm_redis_cache.main.ssl_port}/0"
  }

  set {
    name  = "env"
    value = "production"
  }

  dynamic "set" {
    for_each = var.brockley_image_tag != "" ? [var.brockley_image_tag] : []
    content {
      name  = "server.image.tag"
      value = set.value
    }
  }

  depends_on = [
    azurerm_kubernetes_cluster.main,
    azurerm_postgresql_flexible_server_database.brockley,
    azurerm_redis_cache.main,
  ]
}
