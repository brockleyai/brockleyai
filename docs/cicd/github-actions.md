# GitHub Actions

Use the Brockley GitHub Action to validate and deploy agent graphs in your CI/CD pipeline.

## Quick Start

```yaml
name: Brockley CI/CD

on:
  push:
    branches: [main]
  pull_request:

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: brockleyai/brockley-action@v1
        with:
          command: validate
          path: graphs/

  deploy:
    runs-on: ubuntu-latest
    needs: validate
    if: github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v4
      - uses: brockleyai/brockley-action@v1
        with:
          command: deploy
          path: graphs/
          server-url: ${{ secrets.BROCKLEY_SERVER_URL }}
          api-key: ${{ secrets.BROCKLEY_API_KEY }}
```

## Inputs

| Input | Required | Description |
|-------|----------|-------------|
| `command` | Yes | `validate` or `deploy` |
| `path` | Yes | Path to graph file or directory |
| `server-url` | For deploy | Brockley server URL |
| `api-key` | For deploy | API key |
| `namespace` | No | Override namespace |
| `version` | No | CLI version (default: latest) |

## Validate Command

Validates graph structure locally using the built-in engine. No server connection required. Exits with code 1 if any graph has errors.

```yaml
- uses: brockleyai/brockley-action@v1
  with:
    command: validate
    path: graphs/
```

## Deploy Command

Pushes graph definitions to the server. Creates new graphs or updates existing ones (matched by name).

```yaml
- uses: brockleyai/brockley-action@v1
  with:
    command: deploy
    path: graphs/
    server-url: ${{ secrets.BROCKLEY_SERVER_URL }}
    api-key: ${{ secrets.BROCKLEY_API_KEY }}
```

## Environment Variables via Secrets

Pass LLM API keys to graph deploy using GitHub secrets. Any `${VAR}` in graph JSON files is resolved from the runner environment:

```yaml
- uses: brockleyai/brockley-action@v1
  with:
    command: deploy
    path: graphs/
    server-url: ${{ secrets.BROCKLEY_SERVER_URL }}
    api-key: ${{ secrets.BROCKLEY_API_KEY }}
  env:
    OPENROUTER_API_KEY: ${{ secrets.OPENROUTER_API_KEY }}
```

## Secrets Setup

1. Go to your repository Settings > Secrets and variables > Actions
2. Add `BROCKLEY_SERVER_URL` (e.g., `https://brockley.yourcompany.com`)
3. Add `BROCKLEY_API_KEY` (your server API key)
4. Add any LLM provider keys referenced by `${VAR}` in your graph files

## See Also

- [CLI Overview](../cli/overview.md) -- CLI installation and usage
- [validate command](../cli/validate.md) -- local validation details
- [deploy command](../cli/deploy.md) -- environment variable expansion
- [GitLab CI](gitlab-ci.md) -- GitLab pipeline setup
- [Generic CI](generic-ci.md) -- any CI system
