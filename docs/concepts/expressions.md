# Expressions

Brockley includes a built-in expression language used across all node types that evaluate dynamic values: LLM prompt templates, conditional branch conditions, transform expressions, and back-edge loop conditions. The language is intentionally limited -- no side effects, no function definitions, no variable assignment. It evaluates an expression against a read-only context and returns a value.

For the full operator and filter reference, see the [Expression Language Reference](../expressions/overview.md).

## Where Expressions Are Used

### Prompt Templates (LLM Nodes)

LLM nodes use expressions inside `{{ }}` delimiters in their `system_prompt` and `user_prompt` fields:

```
Summarize the following text in {{input.language ?? "English"}}:

{{input.text}}

{{#if state.prior_summary}}
Previous summary for context: {{state.prior_summary}}
{{/if}}
```

Templates support interpolation (`{{ expr }}`), conditional blocks (`{{#if}}`/`{{#else}}`/`{{/if}}`), and iteration blocks (`{{#each}}`/`{{/each}}`).

### Conditional Branch Conditions

[Conditional nodes](branching.md) evaluate expressions as standalone boolean conditions (no `{{ }}` wrapper):

```
input.category == 'billing' && input.priority == 'high'
```

Branches are evaluated in order. The first one that returns a truthy value fires.

### Transform Expressions

[Transform nodes](../nodes/transform.md) map output port names to expression strings. Each expression is evaluated and its result becomes the output port's value:

```json
{
  "expressions": {
    "full_name": "input.first_name + ' ' + input.last_name",
    "item_count": "input.items | length",
    "active_items": "input.items | filter(x => x.status == 'active')"
  }
}
```

### Back-Edge Conditions

[Back-edges](loops.md) use expressions to determine whether a loop continues:

```
state.iteration_count < 5 && input.verdict == 'needs_more'
```

## Namespaces

Every expression has access to three root namespaces:

### `input` -- Node Input Data

The `input` namespace contains the resolved values of the node's input ports. Keys are port names:

```
input.query              -- string port value
input.items[0]           -- first element of an array port
input.user.name          -- nested field access on an object port
```

### `state` -- Graph State (Read-Only)

The `state` namespace provides a read-only snapshot of all graph [state](state.md) fields at the time the node starts execution. You do not need `state_reads` bindings to access these values:

```
state.conversation_history              -- array state field
state.attempt_count                     -- integer state field
state.metadata.model                    -- nested access on object state
```

This makes it easy to reference state directly in conditions and templates without extra configuration:

```
{{#if state.messages | length > 0}}
Conversation so far:
{{#each state.messages}}
- {{this.role}}: {{this.content}}
{{/each}}
{{/if}}
```

### `meta` -- Execution Metadata

The `meta` namespace provides information about the current execution context:

| Field | Type | Description |
|-------|------|-------------|
| `meta.node_id` | string | ID of the currently executing node |
| `meta.node_name` | string | Human-readable name of the node |
| `meta.node_type` | string | Node type (llm, transform, conditional, etc.) |
| `meta.execution_id` | string | ID of the current execution |
| `meta.graph_id` | string | ID of the current graph |
| `meta.graph_name` | string | Name of the current graph |
| `meta.iteration` | integer | Current loop iteration (0-based, only set inside [loops](loops.md)) |

## Key Examples

### String manipulation

```
input.name | upper                                    -- "ALICE"
input.email | lower | trim                            -- "alice@example.com"
input.description | truncate(100)                     -- "First 100 chars..."
"Hello, " + input.name + "!"                          -- "Hello, Alice!"
input.csv_line | split(",") | map(x => x | trim)      -- ["a", "b", "c"]
```

### Array processing

```
input.items | filter(x => x.score > 0.8)             -- keep high-scoring items
input.items | map(x => x.name) | sort                -- extract names, sort them
input.items | length                                  -- count items
input.numbers | sum                                   -- add up all numbers
input.tags | unique | join(", ")                      -- deduplicate and join
[1, 2, 3] | any(x => x > 2)                          -- true
input.results | groupBy("category")                   -- group into object by field
```

### Object operations

```
input.data | keys                                     -- ["name", "age", "email"]
input.config | pick("host", "port")                   -- keep only these keys
input.defaults | merge(input.overrides)               -- combine objects
input.data | has("optional_field")                    -- check if key exists
```

### Null safety

```
input.optional ?? "default value"                     -- null coalescing
input.user?.address?.city                             -- optional chaining (null if any part is null)
input.score ?? 0 > 0.5                                -- default before comparison
```

### Conditional logic

```
input.age >= 18 ? "adult" : "minor"                   -- ternary
input.items | length > 0 && input.enabled             -- logical AND
!input.processed || input.force_reprocess             -- logical OR with NOT
```

### Type conversion

```
"42" | toInt                                          -- 42
3.14159 | round(2)                                    -- 3.14
{name: "Alice", age: 30} | json                      -- '{"name":"Alice","age":30}'
'{"key": "value"}' | parseJson                        -- {key: "value"}
```

### Token estimation

```
input.text | tokenEstimate                            -- rough token count (chars / 4)
state.messages | tokenEstimate > 4000                 -- check context size
```

## What Expressions Cannot Do

- No variable assignment or mutation
- No function definitions
- No loops (use array operations like `map`, `filter`, `reduce` instead)
- No side effects (no HTTP calls, no state writes, no I/O)
- No imports or external references

The expression language is pure: given the same input, it always produces the same output.

## See Also

- [Expression Language Reference](../expressions/overview.md) -- complete operator and syntax reference
- [Operators Reference](../expressions/operators.md) -- all comparison, logical, and arithmetic operators
- [Filters Reference](../expressions/filters.md) -- all array, string, object, and type filters
- [Ports and Typing](ports-and-typing.md) -- the `input.*` namespace comes from port values
- [State](state.md) -- the `state.*` namespace comes from graph state fields
- [Loops](loops.md) -- `meta.iteration` and back-edge condition expressions
