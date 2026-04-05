# brockley deploy

Push graph definitions from local files to the Brockley server.

## Usage

```bash
brockley deploy -f <file.json> [flags]
brockley deploy -d <directory/> [flags]
```

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--file` | `-f` | Graph JSON file(s) |
| `--dir` | `-d` | Directory(ies) containing graph JSON files |
| `--namespace` | | Override namespace for all graphs |
| `--env-file` | `-e` | Load environment variables from file (e.g. `.env`) |

## Environment Variable Expansion

Graph JSON files can use `${VAR_NAME}` placeholders. The CLI resolves them from the current environment before pushing to the server. This keeps secrets out of graph definitions.

```json
"config": {
  "provider": "openrouter",
  "api_key": "${OPENROUTER_API_KEY}"
}
```

Variables are resolved from:
1. The current shell environment (`export VAR=value`)
2. A `.env` file loaded via `--env-file`

Shell environment takes priority -- `--env-file` does not overwrite existing variables. The CLI warns if any `${...}` placeholders reference unset variables.

## Behavior

- **Create**: If no graph with the same `name` exists on the server, a new graph is created.
- **Update**: If a graph with the same `name` already exists, it is updated (version auto-incremented).

Graphs are matched by `name`, not by `id`.

## Examples

```bash
# Deploy a single graph
brockley deploy -f my-graph.json

# Deploy all graphs in a directory
brockley deploy -d graphs/

# Deploy with namespace override
brockley deploy -d graphs/ --namespace production

# Deploy multiple specific files
brockley deploy -f agents/classifier.json -f agents/responder.json
```

## Using Environment Variables

```bash
# Set your API key in the shell
export OPENROUTER_API_KEY="sk-or-v1-..."
brockley deploy -d examples/

# Or use a .env file
brockley deploy -d examples/ --env-file .env

# Both work together -- shell vars take priority over .env
```

## CI/CD Usage

```bash
# In a GitHub Actions workflow -- secrets come from CI environment
brockley deploy -d graphs/ \
  --server $BROCKLEY_SERVER_URL \
  --api-key $BROCKLEY_API_KEY
```

The `${OPENROUTER_API_KEY}` (or any `${VAR}`) in graph files is resolved from the CI environment automatically.

## See Also

- [CLI Overview](overview.md) -- installation, global flags
- [Multi-File Graphs](multi-file.md) -- directory convention
- [GitHub Actions](../cicd/github-actions.md) -- CI/CD deployment workflow
- [Configuration Reference](../deployment/configuration.md) -- server environment variables
