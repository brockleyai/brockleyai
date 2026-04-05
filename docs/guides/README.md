# Guides

Hands-on tutorials and patterns for building with Brockley. These guides assume you're familiar with [core concepts](../concepts/) and the [node type reference](../nodes/).

## Reading order

Start with tool calling, then move into agents.

1. **[Tool Calling](tool-calling.md)** -- How to configure tool calling on LLM nodes: no tools, tools without loop, and the full autonomous tool loop. Most users start here.

2. **[API Tools](api-tools.md)** -- Configure REST API tool nodes to call external HTTP endpoints directly, with request/response mapping.

3. **[Build Your First Agent](superagent-tutorial.md)** -- Step-by-step walkthrough of building a superagent that plans, uses tools, and produces a final result.

4. **[Customizing the Superagent](superagent-advanced.md)** -- Fine-tune superagent behavior: evaluation strategies, reflection, custom termination, and model selection.

5. **[Multi-Agent Patterns](superagent-patterns.md)** -- Compose multiple agents: sequential pipelines, parallel execution, shared memory, and supervisor patterns.

## Where to go next

- **[Node Types](../nodes/)** -- Reference for every node type used in these guides.
- **[Superagent Concepts](../concepts/superagent.md)** -- Conceptual foundation for the agent guides.
- **[Expression Language](../expressions/)** -- Full reference for the expressions used in examples throughout.
