<p align="center">
  <h1 align="center">Brockley</h1>
  <p align="center">Open-source infrastructure to build, deploy, and scale production AI agents.</p>
</p>

<p align="center">
  <a href="https://github.com/brockleyai/brockleyai/actions/workflows/ci.yml"><img src="https://github.com/brockleyai/brockleyai/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <img src="https://img.shields.io/badge/Go-1.24-00ADD8?logo=go" alt="Go 1.24">
</p>

---

Most AI agent frameworks help you prototype. Brockley helps you ship.

Define agent workflows as typed, validated graphs. Deploy them with Terraform. Run them on horizontally-scaling workers with durable execution, structured observability, and retry guarantees. Manage everything through six peer interfaces -- web UI, REST API, CLI, Terraform, MCP server, and coding agents -- with full parity.

If you're building AI-powered features into a real product -- customer support agents, document processing pipelines, code review bots, autonomous research workflows -- Brockley is the infrastructure layer that gets them to production.

**Self-hostable. Apache 2.0. No vendor lock-in.**

## Why Brockley

Most agent tooling falls into two camps: visual builders with no CI/CD story, or code libraries locked to one language with no deployment model. Brockley is neither. It's infrastructure you deploy, operate, and scale like the rest of your stack.

- **Workflows as code.** Agent graphs are JSON in your Git repo -- reviewable, diffable, versionable. Not opaque UI state.
- **Validate before you ship.** `brockley validate` runs 13 structural and type-safety checks locally with zero network calls. Run it in CI on every PR.
- **Deploy with Terraform.** `brockley_graph` is a Terraform resource. Plan, apply, import, destroy -- same workflow as your cloud infrastructure.
- **Strong typing everywhere.** Every node port has a JSON Schema. Edges enforce type compatibility. LLM nodes validate structured output against schemas.
- **Built for production.** Single static Go binary. Durable async execution with step-level tracking. Prometheus metrics, structured logging, OpenTelemetry traces. Retry and rate limiting on LLM providers.
- **Scale horizontally.** Workers auto-scale based on queue depth. Each LLM call, tool call, and code execution runs as a separate async task. The bottleneck is the LLM API, not Brockley.
- **Free cloud deployment.** [Brockley](https://brockley.ai) provisions and manages Brockley in your own cloud account -- pick your cloud, pick your region, running in minutes. Free for all users.

## Superagent: Autonomous AI Agents as Infrastructure

Superagent is a first-class node type that gives you a fully autonomous agent loop you can drop into any workflow. This is how you build AI agents for your product -- not as fragile scripts, but as bounded, observable, production-grade components.

A single Superagent node handles planning, tool calling, code execution, progress tracking, self-evaluation, and structured output assembly. It connects to any MCP server or REST API as skills, executes Python in a sandbox, and manages its own task list and shared memory across iterations.

**What makes it production-ready:**

- **Bounded execution.** Five-layer termination: max iterations, max tool calls, per-iteration tool limits, wall-clock timeout, and stuck detection with automatic reflection.
- **Distributed by design.** The coordinator stays alive while dispatching LLM calls, MCP calls, and code execution as separate async tasks across your worker pool.
- **Observable.** 10+ event types stream progress in real-time -- iteration starts, tool calls, evaluations, reflections, completions.
- **Composable.** Drop a Superagent into a larger graph alongside LLM nodes, conditionals, transforms, and other Superagents. Chain autonomous agents with deterministic logic.

Use it for anything that needs multi-step autonomy: research agents that gather and synthesize information, support agents that diagnose and resolve tickets, data pipelines that adapt their approach based on what they find.

```json
{
  "type": "superagent",
  "config": {
    "model": "anthropic/claude-sonnet-4-20250514",
    "provider": "openrouter",
    "system_prompt": "You are a research analyst...",
    "skills": [{ "mcp_server": "web-search" }, { "mcp_server": "database" }],
    "max_iterations": 15,
    "timeout_seconds": 120,
    "code_execution": { "enabled": true }
  }
}
```

See the [Superagent guide](docs/concepts/superagent.md) for the full reference.

## Quickstart

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/)
- [Go 1.24+](https://go.dev/dl/) (for the CLI)

### 1. Clone and start

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai
make dev
```

This starts PostgreSQL, Redis, the API server (`:8000`), a worker, a code runner, and the web UI (`:3000`). Example graphs are seeded automatically.

### 2. Install the CLI

```bash
go install ./cmd/brockley/
export PATH=$PATH:$(go env GOPATH)/bin
```

### 3. Deploy and run a graph

```bash
# No API key needed for this one
brockley deploy -f examples/comprehensive/graph.json

# Run it
brockley invoke <graph_id> --input '{"data": {"text": "hello world", "number": 42, "tier": "premium"}}' --sync
```

### 4. Try with an LLM

Set an [OpenRouter](https://openrouter.ai/) API key to unlock LLM-powered graphs (includes free models):

```bash
export OPENROUTER_API_KEY="sk-or-v1-your-key-here"
brockley deploy -f examples/llm-pipeline/graph.json --env-file .env
```

### 5. Open the UI

Open [http://localhost:3000](http://localhost:3000) to build and run graphs visually.

## Production Infrastructure

Brockley is designed to run like the rest of your infrastructure -- not as a side project on someone's laptop.

### Deployment

| Environment | How |
|-------------|-----|
| **Local dev** | `make dev` -- Docker Compose with hot reload |
| **Kubernetes** | Helm chart with HPA, queue-depth autoscaling, ingress |
| **AWS** | Terraform module: EKS + RDS + ElastiCache |
| **GCP** | Terraform module: GKE + Cloud SQL + Memorystore |
| **Azure** | Terraform module: AKS + Azure Database + Redis |
| **Managed (free)** | [brockley.ai](https://brockley.ai) -- provisions in your cloud, you pick region |

### Autoscaling

Workers are stateless. All shared state lives in PostgreSQL and Redis. Scale workers horizontally based on queue depth -- each LLM call, MCP tool call, and code execution is a separate async task that any worker can pick up. Kubernetes HPA with custom queue-depth metrics handles this automatically.

### Observability

- **Metrics.** Prometheus endpoint at `/metrics` -- execution latency, node throughput, provider error rates, queue depth.
- **Logging.** Structured JSON with `execution_id` and `request_id` correlation on every entry.
- **Tracing.** OpenTelemetry export to Langfuse, Opik, Arize Phoenix, and LangSmith.
- **Health.** `/health` (liveness) and `/health/ready` (readiness) for Kubernetes probes.

All config via environment variables (12-factor). See [deployment docs](docs/deployment/) for the full guide.

## Architecture

```
                        ┌─────────────────────────────────────────────┐
                        │             Six Peer Interfaces             │
                        ├─────────┬────────┬─────┬───────┬─────┬─────┤
                        │ Web UI  │REST API│ CLI │Terraform│ MCP │Agent│
                        └────┬────┴───┬────┴──┬──┴───┬────┴──┬──┴──┬─┘
                             │        │       │      │       │     │
                             ▼        ▼       ▼      ▼       ▼     ▼
                        ┌─────────────────────────────────────────────┐
                        │              API Server (Go)                │
                        │   Routes · Auth · Validation · CRUD         │
                        └──────────────────┬──────────────────────────┘
                                           │
                    ┌──────────────────────┼──────────────────────┐
                    ▼                      ▼                      ▼
            ┌──────────────┐    ┌───────────────────┐    ┌──────────────┐
            │  PostgreSQL  │    │  Redis (asynq)    │    │  Workers     │
            │  Graphs,     │    │  Task queue,      │    │  LLM calls,  │
            │  executions, │    │  pub/sub for       │    │  MCP calls,  │
            │  state       │    │  streaming        │    │  code exec   │
            └──────────────┘    └───────────────────┘    └──────────────┘
```

## Build Graphs with Coding Agents

Brockley ships `coding-agent-skills/SKILL.md` -- a self-contained reference that gives Claude Code, Cursor, Copilot, or any coding agent everything it needs to write valid graph JSON.

```bash
# Add to Claude Code
cp coding-agent-skills/SKILL.md .claude/commands/brockley.md

# Then ask:
# "Build me a support ticket classifier that routes urgent issues
#  to an escalation agent and summarizes the rest"
```

Your coding agent produces valid, deployable graph JSON without guessing. See [`coding-agent-skills/README.md`](coding-agent-skills/README.md) for setup.

## Examples

| Example | What it shows | LLM needed? |
|---------|--------------|-------------|
| [`comprehensive`](examples/comprehensive/) | Transforms, conditionals, parallel fork/join, skip propagation | No |
| [`stateful-loop`](examples/stateful-loop/) | ForEach, subgraph, back-edges, state reducers | No |
| [`llm-pipeline`](examples/llm-pipeline/) | LLM classification, conditional routing, template rendering | Yes |
| [`superagent-simple`](examples/superagent-simple/) | Basic autonomous agent execution | E2E only |
| [`superagent-code-exec`](examples/superagent-code-exec/) | Superagent with Python code execution | E2E only |
| [`mcp-tools`](examples/mcp-tools/) | MCP tool chaining, conditional on tool output | E2E only |

## Documentation

- **[Getting Started](docs/getting-started/)** -- installation, quickstart, first graph
- **[Core Concepts](docs/concepts/)** -- graphs, nodes, typing, execution, state, superagent
- **[Node Reference](docs/nodes/)** -- every built-in node type
- **[Superagent Guide](docs/guides/superagent-tutorial.md)** -- building autonomous agents
- **[Expression Language](docs/expressions/)** -- templates, operators, filters
- **[LLM Providers](docs/providers/)** -- OpenAI, Anthropic, Google, OpenRouter, Bedrock
- **[CLI Reference](docs/cli/)** -- validate, deploy, invoke, inspect
- **[REST API](docs/api/)** -- graphs, executions, schemas, health
- **[Terraform Provider](docs/terraform/)** -- manage graphs as infrastructure
- **[Deployment](docs/deployment/)** -- Docker, Kubernetes, AWS, GCP, Azure
- **[CI/CD](docs/cicd/)** -- GitHub Actions, GitLab CI

## Contributing

We welcome contributions. See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
make test       # unit tests
make test-e2e   # E2E tests (requires Docker)
make lint       # linters
make build      # build binaries
```

## License

[Apache License 2.0](LICENSE)
