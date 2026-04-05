# Contributing to Brockley

Thank you for your interest in contributing to Brockley. This guide covers what you need to know to get started.

---

## Project Overview

Brockley is an open-source, self-hostable platform for defining, validating, executing, and operationalizing AI agent workflows. It is written primarily in Go (engine, server, CLI, workers) with a React/TypeScript web UI.

Before contributing, familiarize yourself with the architecture by reading the specs:

- **[architecture.md](../specs/architecture.md)** -- System overview: components, data flow, deployment
- **[data-model.md](../specs/data-model.md)** -- Core entities, fields, relationships, validation rules
- **[expression-language.md](../specs/expression-language.md)** -- Expression language specification
- **[graph-model.md](../specs/graph-model.md)** -- Graph execution model: ports, state, branching, loops
- **[api-design.md](../specs/api-design.md)** -- REST API endpoints and conventions

---

## Getting Started

### Prerequisites

- Go 1.22+
- Node.js 20+ and pnpm (for the web UI)
- Docker and Docker Compose (for local development)
- PostgreSQL 15+ and Redis 7+ (or use Docker Compose)

### Local Development Setup

```bash
# Clone the repository
git clone https://github.com/brockleyai/brockleyai.git
cd brockleyai

# Start infrastructure
docker compose up -d postgres redis

# Run the API server
go run ./cmd/server

# Run a worker
go run ./cmd/worker

# Run the web UI (separate terminal)
cd web-ui
pnpm install
pnpm dev
```

---

## Architecture Decisions for Contributors

### Engine Independence

The execution engine (`engine/` package) is a standalone Go library. It must not depend on the API server, database, or worker infrastructure. If you are adding engine features, ensure they work without any infrastructure -- tests should run with `go test` alone, no Docker required.

### Schema-First Validation

All data flowing through graphs is validated against JSON Schema. When adding new node types or modifying existing ones, define schemas for all input and output ports. Use strong typing rules: no bare `{ "type": "object" }` or `{ "type": "array" }`.

### Interface Parity

Brockley exposes four interfaces: Web UI, Terraform provider, CLI, and MCP server. When adding a new capability to the API, consider which interfaces need updates.

### Extensibility Patterns

- **New node types**: Implement the `NodeExecutor` interface and register in the node type definitions
- **New LLM providers**: Implement the `LLMProvider` interface
- **New MCP tool integrations**: No code changes needed -- MCP tools are discovered dynamically

---

## Coding Conventions

### Go

- Format with `gofmt` (enforced in CI)
- Lint with `golangci-lint`
- Use `slog` for structured logging (JSON output). No `fmt.Println` or unstructured output.
- All external dependencies behind interfaces (LLM providers, MCP servers, database, Redis)
- Field names in structs use `PascalCase`; JSON tags use `snake_case`
- Error handling: return errors, do not panic. Use structured error types with error codes.
- Context propagation: pass `context.Context` as the first parameter

### TypeScript / React (Web UI)

- Format with Prettier
- Lint with ESLint
- State management with Zustand
- Styling with Tailwind CSS
- Component library: shadcn/ui

---

## Testing Strategy

### Requirements

- **No code without tests.** Every change that modifies Go code includes tests.
- **Tests run without external services.** `go test ./...` must pass with zero infrastructure. Only E2E tests use Docker.
- **Table-driven tests** for multi-case functions. Use `testdata/` directories for fixtures.

### Test Layers

1. **Unit tests** -- Test individual functions and types. Mock all external dependencies. These are the most common.
2. **Integration tests** -- Test component interactions (e.g., engine + mock LLM provider). No external services.
3. **E2E tests** -- Test full workflows through the API. Require Docker infrastructure. Run with `make test-e2e`.

### Mocking

All external dependencies are behind Go interfaces. Use the mock implementations in `internal/mocks/` (or create new ones). The mock LLM provider is the primary integration testing tool -- it returns deterministic responses for testing graph execution without real LLM calls.

### Running Tests

```bash
# Run all unit and integration tests
go test ./... -count=1

# Run tests for a specific package
go test ./engine/... -count=1

# Run E2E tests (requires Docker)
make test-e2e

# Run linter
golangci-lint run ./...

# Format all Go files
find . -name "*.go" -not -path "./.git/*" -exec gofmt -w {} \;
```

---

## CI Checks

All of the following must pass before a PR is merged:

```bash
# 1. Format
find . -name "*.go" -not -path "./.git/*" -exec gofmt -w {} \;

# 2. Lint (must return 0 issues)
golangci-lint run ./...

# 3. Build
go build ./...

# 4. Test
go test ./... -count=1
```

---

## Making a Contribution

### Workflow

1. Fork the repository and create a feature branch
2. Make your changes in small, reviewable increments
3. Write tests for all new/changed behavior
4. Add structured logging for significant operations
5. Run CI checks locally (see above)
6. Submit a pull request with a clear description of what and why

### What Makes a Good PR

- **Focused**: One logical change per PR
- **Tested**: Includes unit tests; integration/E2E tests where appropriate
- **Documented**: Updates user-facing docs in `docs/` if behavior changed
- **Observable**: Adds `slog` logging for new operations with correlation IDs

### Areas Where Help Is Welcome

- New LLM provider implementations
- CLI improvements and new commands
- Web UI features and polish
- Documentation improvements
- Bug fixes with reproducible test cases
- Performance improvements with benchmarks

---

## Logging

All components use `slog` with JSON output. When adding new features:

- Log every significant operation (node start/complete/fail, provider calls, state changes)
- Include correlation IDs (`execution_id`, `request_id`) on every log entry
- Never log secrets -- log `api_key_ref` names, not values
- Receive loggers via constructors or context; never create loggers internally

---

## License

Brockley is licensed under Apache 2.0. By contributing, you agree that your contributions will be licensed under the same license.

## See Also

- [Architecture](../specs/architecture.md) -- system overview and component design
- [Data Model](../specs/data-model.md) -- entity definitions and validation rules
- [API Design](../specs/api-design.md) -- REST API conventions
- [Local Development](../deployment/local-dev.md) -- Docker Compose setup
- [Configuration Reference](../deployment/configuration.md) -- environment variables
