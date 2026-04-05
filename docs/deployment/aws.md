# Deploy to AWS (EKS)

This guide walks through deploying Brockley to AWS using EKS, RDS PostgreSQL, and ElastiCache Redis.

**Time estimate:** 20-25 minutes (EKS cluster creation takes ~15 minutes).

## Prerequisites

- AWS account with appropriate permissions
- [AWS CLI v2](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) installed and configured
- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.5
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/) >= 3.0

Configure AWS credentials:

```bash
aws configure
```

Or set environment variables:

```bash
export AWS_ACCESS_KEY_ID="your-access-key"
export AWS_SECRET_ACCESS_KEY="your-secret-key"
export AWS_DEFAULT_REGION="us-east-1"
```

### IAM Permissions Required

The IAM user or role running Terraform needs these permissions:

- `eks:*` -- EKS cluster management
- `ec2:*` -- VPC, subnets, security groups, NAT gateway
- `rds:*` -- RDS instance management
- `elasticache:*` -- ElastiCache management
- `iam:*` -- Service roles for EKS and node groups
- `sts:GetCallerIdentity` -- identity verification

For a least-privilege approach, use the AWS-managed policies:
- `AmazonEKSClusterPolicy`
- `AmazonEKSWorkerNodePolicy`
- `AmazonVPCFullAccess`
- `AmazonRDSFullAccess`
- `AmazonElastiCacheFullAccess`

## Step 1: Clone and Configure

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai/deploy/terraform/aws
```

Create a `terraform.tfvars` file:

```hcl
region = "us-east-1"

# Size: "starter", "standard", or "performance"
size = "standard"

# Optional: customize infrastructure
db_instance_class    = "db.t4g.medium"    # 2 vCPU, 4 GB RAM
db_allocated_storage = 20                  # GB
redis_node_type      = "cache.t4g.small"   # 2 vCPU, 1.37 GB RAM
```

### Available Regions

| Region | Location |
|--------|----------|
| `us-east-1` | N. Virginia |
| `us-west-2` | Oregon |
| `eu-west-1` | Ireland |
| `eu-central-1` | Frankfurt |
| `ap-southeast-1` | Singapore |
| `ap-northeast-1` | Tokyo |

## Step 2: Deploy Infrastructure

```bash
terraform init
terraform plan
```

Review the plan. It will create:

- EKS cluster with managed node group
- RDS PostgreSQL instance (private subnet)
- ElastiCache Redis cluster (private subnet)
- VPC with public/private subnets across 2 AZs
- NAT Gateway for outbound internet from private subnets
- Security groups for inter-service communication
- IAM roles for EKS and node groups

When the plan looks right:

```bash
terraform apply
```

Type `yes` when prompted. This takes 15-20 minutes (EKS cluster creation is the bottleneck).

Expected output on completion:

```
Apply complete! Resources: 28 added, 0 changed, 0 destroyed.

Outputs:

cluster_name     = "brockley-eks"
database_url     = "postgres://brockley:****@brockley-db.xxxxxxxx.us-east-1.rds.amazonaws.com:5432/brockley?sslmode=require"
redis_url        = "redis://brockley-redis.xxxxxx.0001.use1.cache.amazonaws.com:6379/0"
region           = "us-east-1"
```

## Step 3: Configure kubectl

```bash
aws eks update-kubeconfig \
  --name brockley-eks \
  --region us-east-1
```

Verify connectivity:

```bash
kubectl get nodes
```

Expected output:

```
NAME                                           STATUS   ROLES    AGE   VERSION
ip-10-0-1-xxx.us-east-1.compute.internal       Ready    <none>   5m    v1.28.x
ip-10-0-2-xxx.us-east-1.compute.internal       Ready    <none>   5m    v1.28.x
```

## Step 4: Install Brockley with Helm

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

## Step 5: Verify Deployment

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

## Step 6: Set LLM API Key Secrets

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

## Step 7: Access the Web UI

Get the LoadBalancer external hostname:

```bash
kubectl get svc brockley-web -n brockley
```

Expected output:

```
NAME           TYPE           CLUSTER-IP     EXTERNAL-IP                                                              PORT(S)        AGE
brockley-web   LoadBalancer   10.x.x.x      axxxxxxxxxxxxxxxxxxxxxxxxx-xxxxxxxxxx.us-east-1.elb.amazonaws.com        80:xxxxx/TCP   5m
```

AWS ELB hostnames take 1-2 minutes to become resolvable. Open `http://EXTERNAL-IP` in your browser.

For HTTPS with ACM certificates, see [Helm guide -- Ingress configuration](helm.md#ingress-configuration).

## Step 8: Connect the CLI

```bash
go install github.com/brockleyai/brockleyai/cmd/brockley@latest

brockley config set-server http://EXTERNAL-HOSTNAME:8000
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
cd deploy/terraform/aws
terraform destroy
```

Type `yes` when prompted. This deletes the EKS cluster, RDS instance, ElastiCache cluster, VPC, NAT Gateway, and all associated resources.

## Cost Estimate

Rough monthly estimates (us-east-1, standard preset):

| Resource | Estimated Cost |
|----------|---------------|
| EKS cluster | ~$73 (control plane) |
| EKS node group (2x t3.medium) | ~$60-80 |
| RDS (db.t4g.medium, 20 GB) | ~$55-70 |
| ElastiCache (cache.t4g.small) | ~$25 |
| NAT Gateway | ~$33 + data transfer |
| Load Balancer (ALB) | ~$22 |
| **Total** | **~$270-300/month** |

Costs vary by usage and data transfer.

## Next Steps

- [Secrets guide](secrets.md) -- configure additional LLM provider keys
- [Helm values reference](helm.md) -- tune replicas, resources, and autoscaling
- [Configuration reference](configuration.md) -- all environment variables
- [Monitoring](monitoring.md) -- set up Prometheus metrics and trace export
