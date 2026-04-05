# GitLab CI

Use the Brockley CLI directly in GitLab CI pipelines.

## Example `.gitlab-ci.yml`

```yaml
stages:
  - validate
  - deploy

variables:
  GO_VERSION: "1.24"

validate-graphs:
  stage: validate
  image: golang:${GO_VERSION}
  script:
    - go install github.com/brockleyai/brockleyai/cmd/brockley@latest
    - brockley validate -d graphs/
  only:
    changes:
      - graphs/**

deploy-graphs:
  stage: deploy
  image: golang:${GO_VERSION}
  script:
    - go install github.com/brockleyai/brockleyai/cmd/brockley@latest
    - brockley deploy -d graphs/ --namespace production
  variables:
    BROCKLEY_SERVER_URL: $BROCKLEY_SERVER_URL
    BROCKLEY_API_KEY: $BROCKLEY_API_KEY
  only:
    refs:
      - main
    changes:
      - graphs/**
```

## Using the Binary Instead of Go

If you do not want a Go toolchain in your CI image, download the prebuilt binary:

```yaml
validate-graphs:
  stage: validate
  image: alpine:latest
  script:
    - apk add --no-cache curl
    - curl -L https://github.com/brockleyai/brockleyai/releases/latest/download/brockley-linux-amd64 -o /usr/local/bin/brockley
    - chmod +x /usr/local/bin/brockley
    - brockley validate -d graphs/
```

## Variables Setup

1. Go to Settings > CI/CD > Variables
2. Add `BROCKLEY_SERVER_URL` (protected, not masked)
3. Add `BROCKLEY_API_KEY` (protected, masked)

## See Also

- [CLI Overview](../cli/overview.md) -- CLI installation and usage
- [GitHub Actions](github-actions.md) -- GitHub CI setup
- [Generic CI](generic-ci.md) -- any CI system
