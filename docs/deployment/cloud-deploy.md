# Cloud Deployment Overview

This guide covers deploying Brockley to your own cloud account. You get a production-grade Kubernetes cluster with managed PostgreSQL, managed Redis, and Brockley running behind a load balancer.

## Why Deploy to Cloud

- **Scaling** -- Workers and servers scale independently. KEDA autoscales workers based on queue depth.
- **Reliability** -- Managed databases handle backups, failover, and patching.
- **Team access** -- A single deployment serves your whole team via the web UI and API.
- **Data control** -- Everything runs in your cloud account. No data leaves your infrastructure.

## Architecture

```
                         ┌──────────────────────────────────────────┐
                         │            Kubernetes Cluster            │
                         │                                         │
  Users ──► Load    ──►  │  ┌──────────┐    ┌──────────┐           │
            Balancer     │  │  Server   │    │  Server   │          │
                         │  │  (API)    │    │  (API)    │          │
                         │  └────┬─────┘    └────┬─────┘           │
                         │       │               │                 │
                         │       └───────┬───────┘                 │
                         │               │                         │
                         │       ┌───────▼───────┐                 │
                         │       │    Redis       │◄───────┐       │
                         │       │  (task queue)  │        │       │
                         │       └───────┬───────┘        │       │
                         │               │                │       │
                         │  ┌────────────┼────────────┐   │       │
                         │  │            │            │   │       │
                         │  ▼            ▼            ▼   │       │
                         │ ┌──────┐  ┌──────┐  ┌──────┐  │       │
                         │ │Worker│  │Worker│  │Coder.│  │       │
                         │ └──┬───┘  └──┬───┘  └──┬───┘  │       │
                         │    │         │         │      │       │
                         │    └─────────┼─────────┘      │       │
                         │              │                 │       │
                         │              ▼                 │       │
                         │       ┌─────────────┐         │       │
                         │       │  PostgreSQL  │         │       │
                         │       │  (state)     │         │       │
                         │       └─────────────┘         │       │
                         │                               │       │
                         │  ┌──────────┐                 │       │
                         │  │  Web UI  │─────────────────┘       │
                         │  └──────────┘                         │
                         └──────────────────────────────────────────┘
```

**Components:**

| Component | Role | Stateless? |
|-----------|------|-----------|
| Server | REST API, GraphQL, WebSocket streaming | Yes |
| Worker | Async task processor (graph orchestration, node execution) | Yes |
| Coderunner | Python code execution sandbox | Yes |
| Web UI | Visual graph editor | Yes |
| PostgreSQL | Graph definitions, execution state, audit log | -- |
| Redis | Task queue (asynq), pub/sub streaming | -- |

## Deployment Paths

Pick the guide that matches your cloud:

| Cloud | Guide | Infrastructure |
|-------|-------|---------------|
| Google Cloud | [GCP (GKE Autopilot)](gcp.md) | GKE Autopilot + Cloud SQL + Memorystore |
| Amazon Web Services | [AWS (EKS)](aws.md) | EKS + RDS PostgreSQL + ElastiCache |
| Microsoft Azure | [Azure (AKS)](azure.md) | AKS + Azure Database for PostgreSQL + Azure Cache for Redis |
| Existing cluster | [Helm (BYO cluster)](helm.md) | Any Kubernetes 1.24+ cluster |

All three cloud guides use the same Helm chart under the hood. The Terraform modules provision the managed infrastructure; Helm deploys Brockley into the cluster.

## Prerequisites

Install these tools before starting any cloud guide:

| Tool | Minimum Version | Install |
|------|----------------|---------|
| Terraform | 1.5+ | [terraform.io/downloads](https://developer.hashicorp.com/terraform/downloads) |
| kubectl | 1.24+ | [kubernetes.io/docs/tasks/tools](https://kubernetes.io/docs/tasks/tools/) |
| Helm | 3.0+ | [helm.sh/docs/intro/install](https://helm.sh/docs/intro/install/) |
| Git | 2.0+ | [git-scm.com](https://git-scm.com/) |

Cloud-specific CLIs are listed in each guide (gcloud, aws, az).

Optional:

| Tool | Purpose |
|------|---------|
| [KEDA](https://keda.sh) | Autoscale workers based on queue depth. Included in the standard and performance presets. |
| [cert-manager](https://cert-manager.io) | Automated TLS certificates for Ingress. |

## Size Presets

The Helm chart ships with three preset configurations. Pick one as a starting point and adjust later.

| Preset | Server Replicas | Worker Replicas | Worker Concurrency | KEDA | Use Case |
|--------|----------------|----------------|-------------------|------|----------|
| starter | 1 | 1 | 5 | Off | Evaluation, small teams |
| standard | 2 | 2 | 10 | On (2-4 workers) | Production workloads |
| performance | 3 | 4 | 20 | On (4-8 workers) | High-throughput production |

Apply a preset with `-f`:

```bash
helm install brockley ./deploy/helm/brockley \
  -f deploy/helm/brockley/values-starter.yaml \
  --namespace brockley --create-namespace \
  --set postgresql.uri="$DATABASE_URL" \
  --set redis.url="$REDIS_URL"
```

See [Helm deployment guide](helm.md) for the full values reference.

## What's Next

1. Follow the guide for your cloud: [GCP](gcp.md) | [AWS](aws.md) | [Azure](azure.md) | [Helm (BYO)](helm.md)
2. Configure LLM API keys and secrets: [Secrets guide](secrets.md)
3. Review all configuration options: [Configuration reference](configuration.md)
4. Set up monitoring: [Monitoring guide](monitoring.md)
