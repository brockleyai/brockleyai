# Kubernetes Deployment

Brockley provides a Helm chart for deploying to Kubernetes. This guide covers installation, configuration, and production considerations.

## Prerequisites

- Kubernetes cluster (v1.24+)
- [Helm](https://helm.sh/docs/intro/install/) (v3+)
- `kubectl` configured for your cluster

## Quick Install

### From the Repository

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai

helm install brockley deploy/helm/brockley \
  --namespace brockley \
  --create-namespace \
  --set postgresql.uri="postgres://user:pass@your-db:5432/brockley?sslmode=require" \
  --set redis.url="redis://your-redis:6379/0" \
  --set auth.apiKeys="your-api-key-here"
```

## Chart Overview

The Helm chart deploys these components:

| Component | Kind | Default Replicas |
|-----------|------|-----------------|
| Server | Deployment | 2 |
| Worker | Deployment | 2 |
| Web UI | Deployment | 1 |
| ConfigMap | ConfigMap | 1 |
| Secret | Secret | 1 |
| Ingress | Ingress | (disabled by default) |

## Values Reference

### Global

```yaml
# Environment: development | staging | production
env: production
```

### Server

```yaml
server:
  replicas: 2
  image:
    repository: ghcr.io/brockleyai/brockley-server
    tag: ""          # defaults to chart appVersion
    pullPolicy: IfNotPresent
  port: 8000
  resources:
    requests:
      cpu: 250m
      memory: 256Mi
    limits:
      cpu: "1"
      memory: 512Mi
  env: {}
  #   BROCKLEY_LOG_LEVEL: info
  #   BROCKLEY_METRICS_ENABLED: "true"
```

### Worker

```yaml
worker:
  replicas: 2
  concurrency: 10      # tasks per worker
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
  env: {}
```

### Web UI

```yaml
webUI:
  replicas: 1
  image:
    repository: ghcr.io/brockleyai/brockley-web
    tag: ""
    pullPolicy: IfNotPresent
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
  # External PostgreSQL (recommended for production)
  uri: "postgres://user:pass@your-rds.amazonaws.com:5432/brockley?sslmode=require"

  # OR: embedded single-pod PostgreSQL (development/testing only)
  embedded:
    enabled: false
    auth:
      postgresPassword: brockley
    persistence:
      size: 10Gi
```

**For production, always use an external PostgreSQL instance** (AWS RDS, Google Cloud SQL, Azure Database for PostgreSQL, or a managed PostgreSQL service). The embedded option runs a single pod with no replication, backups, or high availability.

### Redis

```yaml
redis:
  # External Redis (recommended for production)
  url: "redis://your-redis.amazonaws.com:6379/0"

  # OR: embedded single-pod Redis (development/testing only)
  embedded:
    enabled: false
    auth:
      password: ""
```

**For production, use an external Redis instance** (AWS ElastiCache, Google Memorystore, Azure Cache for Redis). The embedded option has no persistence or replication.

### Ingress

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

### Authentication

```yaml
auth:
  # Comma-separated API keys
  apiKeys: "key1,key2"
```

### Encryption

```yaml
# 32-byte base64 key for encrypting secrets at rest
encryptionKey: "base64-encoded-32-byte-key"
```

Generate an encryption key:

```bash
openssl rand -base64 32
```

### Metrics

```yaml
metrics:
  enabled: true
```

When enabled, the server exposes a `/metrics` endpoint with Prometheus-compatible metrics.

## Embedded vs External Databases

| Aspect | Embedded | External |
|--------|----------|----------|
| Setup | Simple, no external dependencies | Requires provisioning |
| High availability | No (single pod) | Yes (managed service) |
| Backups | Manual | Automated (managed service) |
| Performance | Limited | Scalable |
| Recommended for | Development, testing | Staging, production |

### Example: External PostgreSQL + External Redis

```yaml
postgresql:
  uri: "postgres://brockley:secret@prod-db.cluster-xyz.us-east-1.rds.amazonaws.com:5432/brockley?sslmode=require"

redis:
  url: "rediss://prod-redis.xyz.ng.0001.use1.cache.amazonaws.com:6379/0"
```

### Example: Embedded (Development Only)

```yaml
postgresql:
  embedded:
    enabled: true
    auth:
      postgresPassword: devpassword
    persistence:
      size: 5Gi

redis:
  embedded:
    enabled: true
```

## Scaling

### Server Scaling

The API server is stateless. Scale horizontally by increasing replicas:

```yaml
server:
  replicas: 4
```

Or use a Horizontal Pod Autoscaler:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: brockley-server
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: brockley-server
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
```

### Worker Scaling

Workers are also stateless. Scale based on queue depth:

```yaml
worker:
  replicas: 4
  concurrency: 20    # 20 concurrent tasks per worker = 80 total
```

For bursty workloads, increase both replicas and concurrency.

## Production Checklist

Before deploying to production, verify:

- [ ] External PostgreSQL with backups, replication, and SSL
- [ ] External Redis with persistence enabled
- [ ] API keys configured (`auth.apiKeys`)
- [ ] Encryption key set (`encryptionKey`) for secrets at rest
- [ ] Ingress with TLS enabled
- [ ] Resource requests and limits tuned for your workload
- [ ] At least 2 server replicas and 2 worker replicas
- [ ] Monitoring enabled (`metrics.enabled: true`)
- [ ] Log aggregation configured (stdout/stderr from pods)
- [ ] Pod disruption budgets set (for rolling updates)

## Upgrading

```bash
# Update values if needed, then upgrade
helm upgrade brockley deploy/helm/brockley \
  --namespace brockley \
  -f my-values.yaml
```

Database migrations run automatically on server startup. No manual migration steps are required.

## Uninstalling

```bash
helm uninstall brockley --namespace brockley
```

This removes all Kubernetes resources created by the chart. It does not delete PersistentVolumeClaims from embedded databases -- delete those manually if needed.

## See Also

- [Configuration Reference](configuration.md) -- all environment variables
- [Local Development](local-dev.md) -- development setup with Docker Compose
- [Monitoring](monitoring.md) -- Prometheus metrics, health endpoints
- [Architecture Overview](../getting-started/architecture-overview.md) -- system components
