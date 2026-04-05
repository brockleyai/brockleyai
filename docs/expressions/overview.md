# Expression Language Overview

Brockley includes a built-in expression language used for template interpolation, conditional evaluation, and data transformation. The language is intentionally limited -- no side effects, no arbitrary computation, no function definitions. It evaluates expressions against a read-only context and produces a value.

## Where Expressions Are Used

| Context | Input | Output | Example |
|---------|-------|--------|---------|
| **Prompt template** | Template string with `{{ }}` blocks | Rendered string | `"Summarize: {{input.text}}"` |
| **Conditional branch** | Expression string | boolean | `input.score > 0.8` |
| **Transform expression** | Expression string | any value | `input.items \| filter(x => x.active)` |
| **Back-edge condition** | Expression string | boolean | `input.verdict == "needs_more"` |
| **ForEach iterator** | Expression string | array | `input.documents` |

In templates (`{{...}}`), the expression is embedded in surrounding text. In conditions and transforms, the expression is the entire value -- no `{{}}` wrapper needed.

## Namespaces

Expressions have access to three root namespaces:

### `input`

Values received on the node's input ports. Keys are port names.

```
input.query                    -- dot access
input.users[0]                 -- array index (zero-based)
input.users[-1]                -- negative index (last element)
input.data["field-name"]       -- bracket access for special characters
```

### `state`

Read-only snapshot of all graph state fields at the time the node starts execution. Keys are state field names.

```
state.conversation_history     -- state field
state.count                    -- accumulated counter
state.previous_results         -- prior outputs
```

The `state` namespace is available in all expression contexts without needing `state_reads` bindings. You can also use `state_reads` to map state fields to input ports (accessible as `input.fieldname`), but direct `state.*` access is simpler for most use cases.

### `meta`

Execution metadata:

| Field | Type | Description |
|-------|------|-------------|
| `meta.node_id` | string | ID of the currently executing node. |
| `meta.node_name` | string | Name of the currently executing node. |
| `meta.node_type` | string | Type of the currently executing node. |
| `meta.execution_id` | string | ID of the current execution. |
| `meta.graph_id` | string | ID of the current graph. |
| `meta.graph_name` | string | Name of the current graph. |
| `meta.iteration` | integer | Current loop iteration (only set inside loops). |

```
meta.execution_id              -- "exec-abc123"
meta.iteration                 -- 0, 1, 2, ...
meta.node_name                 -- "Classify Input"
```

## Literals

| Type | Examples |
|------|---------|
| String | `"hello"`, `'world'` (both single and double quotes, with `\\`, `\"`, `\'`, `\n`, `\t` escapes) |
| Integer | `42`, `-7`, `0` |
| Float | `3.14`, `-0.5` |
| Boolean | `true`, `false` |
| Null | `null` |
| Array | `[1, 2, 3]`, `["a", "b"]`, `[]` |
| Object | `{name: "Alice", age: 30}`, `{}` |

## Operator Precedence

From lowest to highest:

| Precedence | Operator | Description |
|------------|----------|-------------|
| 1 (lowest) | `\|` | Pipe (filter application) |
| 2 | `? :` | Ternary conditional |
| 3 | `??` | Null coalesce |
| 4 | `\|\|` | Logical OR |
| 5 | `&&` | Logical AND |
| 6 | `==` `!=` | Equality |
| 7 | `>` `>=` `<` `<=` | Comparison |
| 8 | `+` `-` | Addition, subtraction, string concatenation |
| 9 | `*` `/` `%` | Multiplication, division, modulo |
| 10 | `!` | Logical NOT (unary) |
| 11 (highest) | `.` `?.` `[]` `()` | Property access, optional chain, index, call |

Parentheses `()` override precedence.

## Property Access

```
input.user.name              -- nested dot access
input.items[0]               -- array index (zero-based)
input.items[-1]              -- negative index (from end)
input.data["key"]            -- bracket notation
input.user?.address?.city    -- optional chaining (null if user or address is null)
```

## Pipe Syntax

The pipe operator `|` passes the left-hand value to the right-hand filter:

```
input.items | length
input.items | filter(x => x.active) | map(x => x.name) | sort
input.name | upper | trim
```

Pipes can be chained to build data processing pipelines.

## Lambda Expressions

Used as arguments to filter functions like `map`, `filter`, `any`, and `all`:

```
x => x.score > 0.5           -- comparison
x => x.name                  -- field extraction
x => x.first + ' ' + x.last -- computation
```

## Method-Style Access

Filters can be called with dot notation as an alternative to pipe notation:

```
input.items.length()          -- same as input.items | length
input.name.upper()            -- same as input.name | upper
input.list.first()            -- same as input.list | first
```

## Template Blocks

Inside `{{...}}` templates (LLM prompts), these block directives are available:

### `#if` / `#else`

```
{{#if input.include_context}}
Context: {{input.context}}
{{#else}}
No context provided.
{{/if}}
```

### `#each`

```
{{#each input.items}}
- {{this}} (index {{@index}})
{{/each}}
```

Inside `#each`: `{{this}}` (current item), `{{@index}}` (zero-based), `{{@first}}`, `{{@last}}`.

### `raw`

```
{{raw}}This {{text}} is not evaluated.{{/raw}}
```

See [Template Syntax](templates.md) for full details.

## Error Handling

- Accessing a missing field returns `null` (not an error).
- `null` propagates through operations: `null + 1` returns `null`.
- Use `??` to provide defaults: `input.name ?? "anonymous"`.
- Use `?.` to safely traverse: `input.user?.profile?.avatar`.
- Division by zero returns `null`.
- Type mismatches in comparisons return `false`.

## What Expressions Cannot Do

- No variable assignment
- No function definitions
- No loops (use array operations instead)
- No side effects (no HTTP calls, no state mutation, no I/O)
- No imports or external references
- No recursion

The expression language is pure: given the same input, it always produces the same output.

## Quick Examples

```
-- Arithmetic
input.price * input.quantity * (1 + input.tax_rate)

-- String operations
input.email | lower | trim

-- Null safety
input.user?.address?.city ?? "Unknown"

-- Conditional logic
input.age >= 18 ? "adult" : "minor"

-- Array processing
input.users | filter(u => u.active) | map(u => u.name) | sort

-- Object construction
{name: input.user.name, count: input.items | length}

-- Using state
state.running_total + input.new_value
```

## See Also

- [Operators](operators.md) -- comparison, logical, arithmetic, null handling
- [Filters](filters.md) -- all pipe filters
- [Arrays](arrays.md) -- array selection, transformation, filtering, aggregation
- [Strings](strings.md) -- string manipulation operations
- [Objects](objects.md) -- object field operations
- [Templates](templates.md) -- `#if`, `#each`, `raw` block syntax
- [Type Operations](type-ops.md) -- type conversion and numeric operations
- [Expression Language Spec](../specs/expression-language.md) -- complete language specification
