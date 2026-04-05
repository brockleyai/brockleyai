# Brockley -- Terraform Deployment Modules

Deploy Brockley to any major cloud with a single `terraform apply`. Each module provisions a managed Kubernetes cluster, managed PostgreSQL, managed Redis, and deploys the Brockley Helm chart.

## Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.5
- Cloud CLI authenticated with appropriate permissions:
  - **GCP**: `gcloud auth application-default login`
  - **AWS**: `aws configure` or environment variables
  - **Azure**: `az login`
- Docker (for building custom images, if needed)

## Size Presets

All modules accept a `size` variable that maps to a Helm values preset:

| Size | Use Case | Server Replicas | Worker Replicas | KEDA |
|------|----------|-----------------|-----------------|------|
| `starter` | Development, evaluation, small teams | 1 | 1 | Off |
| `standard` | Production workloads | 2 | 2-4 (autoscaled) | On |
| `performance` | High-throughput production | 3 | 4-8 (autoscaled) | On |

The `size` also controls database and cache instance types for each cloud.

## Common Variables

Every module supports these variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `name` | Base name for all resources | `brockley` |
| `size` | Deployment size preset | `standard` |
| `helm_chart_path` | Path to the Brockley Helm chart | `../helm/brockley` |
| `brockley_image_tag` | Docker image tag override | `""` (uses chart appVersion) |
| `extra_helm_values` | Additional Helm values (YAML string) | `""` |

---

## GCP

Provisions: GKE Autopilot + Cloud SQL (PostgreSQL 16) + Memorystore (Redis 7.0)

```bash
cd deploy/terraform/gcp

terraform init
terraform plan -var='project_id=my-gcp-project' -var='size=standard'
terraform apply -var='project_id=my-gcp-project' -var='size=standard'
```

### GCP-specific variables

| Variable | Description | Default |
|----------|-------------|---------|
| `project_id` | GCP project ID | (required) |
| `region` | GCP region | `us-central1` |
| `database_tier` | Cloud SQL machine type override | auto |
| `database_disk_size_gb` | Cloud SQL disk size | `20` |
| `redis_memory_size_gb` | Memorystore instance size | `1` |
| `labels` | GCP resource labels | `{}` |

### Connect to the cluster

```bash
# Printed by terraform output
gcloud container clusters get-credentials brockley-gke --region us-central1 --project my-gcp-project
kubectl get pods -n brockley
```

---

## AWS

Provisions: EKS Auto Mode + RDS PostgreSQL 16 + ElastiCache Redis 7.1

```bash
cd deploy/terraform/aws

terraform init
terraform plan -var='size=standard'
terraform apply -var='size=standard'
```

### AWS-specific variables

| Variable | Description | Default |
|----------|-------------|---------|
| `region` | AWS region | `us-east-1` |
| `vpc_cidr` | VPC CIDR block | `10.0.0.0/16` |
| `database_instance_class` | RDS instance class override | auto |
| `database_allocated_storage` | RDS storage in GB | `20` |
| `redis_node_type` | ElastiCache node type override | auto |
| `tags` | AWS resource tags | `{}` |

### Connect to the cluster

```bash
aws eks update-kubeconfig --name brockley-eks --region us-east-1
kubectl get pods -n brockley
```

---

## Azure

Provisions: AKS + Azure Database for PostgreSQL Flexible Server + Azure Cache for Redis

```bash
cd deploy/terraform/azure

terraform init
terraform plan -var='size=standard'
terraform apply -var='size=standard'
```

### Azure-specific variables

| Variable | Description | Default |
|----------|-------------|---------|
| `location` | Azure region | `eastus` |
| `resource_group_name` | Resource group name (created if empty) | auto |
| `vnet_address_space` | VNet address space | `10.0.0.0/16` |
| `database_sku` | PostgreSQL Flexible Server SKU override | auto |
| `database_storage_mb` | PostgreSQL storage in MB | `32768` |
| `redis_sku` | Redis SKU (Basic/Standard/Premium) override | auto |
| `redis_capacity` | Redis capacity | `0` |
| `tags` | Azure resource tags | `{}` |

### Connect to the cluster

```bash
az aks get-credentials --resource-group brockley-rg --name brockley-aks
kubectl get pods -n brockley
```

---

## Passing Extra Helm Values

Use `extra_helm_values` to inject additional configuration:

```bash
terraform apply -var='size=standard' \
  -var='extra_helm_values=ingress:
  enabled: true
  host: brockley.example.com
  tls:
    enabled: true
    secretName: brockley-tls
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod'
```

Or use a `.tfvars` file:

```hcl
# prod.tfvars
size = "performance"
extra_helm_values = <<-YAML
  ingress:
    enabled: true
    host: brockley.example.com
  extraSecrets:
    BROCKLEY_SECRET_OPENAI_API_KEY: "sk-..."
YAML
```

```bash
terraform apply -var-file=prod.tfvars
```

---

## Tearing Down

```bash
terraform destroy
```

All modules set `deletion_protection = false` by default so `terraform destroy` works without manual intervention. For production deployments, consider enabling deletion protection on the database.

---

## Architecture

Each module creates:

```
Cloud VPC / VNet
├── Private Subnets
│   ├── Managed Kubernetes (GKE Autopilot / EKS Auto Mode / AKS)
│   │   └── brockley namespace
│   │       ├── server deployment (API + GraphQL)
│   │       ├── worker deployment (asynq task processor)
│   │       ├── coderunner deployment (Python code execution)
│   │       └── web-ui deployment (React graph editor)
│   ├── Managed PostgreSQL (Cloud SQL / RDS / Flexible Server)
│   └── Managed Redis (Memorystore / ElastiCache / Azure Cache)
└── Public Subnets (load balancer only, where applicable)
```

All data stores are accessible only from within the private network. No public database endpoints are created.
