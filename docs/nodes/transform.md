# Transform Node

**Type:** `transform`

The transform node evaluates [expression language](../expressions/overview.md) expressions against its inputs and produces computed output values. Each expression in the configuration becomes an output port. No external calls are made -- transforms are pure, in-process data manipulation.

## Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `expressions` | map[string]string | Yes | A map of output port name to expression string. Each key becomes an output port, and the expression is evaluated to produce that port's value. |

## How It Works

1. The transform node receives data on its input ports from upstream nodes.
2. For each entry in `expressions`, the expression is evaluated with three namespaces available:
   - `input` -- values received on the node's input ports
   - `state` -- current graph state field values
   - `meta` -- execution metadata (`meta.execution_id`, `meta.node_id`, `meta.iteration`, etc.)
3. The result of each expression is emitted on the corresponding output port.

## Input Ports

Input ports are defined by the graph author. They should match whatever data upstream nodes produce.

## Output Ports

One output port is created for each key in the `expressions` map.

## Examples

### String Formatting

```json
{
  "config": {
    "expressions": {
      "full_name": "input.first_name + ' ' + input.last_name",
      "greeting": "'Hello, ' + input.first_name + '!'"
    }
  }
}
```

### Arithmetic

```json
{
  "config": {
    "expressions": {
      "subtotal": "input.price * input.quantity",
      "tax": "input.price * input.quantity * input.tax_rate",
      "total": "input.price * input.quantity * (1 + input.tax_rate)"
    }
  }
}
```

### Array Filtering

```json
{
  "config": {
    "expressions": {
      "active_items": "input.items | filter(item => item.status == 'active')",
      "active_count": "input.items | filter(item => item.status == 'active') | length",
      "high_value": "input.items | filter(item => item.price > 100) | sort | reverse"
    }
  }
}
```

### Array Transformation and Aggregation

```json
{
  "config": {
    "expressions": {
      "names": "input.users | map(u => u.name) | sort",
      "total_value": "input.items | map(i => i.value) | sum",
      "top_3": "input.scores | sort | reverse | slice(0, 3)",
      "tags": "input.raw_tags | split(',') | map(t => t | trim) | unique"
    }
  }
}
```

### Object Construction and Reshaping

```json
{
  "config": {
    "expressions": {
      "request": "{user: input.user_id, action: input.action, data: input.payload}",
      "summary": "{count: input.items | length, first: input.items | first, last: input.items | last}"
    }
  }
}
```

### Extracting Fields from Nested Objects

```json
{
  "config": {
    "expressions": {
      "items": "input.response.data",
      "total": "input.response.data | length",
      "has_more": "input.response.next_cursor != null",
      "page_info": "{total: input.response.data | length, cursor: input.response.next_cursor}"
    }
  }
}
```

### Null Handling and Defaults

```json
{
  "config": {
    "expressions": {
      "display_name": "input.nickname ?? input.full_name ?? 'Anonymous'",
      "safe_count": "input.data?.items | length ?? 0",
      "status": "input.enabled ? 'active' : 'disabled'"
    }
  }
}
```

### Using Pipe Chains

```json
{
  "config": {
    "expressions": {
      "label": "input.name | upper | truncate(50)",
      "emails": "input.users | filter(u => u.email != null) | map(u => u.email | lower | trim)",
      "csv_row": "[input.id, input.name, input.score | toString] | join(',')",
      "unique_tags": "input.items | map(i => i.tags) | flatten | unique | sort"
    }
  }
}
```

### Combining State and Input

```json
{
  "config": {
    "expressions": {
      "accumulated": "state.running_total + input.new_value",
      "iteration_label": "'Iteration ' + meta.iteration + ': ' + input.status",
      "all_results": "state.previous_results | concat([input.new_result])"
    }
  }
}
```

### Conditional Expressions

```json
{
  "config": {
    "expressions": {
      "tier": "input.score > 0.8 ? 'high' : input.score > 0.5 ? 'medium' : 'low'",
      "message": "input.items | length > 0 ? 'Found ' + (input.items | length | toString) + ' items' : 'No items found'"
    }
  }
}
```

### JSON Serialization

```json
{
  "config": {
    "expressions": {
      "payload": "{query: input.search_term, filters: input.active_filters} | json",
      "debug_output": "input | json"
    }
  }
}
```

## See Also

- [Expression Language Overview](../expressions/overview.md) -- full expression syntax
- [Operators Reference](../expressions/operators.md) -- arithmetic, comparison, logical, null handling
- [Filters Reference](../expressions/filters.md) -- all pipe filters
- [Array Operations](../expressions/arrays.md) -- filtering, mapping, aggregation
- [String Operations](../expressions/strings.md) -- formatting, splitting, matching
- [Object Operations](../expressions/objects.md) -- keys, values, merge, pick, omit
- [Type Operations](../expressions/type-ops.md) -- type conversion and numeric operations
- [Conditional Node](conditional.md) -- routing based on conditions
- [Data Model: Transform Node Config](../specs/data-model.md) -- complete field reference
