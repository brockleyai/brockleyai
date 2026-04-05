# ForEach Node

**Type:** `foreach`

The ForEach node iterates over an input array, executing an inline subgraph for each item. It collects results into an output array. Concurrency and error handling are configurable.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `graph` | object | Yes | An inline subgraph definition (a full graph JSON object). Executed once per item. |
| `concurrency` | integer | No | Maximum number of items processed in parallel. `0` (default) means unlimited concurrency. |
| `on_item_error` | string | No | Error handling policy: `"continue"` (default) or `"abort"`. |

### Error Handling Policies

- **`continue`** (default): If one item's subgraph execution fails, the error is recorded in the `errors` output and processing continues with remaining items. The `results` array contains only successful results, in their original order.
- **`abort`**: If any item's subgraph execution fails, the entire ForEach node fails immediately. No further items are processed.

### Concurrency

The `concurrency` field controls parallelism:

| Value | Behavior |
|-------|----------|
| `0` (default) | Unlimited -- all items processed in parallel. |
| `1` | Sequential -- items processed one at a time, in order. |
| `N` | At most N items in flight at any time. |

Use `concurrency: 1` when the inner graph calls rate-limited APIs or when order-dependent side effects matter. Use a higher value when items are independent and you want throughput.

## Input Ports

| Port | Type | Required | Description |
|------|------|----------|-------------|
| `items` | array | Yes | The array to iterate over. Each element is passed to the inner graph as `item`. |
| `context` | any | No | Optional shared context passed to every inner graph execution. |

## Output Ports

| Port | Type | Description |
|------|------|-------------|
| `results` | array | Array of output values from successful inner graph executions, preserving input order. |
| `errors` | array | Array of error objects for failed items. Each has `index`, `error` (message), and `item` (original input). Empty if all succeed. |

## Inner Subgraph Contract

The inline subgraph must have an input node with these output ports:

| Port | Type | Description |
|------|------|-------------|
| `item` | (matches array element type) | The current item from the `items` array. |
| `index` | integer | Zero-based index of the current item. |
| `context` | (matches context type) | The `context` value from the outer ForEach node (or `null` if not provided). |

The inner graph's output node produces the result for that item. The output value is collected into the `results` array.

The inner graph's state is isolated -- it cannot read or write the outer graph's state.

## Examples

### Basic: Process Each Item Through an LLM

```json
{
  "id": "process-all",
  "name": "Process Each Text",
  "type": "foreach",
  "input_ports": [
    {"name": "items", "schema": {"type": "array", "items": {"type": "string"}}},
    {"name": "context", "schema": {"type": "object"}}
  ],
  "output_ports": [
    {"name": "results", "schema": {"type": "array"}},
    {"name": "errors", "schema": {"type": "array"}}
  ],
  "config": {
    "concurrency": 3,
    "on_item_error": "continue",
    "graph": {
      "nodes": [
        {
          "id": "inner-in", "type": "input",
          "output_ports": [
            {"name": "item", "schema": {"type": "string"}},
            {"name": "index", "schema": {"type": "integer"}},
            {"name": "context", "schema": {"type": "object"}}
          ],
          "config": {}
        },
        {
          "id": "summarize", "type": "llm",
          "input_ports": [{"name": "text", "schema": {"type": "string"}}],
          "output_ports": [{"name": "response_text", "schema": {"type": "string"}}],
          "config": {
            "provider": "openai",
            "model": "gpt-4o-mini",
            "api_key_ref": "openai-key",
            "system_prompt": "Summarize the following text in one sentence.",
            "user_prompt": "{{input.text}}",
            "variables": [{"name": "text", "schema": {"type": "string"}}],
            "response_format": "text"
          }
        },
        {
          "id": "inner-out", "type": "output",
          "input_ports": [{"name": "summary", "schema": {"type": "string"}}],
          "config": {}
        }
      ],
      "edges": [
        {"id": "ie1", "source_node_id": "inner-in", "source_port": "item", "target_node_id": "summarize", "target_port": "text"},
        {"id": "ie2", "source_node_id": "summarize", "source_port": "response_text", "target_node_id": "inner-out", "target_port": "summary"}
      ]
    }
  }
}
```

### Using Context for Shared Configuration

Pass shared data (like a category list) to every iteration:

```json
{
  "config": {
    "concurrency": 5,
    "on_item_error": "continue",
    "graph": {
      "nodes": [
        {
          "id": "inner-in", "type": "input",
          "output_ports": [
            {"name": "item", "schema": {"type": "object"}},
            {"name": "index", "schema": {"type": "integer"}},
            {"name": "context", "schema": {"type": "object"}}
          ],
          "config": {}
        },
        {
          "id": "classify", "type": "llm",
          "input_ports": [
            {"name": "item", "schema": {"type": "object"}},
            {"name": "categories", "schema": {"type": "string"}}
          ],
          "output_ports": [{"name": "response", "schema": {"type": "object"}}],
          "config": {
            "provider": "anthropic",
            "model": "claude-sonnet-4-20250514",
            "api_key_ref": "anthropic-key",
            "user_prompt": "Classify this item into one of: {{input.categories}}\n\nItem: {{input.item | json}}",
            "variables": [
              {"name": "item", "schema": {"type": "object"}},
              {"name": "categories", "schema": {"type": "string"}}
            ],
            "response_format": "json",
            "output_schema": {"type": "object", "properties": {"category": {"type": "string"}, "confidence": {"type": "number"}}, "required": ["category", "confidence"]}
          }
        },
        {
          "id": "inner-out", "type": "output",
          "input_ports": [{"name": "classification", "schema": {"type": "object"}}],
          "config": {}
        }
      ],
      "edges": [
        {"id": "ie1", "source_node_id": "inner-in", "source_port": "item", "target_node_id": "classify", "target_port": "item"},
        {"id": "ie2", "source_node_id": "inner-in", "source_port": "context", "target_node_id": "classify", "target_port": "categories"},
        {"id": "ie3", "source_node_id": "classify", "source_port": "response", "target_node_id": "inner-out", "target_port": "classification"}
      ]
    }
  }
}
```

The `context` input receives something like `"billing, support, sales, other"`, and each inner graph iteration uses it.

### Sequential Processing with Abort on Error

When partial results are not useful and order matters:

```json
{
  "config": {
    "concurrency": 1,
    "on_item_error": "abort",
    "graph": { "..." }
  }
}
```

Items are processed one at a time. If any item fails, the entire node fails immediately.

### Output Structure

Given input items `["hello", "world", "test"]` and a successful run:

```json
{
  "results": [
    {"summary": "A greeting"},
    {"summary": "Reference to the globe"},
    {"summary": "A trial run"}
  ],
  "errors": []
}
```

If the second item fails (with `on_item_error: "continue"`):

```json
{
  "results": [
    {"summary": "A greeting"},
    {"summary": "A trial run"}
  ],
  "errors": [
    {
      "index": 1,
      "error": "llm executor: provider call failed: openai: API error (status 429): rate limit exceeded",
      "item": "world"
    }
  ]
}
```

The `results` array contains only successful results. Failed items appear only in `errors`. The order of successful results is preserved relative to the original input array.

## See Also

- [Loops](../concepts/loops.md) -- back-edges, conditions, and iteration patterns
- [Subgraphs](../concepts/subgraphs.md) -- inner graph contracts, state isolation, reuse patterns
- [Branching](../concepts/branching.md#foreach-fan-out) -- foreach as a fan-out pattern
- [Subgraph Node](subgraph.md) -- execute an inner graph once (not per-item)
- [LLM Node](llm.md) -- commonly used inside ForEach inner graphs
- [Transform Node](transform.md) -- data shaping before/after ForEach
- [Expression Language Overview](../expressions/overview.md) -- expressions in inner graph nodes
- [Data Model: ForEach Node Config](../specs/data-model.md) -- complete field reference
