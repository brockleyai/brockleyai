# Helm Chart -- Brockley

Deploy Brockley to Kubernetes using Helm.

## Install

```bash
helm install brockley ./deploy/helm/brockley
```

### With Embedded Databases

For evaluation or small deployments, enable the bundled PostgreSQL and Redis:

```bash
helm install brockley ./deploy/helm/brockley \
  --set postgresql.embedded.enabled=true \
  --set redis.embedded.enabled=true
```

### With External Databases

For production, point to your own managed databases:

```bash
helm install brockley ./deploy/helm/brockley \
  --set postgresql.uri="postgres://user:pass@your-db:5432/brockley" \
  --set redis.url="redis://your-redis:6379"
```

## Key Values

| Value | Description | Default |
|---|---|---|
| `server.replicas` | Number of server replicas | `2` |
| `server.image` | Server container image | `ghcr.io/brockleyai/brockleyai:latest` |
| `server.port` | Server listen port | `8080` |
| `worker.replicas` | Number of worker replicas | `2` |
| `worker.concurrency` | Goroutines per worker pod | `4` |
| `coderunner.enabled` | Deploy coderunner for Python code execution | `false` |
| `coderunner.replicas` | Number of coderunner replicas | `1` |
| `coderunner.image` | Coderunner container image | `ghcr.io/brockleyai/coderunner:latest` |
| `coderunner.concurrency` | Concurrent code executions per pod | `4` |
| `postgresql.embedded.enabled` | Deploy bundled PostgreSQL | `false` |
| `postgresql.uri` | External PostgreSQL connection string | -- |
| `redis.embedded.enabled` | Deploy bundled Redis | `false` |
| `redis.url` | External Redis connection string | -- |
| `ingress.enabled` | Enable Ingress resource | `false` |
| `ingress.host` | Ingress hostname | `brockley.example.com` |
| `ingress.tls.enabled` | Enable TLS on Ingress | `false` |
| `ingress.tls.secretName` | TLS secret name | -- |
| `apiKeys` | Comma-separated API keys (use a Secret in production) | -- |

## Ingress

Enable ingress and configure your hostname:

```bash
helm install brockley ./deploy/helm/brockley \
  --set ingress.enabled=true \
  --set ingress.host=brockley.example.com \
  --set ingress.tls.enabled=true \
  --set ingress.tls.secretName=brockley-tls
```

Works with any Ingress controller (nginx, Traefik, ALB, etc.).

## Coderunner

The coderunner component processes `node:code-exec` tasks from the `code` queue, enabling superagent nodes to execute Python code. It is an optional component -- only needed when superagent nodes have `code_execution.enabled: true`.

```bash
helm install brockley ./deploy/helm/brockley \
  --set coderunner.enabled=true \
  --set coderunner.replicas=2
```

### Security Considerations

The coderunner executes user-provided Python code. Apply defense-in-depth:

- **Network policy:** Restrict coderunner pods to only communicate with Redis. User code has no direct network access; external interactions go through the Redis relay protocol and are dispatched as normal tool calls by the superagent handler.
- **Resource limits:** Set CPU and memory limits on coderunner pods. The `CodeExecutionConfig` enforces per-execution limits (`max_memory_mb`, `max_execution_time_sec`), but pod-level limits provide an additional safety net.
- **Security context:** Run coderunner pods as non-root with a read-only root filesystem and dropped capabilities.
- **Module allowlist:** The `allowed_modules` config restricts which Python modules user code can import. The default set includes safe modules (json, math, re, datetime, collections, etc.) and excludes os, subprocess, socket, and other sensitive modules.

## Production Checklist

- [ ] Use external managed PostgreSQL with backups enabled
- [ ] Use external managed Redis or a Redis cluster
- [ ] Set API keys via a Kubernetes Secret, not plain values
- [ ] Enable TLS on the Ingress
- [ ] Configure resource requests and limits for all pods
- [ ] Enable horizontal pod autoscaling for server and worker
- [ ] Set `BROCKLEY_LOG_LEVEL=info` (avoid `debug` in production)
- [ ] Connect Prometheus scrape targets to `/metrics` on the server
- [ ] Review and set appropriate `worker.concurrency` for your workload
- [ ] If using code execution: enable coderunner, apply network policies, set resource limits, and run as non-root
