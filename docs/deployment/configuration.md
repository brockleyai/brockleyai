# Configuration Reference

Brockley is configured through environment variables. This page lists every variable, its default value, and what it controls.

## Database and Cache

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_DATABASE_URL` | (none) | PostgreSQL connection string. Example: `postgres://user:pass@host:5432/brockley?sslmode=require` |
| `DATABASE_URL` | (none) | Alternative name for `BROCKLEY_DATABASE_URL` (also accepted by the Docker Compose setup) |
| `BROCKLEY_REDIS_URL` | (none) | Redis connection string. Example: `redis://host:6379/0`. Required for async execution and streaming. |
| `REDIS_URL` | (none) | Alternative name for `BROCKLEY_REDIS_URL` (also accepted by the Docker Compose setup) |

Both `DATABASE_URL` and `BROCKLEY_DATABASE_URL` are supported. The Docker Compose dev setup uses `DATABASE_URL`; the application code reads `BROCKLEY_DATABASE_URL`. Either works.

## Server

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_HOST` | `0.0.0.0` | Address the server binds to |
| `BROCKLEY_PORT` | `8000` | Port the server listens on |
| `BROCKLEY_ENV` | `production` | Environment mode. `development` enables debug features and relaxed security. `production` is the default. |

## Logging

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `BROCKLEY_LOG_FORMAT` | `json` | Log format: `json` (structured, for production) or `text` (human-readable, for development) |

## Security

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_API_KEYS` | (none) | Comma-separated list of valid API keys. When set, all API requests (except health endpoints) must include `Authorization: Bearer <key>`. When empty, no authentication is required. |
| `BROCKLEY_CORS_ORIGINS` | (none) | Allowed CORS origins. Example: `http://localhost:3000,https://app.example.com`. When empty, CORS is not restricted. |
| `BROCKLEY_ENCRYPTION_KEY` | (none) | 32-byte key for encrypting secrets at rest (base64-encoded). Used for storing LLM API keys and other sensitive values securely. |

### API Key Authentication

When `BROCKLEY_API_KEYS` is set, clients must include the API key in requests:

```bash
curl -H "Authorization: Bearer your-api-key" http://localhost:8000/api/v1/graphs
```

Health endpoints (`/health`, `/health/ready`, `/version`) do not require authentication.

Multiple API keys can be configured (comma-separated) for key rotation:

```
BROCKLEY_API_KEYS=key-v2-abc123,key-v1-old456
```

### Generating an Encryption Key

```bash
openssl rand -base64 32
```

## Observability

### Metrics

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_METRICS_ENABLED` | `false` | When `true`, exposes Prometheus metrics at `/metrics` |

### Tracing

Brockley supports OpenInference-compatible trace export for LLM observability platforms.

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_TRACE_ENABLED` | `false` | Enable trace export |
| `BROCKLEY_TRACE_ENDPOINT` | (none) | Trace collector endpoint URL |
| `BROCKLEY_TRACE_API_KEY` | (none) | API key for the trace collector |
| `BROCKLEY_TRACE_PROJECT` | (none) | Project name in the trace collector |
| `BROCKLEY_TRACE_PROTOCOL` | (none) | Protocol for trace export (e.g., `grpc`, `http`) |
| `BROCKLEY_TRACE_INSECURE` | `false` | Use insecure connection to trace collector |

### Example: Phoenix Trace Export

```bash
BROCKLEY_TRACE_ENABLED=true
BROCKLEY_TRACE_ENDPOINT=http://phoenix:6006/v1/traces
BROCKLEY_TRACE_PROTOCOL=http
```

## Multi-Tenancy

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_NAMESPACE` | `default` | Default namespace for graphs and resources |

## Worker

| Variable | Default | Description |
|----------|---------|-------------|
| `BROCKLEY_CONCURRENCY` | `10` | Number of concurrent task executions per worker process |

### Queue Configuration

Workers process tasks from two queues with different priorities:

| Queue | Priority | Task Types | Description |
|-------|----------|------------|-------------|
| `orchestrator` | 3 (lower) | `graph:start` | Long-running orchestrator tasks that wait for node results |
| `nodes` | 7 (higher) | `node:llm-call`, `node:mcp-call`, `node:run` | Short-lived node tasks that should be picked up quickly |

Higher priority means the queue gets more processing slots. The `nodes` queue is prioritized so that dispatched work completes quickly while orchestrators wait.

## Example: Minimal Production Configuration

```bash
# Database
BROCKLEY_DATABASE_URL=postgres://brockley:secretpass@db.internal:5432/brockley?sslmode=require

# Redis
BROCKLEY_REDIS_URL=redis://redis.internal:6379/0

# Security
BROCKLEY_API_KEYS=prod-key-abc123
BROCKLEY_ENCRYPTION_KEY=base64encodedkey==

# Logging
BROCKLEY_LOG_LEVEL=info
BROCKLEY_LOG_FORMAT=json

# Metrics
BROCKLEY_METRICS_ENABLED=true
```

## Example: Development Configuration

```bash
# Database
BROCKLEY_DATABASE_URL=postgres://brockley:brockley@localhost:5432/brockley?sslmode=disable

# Redis
BROCKLEY_REDIS_URL=redis://localhost:6379/0

# Development mode
BROCKLEY_ENV=development
BROCKLEY_LOG_LEVEL=debug
BROCKLEY_LOG_FORMAT=text

# No auth in development
# BROCKLEY_API_KEYS is not set

# CORS for local web UI
BROCKLEY_CORS_ORIGINS=http://localhost:3000
```

## Example: Docker Compose Override

To customize environment variables in the Docker Compose dev setup, create a `docker-compose.override.yml`:

```yaml
services:
  server:
    environment:
      - BROCKLEY_LOG_LEVEL=info
      - BROCKLEY_METRICS_ENABLED=true
      - BROCKLEY_API_KEYS=dev-key-123
```

## See Also

- [Local Development](local-dev.md) -- getting started with Docker Compose
- [Kubernetes Deployment](kubernetes.md) -- production deployment with Helm
- [Monitoring](monitoring.md) -- metrics, logging, tracing configuration
- [Architecture Overview](../getting-started/architecture-overview.md) -- system components
