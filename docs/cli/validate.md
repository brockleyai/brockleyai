# brockley validate

Validate a graph definition for structural and type errors.

## Usage

```bash
# Local validation (no server required)
brockley validate -f graph.json
brockley validate -d graphs/
brockley validate -f graph1.json -f graph2.json

# Remote validation (via server API)
brockley validate <graph_id>
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-f` | Graph JSON file(s) to validate locally |
| `--dir` | `-d` | Directory(ies) containing graph JSON files |
| `--output` | `-o` | Output format: `json` or `table` |

## Local vs Remote Validation

**Local validation** uses the built-in graph validation engine. No server is required. This is ideal for CI/CD pipelines and pre-commit checks.

**Remote validation** calls `POST /api/v1/graphs/{id}/validate` on the server. This validates a graph that already exists on the server.

## Exit Codes

- `0` -- All graphs are valid
- `1` -- One or more graphs have errors

## Examples

```bash
# Validate all graphs in a directory
brockley validate -d agents/

# JSON output for CI parsing
brockley validate -f my-graph.json -o json

# Validate a server-side graph
brockley validate graph_abc123
```

## GitHub Actions Example

```yaml
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: brockleyai/brockley-action@v1
        with:
          command: validate
          path: graphs/
```

## See Also

- [CLI Overview](overview.md) -- installation, global flags
- [Graphs](../concepts/graphs.md) -- graph structure and validation rules
- [GitHub Actions](../cicd/github-actions.md) -- CI/CD pipeline setup
- [Common Errors](../troubleshooting/common-errors.md) -- validation error codes
