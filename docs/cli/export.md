# brockley export

Export a graph definition in different formats.

## Usage

```bash
brockley export <graph_id> [flags]
brockley export -f <file.json> [flags]
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--format` | | Export format: `json`, `yaml`, `terraform` (default: `json`) |
| `--out` | | Output file (default: stdout) |
| `--file` | `-f` | Local graph JSON file to export |

## Formats

### JSON (default)

Pretty-printed JSON graph definition.

### YAML

Human-readable YAML representation of the graph.

### Terraform

Generates a `brockley_graph` Terraform resource block for use with the Brockley Terraform provider.

## Examples

```bash
# Export server-side graph as JSON
brockley export graph_abc123

# Export as Terraform HCL
brockley export graph_abc123 --format terraform --out main.tf

# Export local file as YAML
brockley export -f my-graph.json --format yaml

# Export local file as Terraform
brockley export -f my-graph.json --format terraform
```

## See Also

- [CLI Overview](overview.md) -- installation, global flags
- [Terraform Provider](../terraform/overview.md) -- using exported Terraform HCL
- [Graphs API](../api/graphs.md) -- the export endpoint
