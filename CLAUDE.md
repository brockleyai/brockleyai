# CLAUDE.md

## Purpose

This file is the living operating manual for how to work in this repository. It must be maintained as the project evolves.

---

## Mission

Build Brockley as an open-source, self-hostable AI agent infrastructure platform.

**What it does:** Define, validate, execute, and operationalize agent workflows via a visual web UI, REST API, CLI, Terraform provider, and MCP server.

---

## Resolved Technical Decisions

All major decisions are closed.

| Area | Decision |
|------|----------|
| Backend language | **Go** (engine, server, CLI, workers) |
| Database | **PostgreSQL** with JSONB |
| Task queue | **asynq** + Redis |
| Auth | **API keys** only; no auth in dev mode |
| Multi-tenancy | **`tenant_id` on all entities**; OSS uses `"default"` |
| LLM providers | **Own `LLMProvider` interface**; OpenAI, Anthropic, Google, OpenRouter, Bedrock |
| Execution model | **Distributed** -- orchestrator (`graph:start`) dispatches `node:llm-call`, `node:mcp-call`, `node:run`, `node:superagent`, `node:code-exec` as separate asynq tasks; superagent coordinator stays alive and dispatches its own LLM/MCP/code-exec calls as tasks; pure-computation nodes (transform, conditional) run in-process |
| License | **Apache 2.0** |
| Graph model | **Typed ports + state reducers + back-edges** |
| Branching | **Conditional nodes + exclusive fan-in + foreach + skip propagation** |
| Streaming | **Redis pub/sub** for real-time; PostgreSQL writes fire-and-forget |

### Library Choices

| Library | Purpose |
|---------|---------|
| `net/http.ServeMux` | HTTP router (Go 1.22+ stdlib) |
| `pgx` | PostgreSQL driver |
| `hibiken/asynq` | Redis-based task queue |
| `santhosh-tekuri/jsonschema/v6` | JSON Schema validation (LLM output validation) |
| `cobra` | CLI framework |
| Python 3 (embedded) | Coderunner subprocess for superagent code execution |
| `slog` (stdlib) | Structured logging |
| Custom OTLP exporter | Trace export (Langfuse/Opik/Phoenix/LangSmith) |
| React 18 + TypeScript | Web UI |
| Vite | Web UI build |
| Tailwind CSS | Web UI styling |
| Shadcn/ui | Web UI component library (Tailwind-native) |
| Zustand | Web UI state management |
| React Flow | Web UI graph editor |
| Terraform Plugin Framework | Terraform provider |

---

## Source of Truth

| Document | Path |
|----------|------|
| Architecture overview | `docs/specs/architecture.md` |
| Data model | `docs/specs/data-model.md` |
| Expression language | `docs/specs/expression-language.md` |
| Graph model | `docs/specs/graph-model.md` |
| API design | `docs/specs/api-design.md` |
| Contributing guide | `docs/contributing/internal-guide.md` |
| Tool calling guide | `docs/guides/tool-calling.md` |
| Superagent node reference | `docs/nodes/superagent.md` |
| Superagent concepts | `docs/concepts/superagent.md`, `docs/concepts/superagent-tools.md` |
| Superagent guides | `docs/guides/superagent-tutorial.md`, `docs/guides/superagent-advanced.md`, `docs/guides/superagent-patterns.md` |
| API tool node reference | `docs/nodes/api-tool.md` |
| API tools guide | `docs/guides/api-tools.md` |
| API reference | `docs/api/` |
| Deployment guides | `docs/deployment/` |
| Provider docs | `docs/providers/` |
| Coding agent skill | `coding-agent-skills/SKILL.md` |
| Cloud deploy (Terraform) | `deploy/terraform/` |
| Helm chart | `deploy/helm/brockley/` |
| User-facing docs | `docs/` |

---

## Build Loop

For every task:

1. Read `CLAUDE.md` + relevant specs
2. Check if specs are stale → update if needed
3. Implement a small, reviewable increment
4. Write tests (see Testing Requirements below)
5. Add structured logging (see Logging Requirements below)
6. **Update `CLAUDE.md` if conventions, structure, or workflows changed**
7. **Update `docs/specs/` if architecture, data model, API, or design changed**
8. **Update `docs/` if any user-facing behavior changed**
9. **Run CI checks locally before declaring done** (see CI Checks below)
10. Verify documentation checklist (see Mandatory Documentation Updates below)
11. Summarize: what changed, what's next

---

## CI Checks (must pass before any code is done)

**After ANY code change, run these checks and fix all failures before declaring the work complete:**

```bash
# 1. Format all Go files (fixes gofmt issues)
find . -name "*.go" -not -path "./.git/*" -exec gofmt -w {} \;

# 2. Run linter (must return 0 issues)
golangci-lint run ./...

# 3. Compile all packages
go build ./...

# 4. Run all unit tests
go test ./... -count=1

# 5. Run E2E tests (requires Docker -- MCP server, LLM provider)
make test-e2e

# Optional: debug only the real-LLM E2E manifests and print persisted LLM I/O
make test-e2e-llm-verbose
```

**All five must pass with zero errors.** Unit tests AND E2E tests must both pass before any code change is considered complete. Unit tests verify isolated logic; E2E tests verify end-to-end graph execution with real services. If any fail, fix the issues before moving on.

Common gotchas:
- **gofmt alignment**: Go's formatter standardizes spacing. Run `gofmt -w` on all files after any change.
- **unused variables/functions**: golangci-lint catches unused code. Remove it, don't comment it out.
- **missing imports**: If you use `context.TODO()` or similar, ensure the import is present.
- **errcheck**: Currently disabled in `.golangci.yml` -- idiomatic Go `defer Close()` patterns produce too many false positives.
- **staticcheck**: Don't pass `nil` as context -- use `context.TODO()` or `context.Background()`.

---

## E2E Test Coverage Requirement

**E2E tests must cover every testable graph capability.** When adding or changing graph capabilities (node types, execution model features, expression operators):
- Update existing E2E test graphs or add new test cases (manifests).
- Only create a new graph if the feature cannot fit into existing ones.
- **Run `make test-e2e` after any engine, CLI, or Terraform provider change.**
- Use `make test-e2e-llm-verbose` when debugging whether real LLM requests/responses are being formed correctly. This mode persists debug-only LLM traces on PostgreSQL execution steps.
- **E2E tests must pass alongside unit tests** -- passing `go test ./...` alone is not sufficient. Both must be green.

The goal is minimal graphs with maximal coverage.

---

## Testing Requirements

**Testing is mandatory. No exceptions.**

**Rules:**

1. **No code without tests.** Every PR that changes Go code includes tests.
2. **Interface → Mock → Test → Implementation.** Write the interface first, then the mock, then tests against the mock, then the real code.
3. **Every external dependency behind an interface.** LLM providers, MCP servers, database, Redis, secret store, trace exporters -- all Go interfaces. Tests use mocks.
4. **Tests run without external services.** `go test ./...` passes with zero infrastructure. Only E2E tests use Docker.
5. **Coverage:** >80% per package, >90% for the engine package.
6. **Table-driven tests** for multi-case functions. `testdata/` directories for fixtures. Golden files for expression language.
7. **Mock LLM providers** are the primary integration testing tool.
8. **Tool calling and tool loop tests:**
   - ToolCallCodec implementations must pass round-trip tests (encode → decode fidelity) for each provider.
   - Tool loop executor must be tested with `MockLLMProvider` (using `CompletionResponses` for scripted tool call sequences) and `MockMCPClient`.
   - Safety limits (`max_tool_calls`, `max_loop_iterations`) must have dedicated tests verifying enforcement.
   - Unknown tool recovery, MCP error handling, and context cancellation must be tested.

---

## Logging Requirements

**Structured logging is mandatory. Always on.**

**Rules:**

1. **`slog` with JSON output.** No `fmt.Println`, no unstructured output.
2. **Log every significant operation.** Node start/complete/fail, provider calls, state changes, execution lifecycle.
3. **Correlation IDs on everything.** `execution_id` and `request_id` on every log entry.
4. **No secrets in logs.** Log `api_key_ref` names, not values.
5. **Logger injection.** Components receive loggers via constructors or context. Never create loggers internally.
6. **Metrics opt-in.** Prometheus at `/metrics` via `BROCKLEY_METRICS_ENABLED=true`. No-op collector when disabled.
7. **Trace export opt-in.** Langfuse, Opik, Phoenix, LangSmith via `BROCKLEY_TRACE_*` env vars.

---

## Mandatory Documentation Updates

> **NON-NEGOTIABLE: Every code change MUST include corresponding updates to `CLAUDE.md` and relevant `docs/` files. A change without documentation updates is incomplete and must not be considered done.**

### Rules

1. **Update `CLAUDE.md` when:** conventions change, new components are added, build/test/CI steps change, new libraries are adopted, new decision policies are established, repo structure changes, or any workflow changes.
2. **Update `docs/specs/` when:** architecture changes (`docs/specs/architecture.md`), data model changes (`docs/specs/data-model.md`), API design changes (`docs/specs/api-design.md`), expression language changes (`docs/specs/expression-language.md`), or graph model changes (`docs/specs/graph-model.md`).
3. **Update `docs/` when:** any user-facing behavior changes. This is the user-facing doc layer -- if a user would notice the change, the docs must reflect it.
4. **The build loop is not complete until docs are updated.** CI passing is necessary but not sufficient -- stale docs are a bug.

### Documentation triggers

| What changed | Update |
|-------------|--------|
| Node types | `docs/nodes/` |
| CLI commands | `docs/cli/` |
| API endpoints | `docs/api/` + `docs/specs/api-design.md` |
| Providers | `docs/providers/` |
| Config/env vars | `docs/deployment/configuration.md` |
| Expression operators | `docs/expressions/` + `docs/specs/expression-language.md` |
| Graph execution model | `docs/specs/graph-model.md` |
| Data model/schema | `docs/specs/data-model.md` |
| Architecture | `docs/specs/architecture.md` |
| Build/test/CI conventions | `CLAUDE.md` |

### What to check before declaring done

- [ ] Does `CLAUDE.md` still accurately describe the repo conventions and workflows?
- [ ] Are `docs/specs/` specs consistent with the code that was just changed?
- [ ] Is `docs/` updated for any user-facing changes?
- [ ] Were both doc layers updated in the same PR as the code change?

---

## Decision Policy

**Ask before:**
- Core product scope changes
- Database/queue/auth strategy changes
- Major API convention changes
- Security policy changes
- Licensing changes

**Proceed without asking:**
- Internal naming, code organization, test additions, doc restructuring
- Reasonable defaults for low-risk areas
- Library choices within the decided tech stack

---

## Implementation Principles

- Prefer clarity over cleverness
- Keep components modular
- Prefer explicit schemas and types
- Keep interfaces narrow and intentional
- Write code easy for future agents and humans to understand
- Avoid adding dependencies without justification
- Preserve extensibility for plugins, providers, and custom node types
- Prefer deterministic validation over implicit behavior
- All configuration via environment variables (12-factor)
- All entities include `tenant_id` (always `"default"` for OSS)

---

## Output Expectations

When doing meaningful work:

1. Current phase
2. Files created or modified
3. What changed
4. Assumptions made
5. Decisions requiring input
6. Recommended next step
