# -------------------------------------------------------------------
# Brockley -- GCP deployment
# Provisions: GKE Autopilot + Cloud SQL (PostgreSQL) + Memorystore (Redis)
# Then deploys the Brockley Helm chart into the cluster.
# -------------------------------------------------------------------

provider "google" {
  project = var.project_id
  region  = var.region
}

locals {
  labels = merge({
    app       = "brockley"
    managed   = "terraform"
    component = "brockley-platform"
  }, var.labels)

  # Map size preset to Cloud SQL tier
  db_tier_map = {
    starter     = "db-f1-micro"
    standard    = "db-custom-2-7680"
    performance = "db-custom-4-15360"
  }
  db_tier = var.database_tier != "" ? var.database_tier : local.db_tier_map[var.size]
}

# -------------------------------------------------------------------
# Networking
# -------------------------------------------------------------------

resource "google_compute_network" "vpc" {
  project                 = var.project_id
  name                    = "${var.name}-vpc"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  project       = var.project_id
  name          = "${var.name}-subnet"
  region        = var.region
  network       = google_compute_network.vpc.id
  ip_cidr_range = "10.0.0.0/20"

  # Secondary ranges required by GKE for pods and services
  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.4.0.0/14"
  }
  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.8.0.0/20"
  }

  private_ip_google_access = true
}

# Private services access -- allows Cloud SQL and Memorystore to use
# internal IPs reachable from the VPC without public endpoints.
resource "google_compute_global_address" "private_ip_range" {
  project       = var.project_id
  name          = "${var.name}-private-ip"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 20
  network       = google_compute_network.vpc.id
}

resource "google_service_networking_connection" "private_vpc" {
  network                 = google_compute_network.vpc.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_ip_range.name]
}

# -------------------------------------------------------------------
# GKE Autopilot
# -------------------------------------------------------------------

resource "google_container_cluster" "autopilot" {
  project  = var.project_id
  name     = "${var.name}-gke"
  location = var.region

  # Autopilot manages node pools, scaling, and OS patching automatically.
  enable_autopilot = true

  network    = google_compute_network.vpc.id
  subnetwork = google_compute_subnetwork.subnet.id

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  # Private cluster -- nodes have no public IPs.
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  resource_labels = local.labels

  # Deletion protection is recommended for production. Disable with
  # terraform apply -var='...' if you need to tear down.
  deletion_protection = false
}

# -------------------------------------------------------------------
# Cloud SQL for PostgreSQL
# -------------------------------------------------------------------

resource "google_sql_database_instance" "postgres" {
  project          = var.project_id
  name             = "${var.name}-pg"
  region           = var.region
  database_version = "POSTGRES_16"

  depends_on = [google_service_networking_connection.private_vpc]

  settings {
    tier              = local.db_tier
    availability_type = var.size == "starter" ? "ZONAL" : "REGIONAL"
    disk_size         = var.database_disk_size_gb
    disk_autoresize   = true

    ip_configuration {
      ipv4_enabled                                  = false
      private_network                               = google_compute_network.vpc.id
      enable_private_path_for_google_cloud_services = true
    }

    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = var.size != "starter"
      start_time                     = "03:00"
    }

    database_flags {
      name  = "max_connections"
      value = var.size == "performance" ? "200" : "100"
    }

    user_labels = local.labels
  }

  deletion_protection = false
}

resource "google_sql_database" "brockley" {
  project  = var.project_id
  name     = "brockley"
  instance = google_sql_database_instance.postgres.name
}

resource "google_sql_user" "brockley" {
  project  = var.project_id
  name     = "brockley"
  instance = google_sql_database_instance.postgres.name
  password = random_password.db_password.result
}

resource "random_password" "db_password" {
  length  = 32
  special = false
}

# -------------------------------------------------------------------
# Memorystore for Redis
# -------------------------------------------------------------------

resource "google_redis_instance" "redis" {
  project        = var.project_id
  name           = "${var.name}-redis"
  region         = var.region
  tier           = var.size == "starter" ? "BASIC" : "STANDARD_HA"
  memory_size_gb = var.redis_memory_size_gb

  authorized_network = google_compute_network.vpc.id

  redis_version = "REDIS_7_0"

  labels = local.labels

  depends_on = [google_service_networking_connection.private_vpc]
}

# -------------------------------------------------------------------
# Kubernetes + Helm providers
# -------------------------------------------------------------------

data "google_client_config" "default" {}

provider "kubernetes" {
  host                   = "https://${google_container_cluster.autopilot.endpoint}"
  token                  = data.google_client_config.default.access_token
  cluster_ca_certificate = base64decode(google_container_cluster.autopilot.master_auth[0].cluster_ca_certificate)
}

provider "helm" {
  kubernetes {
    host                   = "https://${google_container_cluster.autopilot.endpoint}"
    token                  = data.google_client_config.default.access_token
    cluster_ca_certificate = base64decode(google_container_cluster.autopilot.master_auth[0].cluster_ca_certificate)
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

  # Database connection -- uses the Cloud SQL private IP
  set {
    name  = "postgresql.uri"
    value = "postgres://brockley:${random_password.db_password.result}@${google_sql_database_instance.postgres.private_ip_address}:5432/brockley?sslmode=require"
  }

  # Redis connection -- Memorystore private IP
  set {
    name  = "redis.url"
    value = "redis://${google_redis_instance.redis.host}:${google_redis_instance.redis.port}/0"
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
    google_container_cluster.autopilot,
    google_sql_database.brockley,
    google_redis_instance.redis,
  ]
}
