# Generic CI/CD

Use the Brockley CLI in any CI/CD system.

## Installation

```bash
# Install via Go
go install github.com/brockleyai/brockleyai/cmd/brockley@latest

# Or download binary
curl -L https://github.com/brockleyai/brockleyai/releases/latest/download/brockley-linux-amd64 -o /usr/local/bin/brockley
chmod +x /usr/local/bin/brockley
```

## Validate (No Server Required)

```bash
brockley validate -d graphs/
# Exit code 0 = all valid, 1 = errors found
```

## Deploy

```bash
export BROCKLEY_SERVER_URL=https://brockley.yourcompany.com
export BROCKLEY_API_KEY=your-api-key

brockley deploy -d graphs/ --namespace production
```

## CI/CD Pattern

```bash
#!/bin/bash
set -e

# 1. Validate locally (fast, no server)
brockley validate -d graphs/

# 2. Deploy to server (only on main branch)
if [ "$CI_BRANCH" = "main" ]; then
  brockley deploy -d graphs/ --namespace production
fi
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `BROCKLEY_SERVER_URL` | Server URL |
| `BROCKLEY_API_KEY` | API key |

## Binary Download (No Go Required)

For CI environments without Go, download the prebuilt binary:

```bash
# Linux amd64
curl -L https://github.com/brockleyai/brockleyai/releases/latest/download/brockley-linux-amd64 \
  -o /usr/local/bin/brockley
chmod +x /usr/local/bin/brockley

# Linux arm64
curl -L https://github.com/brockleyai/brockleyai/releases/latest/download/brockley-linux-arm64 \
  -o /usr/local/bin/brockley
chmod +x /usr/local/bin/brockley

# macOS amd64
curl -L https://github.com/brockleyai/brockleyai/releases/latest/download/brockley-darwin-amd64 \
  -o /usr/local/bin/brockley
chmod +x /usr/local/bin/brockley
```

## Docker

```dockerfile
FROM golang:1.24 AS builder
RUN go install github.com/brockleyai/brockleyai/cmd/brockley@latest

FROM alpine:latest
COPY --from=builder /go/bin/brockley /usr/local/bin/brockley
ENTRYPOINT ["brockley"]
```

## See Also

- [CLI Overview](../cli/overview.md) -- CLI installation and usage
- [GitHub Actions](github-actions.md) -- GitHub-specific setup
- [GitLab CI](gitlab-ci.md) -- GitLab-specific setup
- [deploy command](../cli/deploy.md) -- environment variable expansion
