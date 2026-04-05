# Internal Specifications

Technical specifications and design documents for contributors and maintainers. These are the authoritative reference for how Brockley is built -- not how to use it. For user-facing documentation, see [Core Concepts](../concepts/).

## Reading order

Start with the architecture for the big picture, then drill into the areas you're working on.

1. **[Architecture](architecture.md)** -- System overview: API server, storage layer, execution pipeline, worker model, and how components interact.

2. **[Data Model](data-model.md)** -- Core entities (graphs, nodes, edges, executions), their fields, relationships, JSONB storage, and validation rules.

3. **[Graph Model](graph-model.md)** -- The execution model in detail: typed ports, state reducers, back-edges, skip propagation, and branching primitives.

4. **[Expression Language](expression-language.md)** -- Full DSL specification: grammar, evaluation rules, and type system.

5. **[API Design](api-design.md)** -- REST API conventions: URL structure, versioning, request/response formats, error handling, and pagination.

## Where to go next

- **[Contributing Guide](../contributing/)** -- How to set up your development environment and submit changes.
- **[Core Concepts](../concepts/)** -- The user-facing version of these topics, written for Brockley users rather than contributors.
