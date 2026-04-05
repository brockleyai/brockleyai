# Health and Info API

Health, readiness, version, and metrics endpoints. These do not require authentication.

## Health Check (Liveness)

```
GET /health
```

Returns `200 OK` if the server process is alive. Does not check dependencies.

```bash
curl http://localhost:8000/health
```

Response:

```json
{
  "status": "healthy"
}
```

Use this for Kubernetes liveness probes.

## Readiness Check

```
GET /health/ready
```

Returns `200 OK` if the server, PostgreSQL, and Redis are all connected. Returns `503 Service Unavailable` if any dependency is unreachable.

```bash
curl http://localhost:8000/health/ready
```

Response (healthy):

```json
{
  "status": "ready",
  "components": {
    "postgresql": "connected",
    "redis": "connected"
  }
}
```

Response (unhealthy):

```json
{
  "status": "not_ready",
  "components": {
    "postgresql": "connected",
    "redis": "connection refused"
  }
}
```

Use this for Kubernetes readiness probes and load balancer health checks.

## Version

```
GET /version
```

```bash
curl http://localhost:8000/version
```

Response:

```json
{
  "version": "0.1.0",
  "build": "abc1234",
  "go_version": "go1.24"
}
```

## Metrics

```
GET /metrics
```

Prometheus exposition format. Only available when `BROCKLEY_METRICS_ENABLED=true`.

```bash
curl http://localhost:8000/metrics
```

Returns Prometheus text format with execution, node, provider, MCP, queue, and HTTP metrics. See [Monitoring](../deployment/monitoring.md) for the full list of metrics.

## See Also

- [Monitoring](../deployment/monitoring.md) -- Prometheus metrics, structured logging, health endpoints
- [Configuration Reference](../deployment/configuration.md) -- `BROCKLEY_METRICS_ENABLED`
- [Kubernetes Deployment](../deployment/kubernetes.md) -- probe configuration
