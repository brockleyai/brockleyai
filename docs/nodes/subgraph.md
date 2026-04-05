# Subgraph Node

**Type:** `subgraph`

The subgraph node encapsulates a complete graph inside a single node. It maps the outer node's ports to the inner graph's input and output nodes via port mapping. This enables reusable, composable workflow components with clean interfaces.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `graph` | object | Yes | An inline subgraph definition (a full graph JSON object). |
| `port_mapping` | PortMapping | Yes | Maps outer node ports to inner graph ports. |

### Port Mapping

| Field | Type | Description |
|-------|------|-------------|
| `inputs` | map[string]string | Maps outer input port names to inner graph input ports. |
| `outputs` | map[string]string | Maps inner graph output ports to outer output port names. |

Port names can use two formats:

- **Bare name**: `"query"` -- refers to a port on the inner graph's input or output node.
- **Node-prefixed**: `"sub-in.query"` -- explicitly names the inner node. The `nodeID.` prefix is stripped when resolving.

The node-prefixed format is functionally equivalent but makes the mapping more explicit when the inner graph has multiple nodes.

## State Scope Isolation

The inner graph's state is **completely isolated** from the outer graph:

- State fields defined in the inner graph exist only for the duration of the subgraph execution.
- The inner graph cannot read or write the outer graph's state.
- Data flows in and out exclusively through the mapped ports.

This isolation makes subgraphs predictable and composable. You can embed the same subgraph in different outer graphs without state conflicts.

### Why This Matters

Without isolation, a subgraph that writes to `state.count` could collide with an outer graph's own `state.count` field. Isolation prevents this class of bugs entirely. If you need to pass accumulated data into a subgraph, do it through an input port. If the subgraph produces data for the outer state, return it through an output port and use a `state_writes` binding on the outer subgraph node.

## How It Works

1. The subgraph node receives data on its input ports.
2. The `port_mapping.inputs` determines how outer input values map to the inner graph's input node ports.
3. The inner graph executes as a standalone graph (with its own isolated state).
4. When the inner graph completes, `port_mapping.outputs` maps the inner graph's output node values back to the outer node's output ports.

## Examples

### Basic Subgraph: Classify and Format

```json
{
  "id": "classify-block",
  "name": "Classify and Format",
  "type": "subgraph",
  "input_ports": [
    {"name": "text", "schema": {"type": "string"}},
    {"name": "categories", "schema": {"type": "array", "items": {"type": "string"}}}
  ],
  "output_ports": [
    {"name": "result", "schema": {"type": "object"}}
  ],
  "config": {
    "port_mapping": {
      "inputs": {
        "text": "text",
        "categories": "categories"
      },
      "outputs": {
        "formatted_result": "result"
      }
    },
    "graph": {
      "nodes": [
        {
          "id": "sub-in", "type": "input",
          "output_ports": [
            {"name": "text", "schema": {"type": "string"}},
            {"name": "categories", "schema": {"type": "array", "items": {"type": "string"}}}
          ],
          "config": {}
        },
        {
          "id": "classify", "type": "llm",
          "input_ports": [
            {"name": "text", "schema": {"type": "string"}},
            {"name": "cats", "schema": {"type": "array", "items": {"type": "string"}}}
          ],
          "output_ports": [{"name": "response", "schema": {"type": "object"}}],
          "config": {
            "provider": "openai", "model": "gpt-4o",
            "api_key_ref": "openai-key",
            "system_prompt": "Classify the text into one of the given categories.",
            "user_prompt": "Categories: {{input.cats | join(', ')}}\n\nText: {{input.text}}",
            "variables": [
              {"name": "text", "schema": {"type": "string"}},
              {"name": "cats", "schema": {"type": "array", "items": {"type": "string"}}}
            ],
            "response_format": "json",
            "output_schema": {
              "type": "object",
              "properties": {
                "category": {"type": "string"},
                "confidence": {"type": "number"}
              },
              "required": ["category", "confidence"]
            }
          }
        },
        {
          "id": "format", "type": "transform",
          "input_ports": [
            {"name": "classification", "schema": {"type": "object"}},
            {"name": "original_text", "schema": {"type": "string"}}
          ],
          "output_ports": [{"name": "formatted_result", "schema": {"type": "object"}}],
          "config": {
            "expressions": {
              "formatted_result": "{text: input.original_text, category: input.classification.category, confidence: input.classification.confidence}"
            }
          }
        },
        {
          "id": "sub-out", "type": "output",
          "input_ports": [{"name": "formatted_result", "schema": {"type": "object"}}],
          "config": {}
        }
      ],
      "edges": [
        {"id": "se1", "source_node_id": "sub-in", "source_port": "text", "target_node_id": "classify", "target_port": "text"},
        {"id": "se2", "source_node_id": "sub-in", "source_port": "categories", "target_node_id": "classify", "target_port": "cats"},
        {"id": "se3", "source_node_id": "classify", "source_port": "response", "target_node_id": "format", "target_port": "classification"},
        {"id": "se4", "source_node_id": "sub-in", "source_port": "text", "target_node_id": "format", "target_port": "original_text"},
        {"id": "se5", "source_node_id": "format", "source_port": "formatted_result", "target_node_id": "sub-out", "target_port": "formatted_result"}
      ]
    }
  }
}
```

### Port Mapping with Node ID Prefix

For clarity in complex inner graphs:

```json
{
  "config": {
    "port_mapping": {
      "inputs": {
        "query": "sub-in.query",
        "config": "sub-in.config"
      },
      "outputs": {
        "sub-out.answer": "answer",
        "sub-out.metadata": "metadata"
      }
    },
    "graph": { "..." }
  }
}
```

### Reuse: Same Subgraph, Different Contexts

Define a sentiment analysis subgraph and embed it in multiple places:

```json
{
  "nodes": [
    {
      "id": "analyze-reviews",
      "type": "subgraph",
      "input_ports": [{"name": "text", "schema": {"type": "string"}}],
      "output_ports": [{"name": "sentiment", "schema": {"type": "object"}}],
      "config": {
        "port_mapping": {"inputs": {"text": "text"}, "outputs": {"sentiment": "sentiment"}},
        "graph": { "..." }
      }
    },
    {
      "id": "analyze-feedback",
      "type": "subgraph",
      "input_ports": [{"name": "text", "schema": {"type": "string"}}],
      "output_ports": [{"name": "sentiment", "schema": {"type": "object"}}],
      "config": {
        "port_mapping": {"inputs": {"text": "text"}, "outputs": {"sentiment": "sentiment"}},
        "graph": { "..." }
      }
    }
  ]
}
```

Both nodes use the same inner graph definition. Each runs with its own isolated state.

## Subgraph vs ForEach

| Feature | Subgraph | ForEach |
|---------|----------|---------|
| Purpose | Encapsulate a reusable workflow | Iterate over an array |
| Executions | Once per node execution | Once per array item |
| Port mapping | Explicit via `port_mapping` | Fixed contract (`item`, `index`, `context`) |
| State | Isolated inner scope | Isolated inner scope |
| Concurrency | N/A (single execution) | Configurable via `concurrency` |

## See Also

- [ForEach Node](foreach.md) -- iterate over arrays with an inner graph
- [Input and Output Nodes](input-output.md) -- the inner graph's boundary nodes
- [Transform Node](transform.md) -- simpler data manipulation without a full subgraph
- [Data Model: Subgraph Node Config](../specs/data-model.md) -- complete field reference
- [Graph Model](../specs/graph-model.md) -- graph composition and state semantics
