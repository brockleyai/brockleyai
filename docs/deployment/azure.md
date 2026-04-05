# Deploy to Azure (AKS)

This guide walks through deploying Brockley to Azure using AKS, Azure Database for PostgreSQL Flexible Server, and Azure Cache for Redis.

**Time estimate:** 15-20 minutes.

## Prerequisites

- Azure subscription with an active billing account
- [Azure CLI](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli) installed and authenticated
- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.5
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) >= 3.0

Authenticate with Azure:

```bash
az login
```

Verify your subscription:

```bash
az account show --query '{name:name, id:id}' -o table
```

## Step 1: Create a Resource Group

```bash
az group create \
  --name brockley-rg \
  --location eastus
```

Expected output:

```json
{
  "id": "/subscriptions/xxxx/resourceGroups/brockley-rg",
  "location": "eastus",
  "name": "brockley-rg"
}
```

## Step 2: Clone and Configure

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai/deploy/terraform/azure
```

Create a `terraform.tfvars` file:

```hcl
resource_group_name = "brockley-rg"
location            = "eastus"

# Size: "starter", "standard", or "performance"
size = "standard"

# Optional: customize infrastructure
db_sku_name     = "GP_Standard_D2s_v3"    # 2 vCPU, 8 GB RAM
db_storage_mb   = 32768                     # 32 GB
redis_sku       = "Basic"
redis_capacity  = 1                         # C1 (1.5 GB)
```

### Available Regions

| Region | Location |
|--------|----------|
| `eastus` | East US (Virginia) |
| `westus2` | West US 2 (Washington) |
| `westeurope` | West Europe (Netherlands) |
| `northeurope` | North Europe (Ireland) |
| `southeastasia` | Southeast Asia (Singapore) |
| `japaneast` | Japan East (Tokyo) |

## Step 3: Deploy Infrastructure

```bash
terraform init
terraform plan
```

Review the plan. It will create:

- AKS cluster with system node pool
- Azure Database for PostgreSQL Flexible Server (private access)
- Azure Cache for Redis
- Virtual network with subnets
- Private DNS zone for database connectivity

When the plan looks right:

```bash
terraform apply
```

Type `yes` when prompted. This takes 10-15 minutes.

Expected output on completion:

```
Apply complete! Resources: 18 added, 0 changed, 0 destroyed.

Outputs:

cluster_name     = "brockley-aks"
database_url     = "postgres://brockley:****@brockley-db.postgres.database.azure.com:5432/brockley?sslmode=require"
redis_url        = "rediss://:****@brockley-redis.redis.cache.windows.net:6380/0"
resource_group   = "brockley-rg"
```

## Step 4: Configure kubectl

```bash
az aks get-credentials \
  --resource-group brockley-rg \
  --name brockley-aks
```

Verify connectivity:

```bash
kubectl get nodes
```

Expected output:

```
NAME                                STATUS   ROLES   AGE   VERSION
aks-default-xxxxxxxx-vmss000000    Ready    agent   5m    v1.28.x
aks-default-xxxxxxxx-vmss000001    Ready    agent   5m    v1.28.x
```

## Step 5: Install Brockley with Helm

Export the Terraform outputs:

```bash
export DATABASE_URL=$(terraform output -raw database_url)
export REDIS_URL=$(terraform output -raw redis_url)
```

Install with the standard preset:

```bash
cd ../../../   # back to repo root

helm install brockley deploy/helm/brockley \
  --namespace brockley \
  --create-namespace \
  -f deploy/helm/brockley/values-standard.yaml \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL" \
  --set env=production \
  --set auth.apiKeys="$(openssl rand -hex 24)" \
  --set encryptionKey="$(openssl rand -base64 32)"
```

Save the generated API key:

```bash
kubectl get secret brockley-secret -n brockley -o jsonpath='{.data.BROCKLEY_API_KEYS}' | base64 -d
```

## Step 6: Verify Deployment

```bash
kubectl get pods -n brockley
```

Expected output:

```
NAME                                READY   STATUS    RESTARTS   AGE
brockley-server-xxxxxxxxxx-xxxxx    1/1     Running   0          2m
brockley-server-xxxxxxxxxx-xxxxx    1/1     Running   0          2m
brockley-worker-xxxxxxxxxx-xxxxx    1/1     Running   0          2m
brockley-worker-xxxxxxxxxx-xxxxx    1/1     Running   0          2m
brockley-coderunner-xxxxxxxx-xxxx   1/1     Running   0          2m
brockley-web-xxxxxxxxxx-xxxxx       1/1     Running   0          2m
```

Check health:

```bash
kubectl port-forward svc/brockley-server -n brockley 8000:8000 &
curl http://localhost:8000/health
```

Expected:

```json
{"status":"ok"}
```

Check readiness:

```bash
curl http://localhost:8000/health/ready
```

Expected:

```json
{"status":"ready","database":"ok","redis":"ok"}
```

Kill the port-forward:

```bash
kill %1
```

## Step 7: Set LLM API Key Secrets

```bash
kubectl create secret generic brockley-llm-keys \
  --namespace brockley \
  --from-literal=BROCKLEY_SECRET_OPENAI_API_KEY="sk-your-openai-key" \
  --from-literal=BROCKLEY_SECRET_ANTHROPIC_API_KEY="sk-ant-your-anthropic-key"
```

Patch deployments to use the secret:

```bash
kubectl set env deployment/brockley-server -n brockley --from=secret/brockley-llm-keys
kubectl set env deployment/brockley-worker -n brockley --from=secret/brockley-llm-keys
```

Wait for rollout:

```bash
kubectl rollout status deployment/brockley-server -n brockley
kubectl rollout status deployment/brockley-worker -n brockley
```

See [Secrets guide](secrets.md) for details on the `BROCKLEY_SECRET_*` convention.

## Step 8: Access the Web UI

Get the LoadBalancer external IP:

```bash
kubectl get svc brockley-web -n brockley
```

Expected output:

```
NAME           TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)        AGE
brockley-web   LoadBalancer   10.x.x.x      20.x.x.x        80:xxxxx/TCP   5m
```

Open `http://EXTERNAL-IP` in your browser.

For HTTPS with Azure-managed certificates, see [Helm guide -- Ingress configuration](helm.md#ingress-configuration).

## Step 9: Connect the CLI

```bash
go install github.com/brockleyai/brockleyai/cmd/brockley@latest

brockley config set-server http://EXTERNAL-IP:8000
brockley config set-api-key YOUR_API_KEY

brockley graph list
```

## Updating

```bash
helm upgrade brockley deploy/helm/brockley \
  --namespace brockley \
  -f deploy/helm/brockley/values-standard.yaml \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL" \
  --set server.image.tag="NEW_VERSION"
```

Database migrations run automatically on server startup.

## Tearing Down

Remove Brockley but keep infrastructure:

```bash
helm uninstall brockley --namespace brockley
```

Destroy all infrastructure:

```bash
cd deploy/terraform/azure
terraform destroy
```

Then delete the resource group (removes anything Terraform missed):

```bash
az group delete --name brockley-rg --yes --no-wait
```

## Cost Estimate

Rough monthly estimates (East US, standard preset):

| Resource | Estimated Cost |
|----------|---------------|
| AKS cluster (free control plane) | $0 |
| AKS node pool (2x Standard_D2s_v3) | ~$120-140 |
| Azure DB for PostgreSQL (GP_Standard_D2s_v3, 32 GB) | ~$100-130 |
| Azure Cache for Redis (Basic C1) | ~$25 |
| Load Balancer | ~$20 |
| **Total** | **~$265-315/month** |

Costs vary by usage. AKS control plane is free; you pay only for node VMs.

## Next Steps

- [Secrets guide](secrets.md) -- configure additional LLM provider keys
- [Helm values reference](helm.md) -- tune replicas, resources, and autoscaling
- [Configuration reference](configuration.md) -- all environment variables
- [Monitoring](monitoring.md) -- set up Prometheus metrics and trace export
