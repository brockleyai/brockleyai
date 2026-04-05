# Brockley CLI

Command-line interface for managing Brockley agent workflows.

## Installation

```bash
go install github.com/brockleyai/brockleyai/cmd/brockley@latest
```

Or build from source:

```bash
go build -o brockley ./cmd/brockley
```

## Authentication

### Environment Variables

```bash
export BROCKLEY_SERVER_URL=http://localhost:8000
export BROCKLEY_API_KEY=your-api-key
```

### Config File

```bash
brockley auth set --server http://localhost:8000 --key your-api-key
```

Config is stored in `~/.brockley/config.json`.

### CLI Flags

```bash
brockley --server http://localhost:8000 --api-key your-key list graphs
```

Priority: CLI flags > environment variables > config file.

## Commands

### validate

Validate graph structure locally (no server required) or remotely.

```bash
# Local validation (offline)
brockley validate -f graph.json
brockley validate -d graphs/

# Remote validation (via API)
brockley validate graph_abc123
```

### invoke

Invoke a graph execution.

```bash
brockley invoke graph_abc123 --input '{"text": "hello"}'
brockley invoke graph_abc123 -f input.json --sync
brockley invoke graph_abc123 --input '{"x": 1}' --poll
```

### list

List resources.

```bash
brockley list graphs
brockley list graphs --status active --namespace production
brockley list executions --graph-id graph_abc123
brockley list schemas
brockley list prompt-templates
brockley list provider-configs
```

### inspect

Inspect a specific resource.

```bash
brockley inspect graph graph_abc123
brockley inspect execution exec_xyz789 --steps
brockley inspect schema schema_001
```

### export

Export a graph in different formats.

```bash
brockley export graph_abc123
brockley export graph_abc123 --format terraform --out main.tf
brockley export -f graph.json --format yaml
```

### deploy

Push graph definitions to the server.

```bash
brockley deploy -f graph.json
brockley deploy -d graphs/
brockley deploy -f graph1.json -f graph2.json --namespace production
```

### auth

Manage authentication.

```bash
brockley auth set --server http://localhost:8000 --key your-key
brockley auth show
brockley auth test
```

## Output Formats

All commands support `--output` / `-o` flag:

- `table` (default) -- human-readable tabular output
- `json` -- machine-readable JSON

```bash
brockley list graphs -o json
brockley inspect graph graph_abc123 -o json
```

## Multi-file Graph Composition

Use `-f` and `-d` flags to work with multiple graph files:

```bash
# Validate all graphs in a directory
brockley validate -d graphs/

# Deploy multiple files
brockley deploy -f agents/classifier.json -f agents/responder.json

# Deploy entire directory
brockley deploy -d agents/
```

## CI/CD Usage

The CLI is designed for CI/CD pipelines:

```bash
# Validate (no server needed -- exit code 1 on failure)
brockley validate -d graphs/

# Deploy (requires server)
brockley deploy -d graphs/ --server $BROCKLEY_URL --api-key $BROCKLEY_KEY
```
