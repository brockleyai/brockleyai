# Docker Deployment

Run Brockley locally with Docker Compose. This is the fastest way to get a complete environment running.

## Quick Start

```bash
docker compose -f deploy/docker/docker-compose.yml up --build
```

Or from the project root:

```bash
make dev
```

This starts the following services:

| Service | Description | Port |
|---|---|---|
| `server` | Brockley API server | `8000` |
| `worker` | Async task processor | -- |
| `coderunner` | Code execution runtime (Python) | -- |
| `web-ui` | Visual graph editor | `3000` |
| `seed` | Loads example graphs on first run | -- |
| `postgresql` | PostgreSQL 16 database | `5432` |
| `redis` | Redis 7 for task queue and streaming | `6379` |

The web UI is available at [http://localhost:3000](http://localhost:3000) and the API at [http://localhost:8000/api/v1/](http://localhost:8000/api/v1/).

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://brockley:brockley@postgresql:5432/brockley` |
| `REDIS_URL` | Redis connection string | `redis://redis:6379` |
| `BROCKLEY_API_KEYS` | Comma-separated API keys (empty = dev mode, no auth) | (none) |
| `BROCKLEY_PORT` | Server listen port | `8000` |
| `BROCKLEY_CONCURRENCY` | Worker concurrency | `5` |
| `BROCKLEY_LOG_LEVEL` | Log level | `info` |
| `OPENROUTER_API_KEY` | OpenRouter API key (for LLM examples) | (none) |

## Customization

Override environment variables by creating a `.env` file in the project root (the compose file loads it automatically):

```env
OPENROUTER_API_KEY=sk-or-v1-...
BROCKLEY_LOG_LEVEL=debug
```

Mount custom configuration files by adding volumes in a `docker-compose.override.yml`.

## Data Persistence

PostgreSQL data is persisted in a named Docker volume (`pg-data`). To reset all data:

```bash
docker compose -f deploy/docker/docker-compose.yml down -v
```
