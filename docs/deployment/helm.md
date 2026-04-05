# Helm Deployment (Bring Your Own Cluster)

Deploy Brockley to any Kubernetes 1.24+ cluster using the Helm chart. This guide covers the full values reference, Ingress configuration, KEDA autoscaling, and production tuning.

If you need to provision cloud infrastructure first, see the cloud-specific guides: [GCP](gcp.md) | [AWS](aws.md) | [Azure](azure.md).

## Prerequisites

- Kubernetes cluster (v1.24+)
- [Helm](https://helm.sh/docs/intro/install/) >= 3.0
- [kubectl](https://kubernetes.io/docs/tasks/tools/) configured for your cluster
- External PostgreSQL instance (or use the embedded option for testing)
- External Redis instance (or use the embedded option for testing)

## Quick Start

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai

helm install brockley deploy/helm/brockley \
  --namespace brockley \
  --create-namespace \
  --set postgresql.uri="postgres://user:pass@your-db:5432/brockley?sslmode=require" \
  --set redis.url="redis://your-redis:6379/0" \
  --set auth.apiKeys="your-api-key" \
  --set encryptionKey="$(openssl rand -base64 32)" \
  --set env=production
```

## Chart Info

| Field | Value |
|-------|-------|
| Chart name | `brockley` |
| Chart version | `0.1.0` |
| App version | `0.1.0` |
| Source | `deploy/helm/brockley/` |

## What Gets Deployed

| Resource | Kind | Description |
|----------|------|-------------|
| brockley-server | Deployment | API server (REST, GraphQL, WebSocket) |
| brockley-worker | Deployment | Async task processor (asynq) |
| brockley-coderunner | Deployment | Python code execution tier |
| brockley-web | Deployment | Web UI (graph editor) |
| brockley-config | ConfigMap | Non-sensitive configuration |
| brockley-secret | Secret | API keys, encryption key |
| brockley-server | Service | ClusterIP for the API server |
| brockley-web | Service | LoadBalancer for the web UI |
| brockley-ingress | Ingress | (optional) External access with TLS |
| brockley-worker | ScaledObject | (optional) KEDA autoscaler for workers |

---

## Complete Values Reference

### Global

```yaml
# Environment: development | staging | production
# Controls log defaults and security behavior.
env: production
```

### Server

The API server handles REST endpoints, GraphQL, and WebSocket streaming.

```yaml
server:
  # Number of server pods. Stateless -- scale freely.
  replicas: 2

  image:
    repository: ghcr.io/brockleyai/brockley-server
    tag: ""              # defaults to chart appVersion
    pullPolicy: IfNotPresent

  # Port the server listens on inside the container.
  port: 8000

  resources:
    requests:
      cpu: 250m
      memory: 256Mi
    limits:
      cpu: "1"
      memory: 512Mi

  # Additional environment variables injected into server pods.
  # These are added to the ConfigMap alongside the chart's defaults.
  env: {}
  #   BROCKLEY_LOG_LEVEL: debug
  #   BROCKLEY_METRICS_ENABLED: "true"
  #   BROCKLEY_CORS_ORIGINS: "https://app.example.com"
```

**Health probes** (built into the deployment template, not configurable via values):

| Probe | Path | Initial Delay | Period | Timeout | Failure Threshold |
|-------|------|--------------|--------|---------|-------------------|
| Liveness | `/health` | 10s | 15s | 5s | 3 |
| Readiness | `/health/ready` | 5s | 10s | 5s | 3 |

- `/health` returns 200 if the server process is alive.
- `/health/ready` returns 200 only if PostgreSQL and Redis are reachable.

### Worker

Workers process async tasks from Redis queues via asynq. Tasks include graph orchestration (`graph:start`), LLM calls (`node:llm-call`), MCP calls (`node:mcp-call`), code execution (`node:code-exec`), and more.

```yaml
worker:
  # Number of worker pods. Stateless -- scale freely.
  replicas: 2

  # Max concurrent tasks per worker pod.
  # Total cluster concurrency = replicas * concurrency.
  concurrency: 10

  image:
    repository: ghcr.io/brockleyai/brockley-worker
    tag: ""
    pullPolicy: IfNotPresent

  resources:
    requests:
      cpu: 250m
      memory: 256Mi
    limits:
      cpu: "1"
      memory: 512Mi

  # Additional environment variables for worker pods.
  env: {}
```

### Coderunner

The coderunner provides sandboxed Python execution for code nodes and superagent code-exec tasks.

```yaml
coderunner:
  # Number of coderunner pods.
  replicas: 1

  # Max concurrent code executions per pod.
  concurrency: 3

  image:
    repository: ghcr.io/brockleyai/brockley-coderunner
    tag: ""
    pullPolicy: IfNotPresent

  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 512Mi

  # Additional environment variables for coderunner pods.
  env: {}
```

### Web UI

The web UI provides a visual graph editor for building and monitoring agent workflows.

```yaml
webUI:
  # Number of web UI pods.
  replicas: 1

  image:
    repository: ghcr.io/brockleyai/brockley-web
    tag: ""
    pullPolicy: IfNotPresent

  # Port the web server listens on.
  port: 80

  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 500m
      memory: 256Mi
```

### PostgreSQL

```yaml
postgresql:
  # Connection string for an external PostgreSQL instance.
  # This takes precedence over the embedded option.
  # Use sslmode=require for production.
  uri: ""

  # Embedded single-pod PostgreSQL. NOT for production.
  # No replication, no automated backups, no HA.
  embedded:
    enabled: false
    auth:
      postgresPassword: brockley
    persistence:
      size: 10Gi
```

**Production:** Always use an external managed PostgreSQL (RDS, Cloud SQL, Azure Database for PostgreSQL, or similar). The embedded option is for local testing only.

### Redis

```yaml
redis:
  # URL for an external Redis instance.
  # This takes precedence over the embedded option.
  url: ""

  # Embedded single-pod Redis. NOT for production.
  # No persistence, no replication.
  embedded:
    enabled: false
    auth:
      password: ""
```

**Production:** Always use an external managed Redis (ElastiCache, Memorystore, Azure Cache for Redis, or similar).

### Ingress

```yaml
ingress:
  enabled: false
  host: brockley.local

  tls:
    enabled: false
    secretName: brockley-tls

  annotations: {}
  #   kubernetes.io/ingress.class: nginx
  #   cert-manager.io/cluster-issuer: letsencrypt-prod
```

### Auth and Encryption

```yaml
auth:
  # Comma-separated API keys. When set, all API requests must include
  # Authorization: Bearer <key>. Leave empty to disable auth (dev only).
  apiKeys: ""

# 32-byte base64 key for encrypting secrets at rest.
# Generate with: openssl rand -base64 32
encryptionKey: ""
```

### Metrics

```yaml
metrics:
  # When true, server exposes Prometheus metrics at /metrics.
  enabled: false
```

### KEDA (Worker Autoscaling)

KEDA is not configured in the default `values.yaml`. It is enabled via the size presets (standard and performance) or by adding these values:

```yaml
keda:
  # Enable KEDA ScaledObject for worker autoscaling.
  enabled: false

  # Min/max worker replicas.
  minReplicas: 2
  maxReplicas: 4

  # How often (seconds) KEDA checks the Redis queue.
  pollingInterval: 15

  # Seconds to wait after last trigger before scaling down.
  cooldownPeriod: 120

  # Number of pending tasks in the asynq queue that triggers a scale-up.
  threshold: "20"
```

KEDA must be installed in your cluster before enabling this. See [KEDA setup](#keda-setup) below.

---

## Size Presets

The chart ships with three preset files. Use them as a starting point and override individual values as needed.

### Starter

Minimal footprint for evaluation and small workloads.

```bash
helm install brockley deploy/helm/brockley \
  -f deploy/helm/brockley/values-starter.yaml \
  --namespace brockley --create-namespace \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL"
```

| Component | Replicas | CPU (req/limit) | Memory (req/limit) |
|-----------|---------|----------------|-------------------|
| Server | 1 | 100m / 500m | 256Mi / 512Mi |
| Worker | 1 (concurrency: 5) | 100m / 500m | 256Mi / 512Mi |
| Coderunner | 1 (concurrency: 2) | 100m / 250m | 128Mi / 256Mi |
| Web UI | 1 | 50m / 250m | 64Mi / 128Mi |

KEDA: disabled.

### Standard

Production workloads with autoscaling.

```bash
helm install brockley deploy/helm/brockley \
  -f deploy/helm/brockley/values-standard.yaml \
  --namespace brockley --create-namespace \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL"
```

| Component | Replicas | CPU (req/limit) | Memory (req/limit) |
|-----------|---------|----------------|-------------------|
| Server | 2 | 250m / 1 | 512Mi / 1Gi |
| Worker | 2 (concurrency: 10) | 250m / 1 | 512Mi / 1Gi |
| Coderunner | 1 (concurrency: 5) | 200m / 500m | 256Mi / 512Mi |
| Web UI | 2 | 100m / 500m | 128Mi / 256Mi |

KEDA: enabled (2-4 workers, 15s polling, 120s cooldown, threshold 20).

### Performance

High-throughput production.

```bash
helm install brockley deploy/helm/brockley \
  -f deploy/helm/brockley/values-performance.yaml \
  --namespace brockley --create-namespace \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL"
```

| Component | Replicas | CPU (req/limit) | Memory (req/limit) |
|-----------|---------|----------------|-------------------|
| Server | 3 | 500m / 2 | 1Gi / 2Gi |
| Worker | 4 (concurrency: 20) | 500m / 2 | 1Gi / 2Gi |
| Coderunner | 2 (concurrency: 10) | 500m / 1 | 512Mi / 1Gi |
| Web UI | 2 | 100m / 500m | 128Mi / 256Mi |

KEDA: enabled (4-8 workers, 10s polling, 60s cooldown, threshold 10).

---

## KEDA Setup

[KEDA](https://keda.sh) (Kubernetes Event-Driven Autoscaling) scales worker pods based on the number of pending tasks in the asynq Redis queue. Install it before enabling the `keda.enabled` value.

### Install KEDA

```bash
helm repo add kedacore https://kedacore.github.io/charts
helm repo update

helm install keda kedacore/keda \
  --namespace keda \
  --create-namespace
```

Verify KEDA is running:

```bash
kubectl get pods -n keda
```

Expected:

```
NAME                                      READY   STATUS    RESTARTS   AGE
keda-operator-xxxxxxxxxx-xxxxx            1/1     Running   0          1m
keda-metrics-apiserver-xxxxxxxxxx-xxxxx   1/1     Running   0          1m
```

### How It Works

The Helm chart creates a KEDA `ScaledObject` that watches the Redis list `asynq:{default}:pending`. When the number of pending tasks exceeds the threshold, KEDA scales up worker pods. When the queue drains, it scales back down after the cooldown period.

### Tuning

| Parameter | What it controls | Tune when... |
|-----------|-----------------|-------------|
| `keda.threshold` | Pending tasks per worker replica | Lower for faster response, higher for efficiency |
| `keda.pollingInterval` | Seconds between queue checks | Lower for faster scaling, higher to reduce Redis load |
| `keda.cooldownPeriod` | Seconds before scale-down | Higher to avoid flapping on bursty workloads |
| `keda.maxReplicas` | Ceiling on worker pods | Set based on your cluster capacity |

---

## Ingress Configuration

By default, the web UI is exposed via a LoadBalancer service. For production, you probably want Ingress with TLS.

### nginx-ingress + cert-manager

```yaml
ingress:
  enabled: true
  host: brockley.example.com
  tls:
    enabled: true
    secretName: brockley-tls
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
```

Prerequisites:
- [nginx-ingress controller](https://kubernetes.github.io/ingress-nginx/deploy/)
- [cert-manager](https://cert-manager.io/docs/installation/) with a ClusterIssuer named `letsencrypt-prod`

### AWS ALB Ingress

```yaml
ingress:
  enabled: true
  host: brockley.example.com
  tls:
    enabled: true
    secretName: brockley-tls
  annotations:
    kubernetes.io/ingress.class: alb
    alb.ingress.kubernetes.io/scheme: internet-facing
    alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:us-east-1:ACCOUNT:certificate/CERT-ID
    alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS": 443}]'
```

Prerequisite: [AWS Load Balancer Controller](https://kubernetes-sigs.github.io/aws-load-balancer-controller/).

### GCP GCE Ingress

```yaml
ingress:
  enabled: true
  host: brockley.example.com
  tls:
    enabled: true
    secretName: brockley-tls
  annotations:
    kubernetes.io/ingress.class: gce
    networking.gke.io/managed-certificates: brockley-cert
```

---

## Secrets Configuration

The Helm chart creates a Kubernetes Secret containing `BROCKLEY_API_KEYS` and `BROCKLEY_ENCRYPTION_KEY`. For LLM provider API keys, see the [Secrets guide](secrets.md).

Quick approach -- pass secrets at install time:

```bash
helm install brockley deploy/helm/brockley \
  --namespace brockley --create-namespace \
  --set auth.apiKeys="your-api-key" \
  --set encryptionKey="$(openssl rand -base64 32)" \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL"
```

For a values file approach, do not commit secrets to version control. Use `--set` overrides or a gitignored values file.

---

## Upgrading

```bash
helm upgrade brockley deploy/helm/brockley \
  --namespace brockley \
  -f deploy/helm/brockley/values-standard.yaml \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL"
```

Database migrations run automatically on server startup. No manual migration steps required.

To update to a specific image version:

```bash
helm upgrade brockley deploy/helm/brockley \
  --namespace brockley \
  --set server.image.tag="0.2.0" \
  --set worker.image.tag="0.2.0" \
  --set coderunner.image.tag="0.2.0" \
  --set webUI.image.tag="0.2.0"
```

## Uninstalling

```bash
helm uninstall brockley --namespace brockley
```

This removes all Kubernetes resources created by the chart. PersistentVolumeClaims from embedded databases are not deleted -- remove those manually if needed.

## Production Checklist

Before going to production, verify:

- [ ] External PostgreSQL with backups, replication, and SSL
- [ ] External Redis with persistence enabled
- [ ] API keys set (`auth.apiKeys`)
- [ ] Encryption key set (`encryptionKey`) for secrets at rest
- [ ] `env` set to `production`
- [ ] Ingress with TLS enabled (or LoadBalancer with external TLS termination)
- [ ] Resource requests and limits tuned for your workload
- [ ] At least 2 server replicas and 2 worker replicas
- [ ] KEDA installed and enabled for worker autoscaling
- [ ] Metrics enabled (`metrics.enabled: true`)
- [ ] Log aggregation configured (pods emit structured JSON to stdout)
- [ ] LLM API keys configured (see [Secrets guide](secrets.md))

## Troubleshooting

### Pods stuck in Pending

Check if the cluster has enough resources:

```bash
kubectl describe pod <pod-name> -n brockley
```

Look for `Insufficient cpu` or `Insufficient memory` events. Either reduce resource requests or add nodes.

### Server CrashLoopBackOff

Check logs:

```bash
kubectl logs deployment/brockley-server -n brockley
```

Common causes:
- Invalid `DATABASE_URL` -- verify the PostgreSQL connection string.
- PostgreSQL unreachable -- check security groups, VPC peering, or private service access.
- Invalid `REDIS_URL` -- verify the Redis connection string.

### Workers not processing tasks

```bash
kubectl logs deployment/brockley-worker -n brockley
```

Common causes:
- Redis unreachable -- verify `REDIS_URL`.
- Concurrency set to 0 -- check `worker.concurrency`.

### Web UI shows connection error

The web UI needs to reach the server API. Check the `BROCKLEY_API_URL` environment variable in the web UI deployment, and verify the server service is reachable from within the cluster.

## Next Steps

- [Secrets guide](secrets.md) -- configure LLM API keys
- [Configuration reference](configuration.md) -- all environment variables
- [Monitoring](monitoring.md) -- Prometheus metrics and trace export
- [Cloud deployment overview](cloud-deploy.md) -- infrastructure provisioning guides
