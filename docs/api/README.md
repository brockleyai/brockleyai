# REST API Reference

Brockley exposes a versioned REST API at `/api/v1/` for programmatic access to all platform capabilities. Everything you can do in the CLI or web UI is available through this API.

## Reading order

Start with the overview for authentication and conventions, then read the endpoint groups you need.

1. **[API Overview](overview.md)** -- Base URL, authentication, request IDs, error format, versioning, and pagination.

2. **[Graphs](graphs.md)** -- Create, list, get, validate, update, and delete graphs.

3. **[Executions](executions.md)** -- Invoke graphs, check execution status, retrieve results, and stream events.

4. **[Schemas](schemas.md)** -- The schema library: reusable JSON Schema definitions shared across graphs.

5. **[Prompt Templates](prompt-templates.md)** -- The prompt template library: reusable prompt definitions for LLM nodes.

6. **[Provider Configs](provider-configs.md)** -- The provider config library: pre-configured LLM provider connections.

7. **[Health and Info](health.md)** -- Health checks, version info, and server metrics.

## Where to go next

- **[CLI Reference](../cli/)** -- The same operations from the command line.
- **[Terraform Provider](../terraform/)** -- Manage graphs declaratively as infrastructure.
