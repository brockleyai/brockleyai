# Multi-File Graph Composition

The Brockley CLI supports working with multiple graph files using `-f` and `-d` flags.

## File Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-f` | One or more specific graph JSON files |
| `--dir` | `-d` | One or more directories containing `.json` files |

These flags can be combined and repeated:

```bash
brockley validate -f graph1.json -f graph2.json -d more-graphs/
```

## Directory Scanning

When using `-d`, the CLI scans the directory for all `.json` files (non-recursive). Non-JSON files are ignored.

## Supported Commands

| Command | `-f` | `-d` |
|---------|------|------|
| `validate` | Yes | Yes |
| `deploy` | Yes | Yes |
| `export` | Yes (single file) | No |

## Project Structure Example

```
my-project/
  graphs/
    classifier.json
    responder.json
    router.json
```

```bash
# Validate all
brockley validate -d graphs/

# Deploy all
brockley deploy -d graphs/
```

## Merging Behavior

When multiple files are provided, the CLI merges them into a single graph object. Files can split a graph across concerns:

- `graph.yaml` -- name, description, metadata, state
- `nodes/*.yaml` -- node definitions (one per file or multiple)
- `edges.yaml` -- all edges

The CLI performs a shallow merge: `nodes` arrays are concatenated, `edges` arrays are concatenated, and top-level fields (`name`, `description`, `state`) take the last value seen.

## See Also

- [CLI Overview](overview.md) -- installation, global flags
- [validate command](validate.md) -- validating multi-file graphs
- [deploy command](deploy.md) -- deploying multi-file graphs
- [Data Model](../specs/data-model.md) -- multi-file graph definitions
