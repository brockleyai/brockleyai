# brockley invoke

Invoke a graph execution on the server.

## Usage

```bash
brockley invoke <graph_id> [flags]
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--input` | `-i` | JSON input (inline string) |
| `--input-file` | `-f` | JSON input file |
| `--sync` | | Wait for execution to complete (blocks until done) |
| `--stream` | | Stream execution events to stdout (SSE) |
| `--poll` | | Poll for completion (async with polling) |
| `--timeout` | | Timeout in seconds |
| `--output` | `-o` | Output format: `json` or `table` |

## Modes

### Async (default)

Returns immediately with the execution ID:

```bash
brockley invoke graph_abc123 --input '{"text": "hello"}'
```

### Sync

Blocks until the execution completes:

```bash
brockley invoke graph_abc123 --input '{"text": "hello"}' --sync
```

### Stream

Streams execution events (SSE) in real time:

```bash
brockley invoke graph_abc123 --input '{"text": "hello"}' --stream
```

### Poll

Starts async, then polls until completion:

```bash
brockley invoke graph_abc123 --input '{"text": "hello"}' --poll
```

## Examples

```bash
# Inline input
brockley invoke graph_abc123 -i '{"ticket": "billing issue"}'

# Input from file
brockley invoke graph_abc123 -f input.json --sync

# With timeout
brockley invoke graph_abc123 -i '{"x": 1}' --sync --timeout 60

# JSON output
brockley invoke graph_abc123 -i '{}' --sync -o json

# Stream events
brockley invoke graph_abc123 -i '{"text": "hello"}' --stream
```

## See Also

- [CLI Overview](overview.md) -- installation, global flags
- [Executions API](../api/executions.md) -- the underlying API
- [inspect command](inspect.md) -- inspect execution results after completion
