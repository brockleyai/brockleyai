# Local Development

This guide covers setting up Brockley for local development using Docker Compose.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) (v20+) with Docker Compose
- [Go](https://go.dev/dl/) (v1.22+) if you want to build or test locally outside Docker
- [Git](https://git-scm.com/)
- `make`

## Quick Start

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai
make dev
```

That's it. This starts all services and you can access:

- **API server**: http://localhost:8000
- **Web UI**: http://localhost:3000
- **PostgreSQL**: localhost:5432
- **Redis**: localhost:6379

## What `make dev` Does

`make dev` runs `docker compose -f deploy/docker/docker-compose.yml up --build`, which starts:

| Service | Description | Port |
|---------|-------------|------|
| `server` | API server (Go, hot-reload in dev mode) | 8000 |
| `worker` | Async task processor | (internal) |
| `web-ui` | React visual editor | 3000 |
| `postgresql` | PostgreSQL 16 (Alpine) | 5432 |
| `redis` | Redis 7 (Alpine) | 6379 |
| `seed` | Loads example graphs on startup | (exits after seeding) |

### Service Dependencies

The server and worker wait for PostgreSQL and Redis health checks to pass before starting. The seed container waits for the server to start.

## Docker Compose Configuration

The Docker Compose file is at `deploy/docker/docker-compose.yml`.

### Default Environment Variables

The dev setup uses these defaults:

| Variable | Value | Purpose |
|----------|-------|---------|
| `DATABASE_URL` | `postgres://brockley:brockley@postgresql:5432/brockley?sslmode=disable` | PostgreSQL connection |
| `REDIS_URL` | `redis://redis:6379/0` | Redis connection |
| `BROCKLEY_ENV` | `development` | Enables dev features |
| `BROCKLEY_LOG_LEVEL` | `debug` | Verbose logging |
| `BROCKLEY_LOG_FORMAT` | `text` | Human-readable logs (vs JSON in production) |

### Worker Configuration

| Variable | Value | Purpose |
|----------|-------|---------|
| `BROCKLEY_CONCURRENCY` | `5` | Number of concurrent task workers |

### Web UI Configuration

| Variable | Value | Purpose |
|----------|-------|---------|
| `BROCKLEY_API_URL` | `http://localhost:8000` | Where the UI sends API requests |

## Makefile Targets

| Target | Command | Description |
|--------|---------|-------------|
| `make dev` | `docker compose up --build` | Start development environment |
| `make dev-down` | `docker compose down` | Stop development environment |
| `make test` | `go test ./... -race -count=1` | Run all tests (no external services needed) |
| `make test-coverage` | `go test ./... -race -coverprofile=...` | Run tests with coverage report |
| `make lint` | `golangci-lint run ./...` | Run linters |
| `make build` | `go build -o bin/...` | Build server, worker, and CLI binaries |
| `make docker-build` | `docker build ...` | Build production Docker image |
| `make help` | | Show all available targets |

## Hot Reload

In development mode (`BROCKLEY_ENV=development`), the Docker setup uses a dev build target that supports hot-reload. Changes to Go source files trigger automatic rebuilds inside the container.

## Database

PostgreSQL runs in a Docker container with data persisted to a named volume (`pg-data`).

### Connecting Directly

```bash
psql postgres://brockley:brockley@localhost:5432/brockley
```

### Resetting the Database

To wipe the database and start fresh:

```bash
make dev-down
docker volume rm docker_pg-data
make dev
```

### Migrations

The server runs migrations automatically on startup. No manual migration steps are required.

## Seed Data

The `seed` container loads example graph files from the `examples/` directory into the server via the API. It runs once on startup and then exits.

To re-seed after the environment is already running, restart the seed container:

```bash
docker compose -f deploy/docker/docker-compose.yml restart seed
```

## Running Tests

Tests are designed to run without external services (PostgreSQL, Redis). All dependencies are mocked.

```bash
# Run all tests
make test

# Run tests for a specific package
go test ./engine/graph/... -v

# Run tests with coverage
make test-coverage
```

## Building Binaries Locally

If you want to build and run outside Docker:

```bash
make build
```

This produces three binaries in `bin/`:

| Binary | Purpose |
|--------|---------|
| `bin/brockley-server` | API server |
| `bin/brockley-worker` | Async worker |
| `bin/brockley` | CLI tool |

To run the server locally (requires PostgreSQL and Redis):

```bash
export DATABASE_URL="postgres://brockley:brockley@localhost:5432/brockley?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"
export BROCKLEY_ENV=development
./bin/brockley-server
```

## Troubleshooting Local Dev

### Port Conflicts

If port 8000, 3000, 5432, or 6379 is already in use, either stop the conflicting service or modify the port mappings in `deploy/docker/docker-compose.yml`.

### Docker Build Failures

```bash
# Clean rebuild
docker compose -f deploy/docker/docker-compose.yml build --no-cache
```

### Logs

```bash
# All services
docker compose -f deploy/docker/docker-compose.yml logs -f

# Specific service
docker compose -f deploy/docker/docker-compose.yml logs -f server
```

### Health Checks

```bash
# Server health
curl http://localhost:8000/health

# Readiness (DB + Redis)
curl http://localhost:8000/health/ready
```

## See Also

- [Configuration Reference](configuration.md) -- all environment variables
- [Kubernetes Deployment](kubernetes.md) -- production deployment
- [Monitoring](monitoring.md) -- Prometheus metrics, structured logging
- [Quickstart](../getting-started/quickstart.md) -- create and execute your first graph
