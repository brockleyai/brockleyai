# brockley inspect

Fetch and display a resource in detail.

## Usage

```bash
brockley inspect <resource> <id> [flags]
```

## Resources

- `graph` (alias: `g`)
- `execution` (aliases: `exec`, `e`)
- `schema` (alias: `s`)
- `prompt-template` (alias: `pt`)
- `provider-config` (alias: `pc`)

## Flags

### execution

| Flag | Description |
|------|-------------|
| `--steps` | Include execution steps |

## Examples

```bash
brockley inspect graph graph_abc123
brockley inspect execution exec_xyz789 --steps
brockley inspect schema schema_001 -o json

# Inspect a prompt template
brockley inspect prompt-template pt_001

# Inspect a provider config
brockley inspect provider-config pc_001
```

## See Also

- [CLI Overview](overview.md) -- installation, global flags
- [list command](list.md) -- list resources before inspecting
- [Executions API](../api/executions.md) -- execution details via API
