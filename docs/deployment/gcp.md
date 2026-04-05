# Deploy to Google Cloud (GKE Autopilot)

This guide walks through deploying Brockley to GCP using GKE Autopilot, Cloud SQL for PostgreSQL, and Memorystore for Redis.

**Time estimate:** 15-20 minutes (most of it waiting for Terraform).

## Prerequisites

- GCP account with billing enabled
- [gcloud CLI](https://cloud.google.com/sdk/docs/install) installed and authenticated
- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.5
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) >= 3.0

Authenticate with GCP:

```bash
gcloud auth login
gcloud auth application-default login
```

## Step 1: Enable Required APIs

```bash
gcloud services enable \
  container.googleapis.com \
  sqladmin.googleapis.com \
  redis.googleapis.com \
  compute.googleapis.com \
  servicenetworking.googleapis.com \
  --project YOUR_PROJECT_ID
```

Replace `YOUR_PROJECT_ID` with your GCP project ID.

## Step 2: Clone and Configure

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai/deploy/terraform/gcp
```

Create a `terraform.tfvars` file:

```hcl
project_id = "your-gcp-project-id"
region     = "us-central1"

# Size: "starter", "standard", or "performance"
size = "standard"

# Optional: customize database
db_tier        = "db-custom-2-7680"    # 2 vCPU, 7.5 GB RAM
db_disk_size   = 20                     # GB
redis_tier     = "BASIC"
redis_memory   = 1                      # GB
```

### Available Regions

Pick a region close to your users. Common choices:

| Region | Location |
|--------|----------|
| `us-central1` | Iowa |
| `us-east1` | South Carolina |
| `europe-west1` | Belgium |
| `europe-west2` | London |
| `asia-southeast1` | Singapore |
| `asia-northeast1` | Tokyo |

## Step 3: Deploy Infrastructure

```bash
terraform init
terraform plan
```

Review the plan. It will create:

- GKE Autopilot cluster
- Cloud SQL PostgreSQL instance (private IP)
- Memorystore Redis instance
- VPC with private service access
- Service account for workload identity

When the plan looks right:

```bash
terraform apply
```

Type `yes` when prompted. This takes 10-15 minutes.

Expected output on completion:

```
Apply complete! Resources: 12 added, 0 changed, 0 destroyed.

Outputs:

cluster_name     = "brockley-gke"
database_url     = "postgres://brockley:****@10.x.x.x:5432/brockley?sslmode=require"
redis_url        = "redis://10.x.x.x:6379/0"
region           = "us-central1"
```

## Step 4: Configure kubectl

```bash
gcloud container clusters get-credentials brockley-gke \
  --region us-central1 \
  --project YOUR_PROJECT_ID
```

Verify connectivity:

```bash
kubectl get nodes
```

Expected output (GKE Autopilot provisions nodes on demand):

```
NAME                                          STATUS   ROLES    AGE   VERSION
gk3-brockley-gke-default-pool-xxxxxxxx-xxx   Ready    <none>   5m    v1.28.x
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

Save the generated API key -- you will need it to connect the CLI and make API calls:

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

Check server health:

```bash
kubectl port-forward svc/brockley-server -n brockley 8000:8000 &
curl http://localhost:8000/health
```

Expected:

```json
{"status":"ok"}
```

Check readiness (confirms DB and Redis are connected):

```bash
curl http://localhost:8000/health/ready
```

Expected:

```json
{"status":"ready","database":"ok","redis":"ok"}
```

Kill the port-forward when done:

```bash
kill %1
```

## Step 7: Set LLM API Key Secrets

Brockley needs API keys to call LLM providers. Create a Kubernetes secret with your keys:

```bash
kubectl create secret generic brockley-llm-keys \
  --namespace brockley \
  --from-literal=BROCKLEY_SECRET_OPENAI_API_KEY="sk-your-openai-key" \
  --from-literal=BROCKLEY_SECRET_ANTHROPIC_API_KEY="sk-ant-your-anthropic-key"
```

Patch the server and worker deployments to mount the secret:

```bash
kubectl set env deployment/brockley-server -n brockley --from=secret/brockley-llm-keys
kubectl set env deployment/brockley-worker -n brockley --from=secret/brockley-llm-keys
```

The pods will restart automatically. Verify they come back up:

```bash
kubectl rollout status deployment/brockley-server -n brockley
kubectl rollout status deployment/brockley-worker -n brockley
```

See [Secrets guide](secrets.md) for details on the `BROCKLEY_SECRET_*` convention and alternative approaches.

## Step 8: Access the Web UI

Get the LoadBalancer external IP:

```bash
kubectl get svc brockley-web -n brockley
```

Expected output:

```
NAME           TYPE           CLUSTER-IP     EXTERNAL-IP     PORT(S)        AGE
brockley-web   LoadBalancer   10.x.x.x      34.x.x.x        80:xxxxx/TCP   5m
```

Open `http://EXTERNAL-IP` in your browser. The web UI graph editor is ready to use.

For HTTPS, set up Ingress with cert-manager. See [Helm guide -- Ingress configuration](helm.md#ingress-configuration).

## Step 9: Connect the CLI

Install the Brockley CLI:

```bash
go install github.com/brockleyai/brockleyai/cmd/brockley@latest
```

Point it at your deployment:

```bash
brockley config set-server http://EXTERNAL-IP:8000
brockley config set-api-key YOUR_API_KEY
```

Test the connection:

```bash
brockley graph list
```

## Updating

To update Brockley to a new version:

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

To remove Brockley but keep the infrastructure:

```bash
helm uninstall brockley --namespace brockley
```

To destroy all infrastructure:

```bash
cd deploy/terraform/gcp
terraform destroy
```

Type `yes` when prompted. This deletes the GKE cluster, Cloud SQL instance, Memorystore instance, and VPC.

## Cost Estimate

Rough monthly estimates (us-central1, standard preset):

| Resource | Estimated Cost |
|----------|---------------|
| GKE Autopilot | ~$70-150 (pay per pod resource) |
| Cloud SQL (db-custom-2-7680) | ~$50-80 |
| Memorystore (1 GB, BASIC) | ~$35 |
| Load Balancer | ~$18 |
| **Total** | **~$175-285/month** |

Costs vary by usage. GKE Autopilot charges per pod resource-second, so idle clusters cost less.

## Next Steps

- [Secrets guide](secrets.md) -- configure additional LLM provider keys
- [Helm values reference](helm.md) -- tune replicas, resources, and autoscaling
- [Configuration reference](configuration.md) -- all environment variables
- [Monitoring](monitoring.md) -- set up Prometheus metrics and trace export
