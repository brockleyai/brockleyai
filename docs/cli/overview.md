# CLI Overview

The Brockley CLI (`brockley`) lets you manage agent workflows from the command line. Validate graphs locally, invoke executions, inspect results, export to Terraform, and deploy graphs to your server.

## Installation

### From Source

```bash
go install github.com/brockleyai/brockleyai/cmd/brockley@latest
```

### Binary

Download the latest release from the [releases page](https://github.com/brockleyai/brockleyai/releases).

## Quick Start

```bash
# Configure connection
export BROCKLEY_SERVER_URL=http://localhost:8000
export BROCKLEY_API_KEY=your-api-key

# Or save to config
brockley auth set --server http://localhost:8000 --key your-api-key

# Validate a local graph
brockley validate -f my-graph.json

# Deploy to server
brockley deploy -f my-graph.json

# List graphs
brockley list graphs

# Invoke a graph
brockley invoke graph_abc123 --input '{"text": "hello"}' --sync
```

## Authentication

The CLI resolves credentials in this order:

1. CLI flags (`--server`, `--api-key`)
2. Environment variables (`BROCKLEY_SERVER_URL`, `BROCKLEY_API_KEY`)
3. Config file (`~/.brockley/config.json`)

## Output Formats

All commands support `-o json` for machine-readable output and `-o table` (default) for human-readable output.

## Commands

| Command | Description |
|---------|-------------|
| [`validate`](validate.md) | Validate graph structure |
| [`invoke`](invoke.md) | Invoke a graph execution |
| [`list`](list.md) | List resources |
| [`inspect`](inspect.md) | Inspect a resource in detail |
| [`export`](export.md) | Export graphs in different formats |
| [`deploy`](deploy.md) | Push graphs to the server |
| [`multi-file`](multi-file.md) | Working with multi-file graph definitions |

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--server` | | Brockley server URL (overrides `BROCKLEY_SERVER_URL`) |
| `--api-key` | | API key (overrides `BROCKLEY_API_KEY`) |
| `--output` | `-o` | Output format: `json` or `table` (default: `table`) |

## See Also

- [GitHub Actions](../cicd/github-actions.md) -- using the CLI in CI/CD
- [Generic CI](../cicd/generic-ci.md) -- installing the CLI binary in CI
- [Multi-File Graphs](multi-file.md) -- directory convention and file merging
