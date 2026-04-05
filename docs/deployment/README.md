# Deployment

Everything you need to run Brockley -- from local development to production cloud deployments. This section covers setup, configuration, cloud-specific modules, and operational monitoring.

## Local development

- **[Local Development](local-dev.md)** -- Start the full stack locally with Docker Compose (`make dev`). Prerequisites, first-run guide, and troubleshooting.

## Configuration

- **[Configuration Reference](configuration.md)** -- All environment variables, their defaults, and how they interact. This is the single reference for tuning Brockley's behavior.
- **[Secrets Management](secrets.md)** -- How to manage LLM API keys and other secrets: inline keys, secret references, and environment-based resolution.

## Kubernetes

- **[Kubernetes Deployment](kubernetes.md)** -- Deploy to Kubernetes with the Helm chart. Architecture, scaling, and production considerations.
- **[Helm Values Reference](helm.md)** -- Complete reference for every Helm chart value.

## Cloud deployment

- **[Cloud Deployment Overview](cloud-deploy.md)** -- Multi-cloud strategy: what the Terraform modules provision and how to choose between them.
- **[AWS](aws.md)** -- Deploy to AWS with EKS, RDS, and ElastiCache.
- **[GCP](gcp.md)** -- Deploy to GCP with GKE, Cloud SQL, and Memorystore.
- **[Azure](azure.md)** -- Deploy to Azure with AKS, Azure Database for PostgreSQL, and Azure Cache for Redis.

## Operations

- **[Monitoring and Observability](monitoring.md)** -- Prometheus metrics, structured logging, OpenTelemetry traces, and integrations with Langfuse, Opik, Phoenix, and LangSmith.

## Where to go next

- **[LLM Providers](../providers/)** -- Configure the LLM backends your deployment will use.
- **[CI/CD](../cicd/)** -- Automate deployments with GitHub Actions or GitLab CI.
- **[Terraform Provider](../terraform/)** -- Manage graphs as infrastructure-as-code (separate from deploying Brockley itself).
