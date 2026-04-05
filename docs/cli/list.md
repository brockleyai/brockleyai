# brockley list

List resources on the server.

## Usage

```bash
brockley list <resource> [flags]
```

## Resources

- `graphs` (aliases: `graph`, `g`)
- `executions` (aliases: `execution`, `exec`, `e`)
- `schemas` (aliases: `schema`, `s`)
- `prompt-templates` (aliases: `prompts`, `pt`)
- `provider-configs` (aliases: `providers`, `pc`)

## Common Flags

| Flag | Description |
|------|-------------|
| `--namespace` | Filter by namespace |
| `--limit` | Max items to return (default: 20) |
| `--output` / `-o` | Output format: `json` or `table` |

## Resource-Specific Flags

### graphs

| Flag | Description |
|------|-------------|
| `--status` | Filter by status (`draft`, `active`, `archived`) |

### executions

| Flag | Description |
|------|-------------|
| `--graph-id` | Filter by graph ID |
| `--status` | Filter by status |

## Examples

```bash
brockley list graphs
brockley list graphs --status active --namespace production
brockley list executions --graph-id graph_abc123
brockley list schemas -o json
brockley list prompt-templates
brockley list provider-configs --namespace production
```

## See Also

- [CLI Overview](overview.md) -- installation, global flags
- [inspect command](inspect.md) -- detailed view of a single resource
- [Graphs API](../api/graphs.md) -- the list endpoint
