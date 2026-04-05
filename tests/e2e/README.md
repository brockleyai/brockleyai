# E2E Tests

End-to-end tests that exercise Brockley via the CLI and Terraform provider against a live stack.

## Prerequisites

- Docker + Docker Compose v2+
- Go 1.24+
- Terraform 1.0+ (for TF tests)
- jq 1.6+
- bash 4+
- `OPENROUTER_API_KEY` in `.env` or environment (free at [openrouter.ai](https://openrouter.ai/keys))

## Quick Start

```bash
# Run all E2E tests (requires OPENROUTER_API_KEY)
make test-e2e

# Run only LLM E2E tests and print LLM request/response traces
make test-e2e-llm-verbose

# Run only CLI tests
make test-e2e-cli-only

# Run only Terraform tests
make test-e2e-tf-only

# Run a single graph
ONLY=llm-pipeline make test-e2e

# Skip LLM graphs (CI without API key)
make test-e2e-no-llm

# Skip MCP graphs
make test-e2e-no-mcp

# Skip both LLM and MCP graphs (deterministic, no external services)
make test-e2e-no-external
```

## How It Works

1. `run.sh` builds the CLI binary and Terraform provider from source
2. Starts an isolated Docker Compose stack (port 18000) with PostgreSQL, Redis, server, worker, and MCP test server
3. For each test manifest in `manifests/`:
   - **CLI path**: deploy graph via CLI, validate, invoke (sync), assert, cleanup
   - **TF path**: generate Terraform config, init/plan/apply, invoke via API, assert, destroy
4. Tears down the Docker stack on exit (even on failure or Ctrl-C)

## Test Graphs

| Graph | Purpose | Requires LLM | Requires MCP |
|-------|---------|:---:|:---:|
| `comprehensive` | Transforms, conditionals, parallel fork/join, skip propagation, expression coverage | No | No |
| `stateful-loop` | ForEach, subgraph, back-edge loops, state reducers | No | No |
| `llm-pipeline` | LLM classification, conditional routing, template rendering | Yes | No |
| `mcp-tools` | Tool nodes via MCP, tool chaining, conditional on tool output, forEach with tools, error handling | No | Yes |
| `api-tool-standalone` | Standalone api_tool node calling a REST endpoint via referenced API tool definition | No | No |
| `superagent-*` | Distributed superagent execution with a real LLM, built-in task/buffer tools, and MCP tool calls | Yes | Yes |
| `superagent-retrieve` | Real-LLM superagent retrieves an opaque pre-seeded secret from MCP, proving the model chose a real tool call | Yes | Yes |

## MCP Test Server

The `mcp-server/` directory contains a standalone Go HTTP server that implements a minimal MCP (JSON-RPC 2.0) endpoint for E2E testing. It provides deterministic tools including:

| Tool | Description |
|------|-------------|
| `echo` | Returns the message as-is. Prepends `X-Test-Header` value if present. |
| `word_count` | Counts words in the text, returns count as a string. |
| `lookup` | Maps keys to ordinal values (`alpha`=`first`, `beta`=`second`, etc.). Returns error for unknown keys. |
| `store_value` / `retrieve_value` | Stores and retrieves opaque values in MCP-server memory for multi-step and superagent tests. |

The server runs on port 9090 inside the Docker network and is automatically started as part of the E2E stack.

## Directory Structure

```
tests/e2e/
├── run.sh                    # Main entrypoint
├── docker-compose.e2e.yml    # Isolated stack
├── lib/
│   ├── common.sh             # Assertions, logging
│   ├── stack.sh              # Docker lifecycle
│   ├── cli_test.sh           # CLI test flow
│   └── tf_test.sh            # Terraform test flow
├── api-tools/                # API tool definitions for E2E tests
│   └── test-api.json         # Test API definition (customers, charges, search)
├── graphs/
│   ├── api-tool-standalone/graph.json
│   ├── comprehensive/graph.json
│   ├── stateful-loop/graph.json
│   ├── llm-pipeline/graph.json
│   ├── mcp-tools/graph.json
│   └── superagent-*/graph.json
├── manifests/
│   ├── api-tool-standalone.json
│   ├── comprehensive-premium-high.json
│   ├── comprehensive-premium-low.json
│   ├── comprehensive-standard.json
│   ├── stateful-loop.json
│   ├── llm-pipeline.json
│   ├── mcp-tools-short.json
│   ├── mcp-tools-long.json
│   └── mcp-tools-error.json
└── mcp-server/               # Test MCP server
    ├── main.go               # HTTP server + JSON-RPC dispatcher
    ├── tools.go              # Tool implementations
    ├── go.mod                # Standalone module (stdlib only)
    └── Dockerfile            # Multi-stage build
```

## Options

| Flag | Description |
|------|-------------|
| `--env-file <path>` | Load env vars from file (default: auto-detects `.env`) |
| `--no-llm` | Skip LLM graphs |
| `--no-mcp` | Skip MCP graphs |
| `--cli-only` | CLI tests only |
| `--tf-only` | Terraform tests only |
| `--no-stack` | Don't manage Docker stack (assume running) |
| `--llm-verbose` | Run only `requires_llm` manifests, invoke executions with `debug: true`, and print only LLM request/response traces plus pass/fail |

| Env Var | Description |
|---------|-------------|
| `ONLY=<graph>` | Run only the named graph |
| `E2E_PORT=<port>` | Server port (default: 18000) |
| `OPENROUTER_API_KEY` | Required for LLM graph tests |

## LLM Verbose Mode

`make test-e2e-llm-verbose` enables a debug-only execution mode for E2E tests. In this mode:

- Only manifests with `requires_llm: true` are run
- Each execution is invoked with `debug: true`
- Raw LLM request/response payloads are persisted to PostgreSQL on `execution_steps.llm_debug`
- The runner fetches `/api/v1/executions/{id}/steps` after completion and prints only pass/fail plus per-node LLM request/response blocks

Redis remains transient transport for execution coordination; the durable source for `llm-verbose` output is PostgreSQL execution-step storage.

## Adding Tests

To add coverage for a new feature:

1. If it fits in an existing graph, add nodes/edges to that graph's `graph.json`
2. Add or update manifests in `manifests/` with appropriate assertions
3. Only create a new graph if the feature truly doesn't fit existing ones
