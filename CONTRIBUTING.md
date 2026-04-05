# Contributing to Brockley

Thank you for your interest in contributing to Brockley. This guide covers everything you need to get started.

## Prerequisites

- Go 1.24+
- Docker and Docker Compose
- Node.js 20+
- Make

## Setup

```bash
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai
make dev
```

This starts all dependencies (PostgreSQL, Redis) and runs the server with hot-reload.

## Testing

Run the full test suite (no external dependencies required):

```bash
make test
```

Run tests with coverage:

```bash
make test-coverage
```

Coverage requirements:

- Engine package: >90% line coverage
- All other packages: >80% line coverage

## Code Style

All Go code must pass:

```bash
gofmt -s -w .
golangci-lint run
```

Both checks run automatically in CI on every pull request.

## Testing Requirements

Follow the interface-driven testing pattern:

1. Define the interface
2. Write a mock implementation
3. Write tests against the mock
4. Implement the real code
5. Verify tests pass against both mock and real implementations

This keeps tests fast, deterministic, and infrastructure-free.

## Pull Request Process

1. Branch from `main`
2. Make your changes in small, focused commits
3. Write or update tests for every change
4. Update documentation if your change affects user-facing behavior
5. Run `make test` and `golangci-lint run` locally
6. Submit a pull request against `main`

A maintainer will review your PR. Please respond to feedback promptly.

## Commit Messages

Write commit messages in imperative mood, keep the subject line concise (under 72 characters), and add a body when context is needed.

Good:

```
Add foreach node concurrency limit

The foreach node previously spawned unlimited goroutines. Add a
configurable max_concurrency field that defaults to 10.
```

Bad:

```
fixed the thing
```

## Where to Get Help

- [GitHub Discussions](https://github.com/brockleyai/brockleyai/discussions) -- Q&A, design discussions, and maintainer coordination
- [GitHub Issues](https://github.com/brockleyai/brockleyai/issues) -- bug reports and feature requests

## Reporting Issues

Use the GitHub issue templates for bug reports and feature requests. Include enough detail for someone unfamiliar with your setup to reproduce the problem or understand the request.

## Good First Issues

Look for issues labeled [`good first issue`](https://github.com/brockleyai/brockleyai/labels/good%20first%20issue) -- these are well-scoped, documented, and accessible to new contributors.

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.
