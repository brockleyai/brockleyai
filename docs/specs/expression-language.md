# Expression Language

Brockley uses a single expression language across all contexts: prompt templates, conditional branch conditions, transform expressions, back-edge conditions, and foreach iterators. The language is intentionally limited -- no side effects, no arbitrary computation, no function definitions. It evaluates expressions against a read-only context and produces a value.

---

## Contexts Where Expressions Are Used

| Context | Input | Output | Example |
|---------|-------|--------|---------|
| **Prompt template** | Template string with `{{ }}` blocks | Rendered string | `"Summarize: {{input.text}}"` |
| **Conditional branch condition** | Expression string | boolean | `input.score > 0.8` |
| **Transform expression** | Expression string | any value | `input.items \| filter(x => x.status == "active")` |
| **Back-edge condition** | Expression string | boolean | `input.verdict == "needs_more"` |
| **ForEach iterator** | Expression string | array | `input.documents` |

---

## Namespaces

Every expression has access to these namespaces:

| Namespace | Available in | Description |
|-----------|-------------|-------------|
| `input` | All contexts | The node's resolved input port values |
| `state` | All contexts | Current graph state (read-only snapshot at node execution start) |
| `meta` | All contexts | Execution metadata |

### `state` namespace

The `state` namespace provides a read-only snapshot of all graph state fields at the time the node starts execution. State values are accessible in all expression contexts without needing `state_reads` bindings. For example, `state.count` returns the current value of the `count` state field.

Note: `state_reads` bindings are still supported for mapping state fields to specific input port names (accessible as `input.fieldname`), but direct `state.*` access is simpler for most use cases.

### `meta` fields

| Field | Type | Description |
|-------|------|-------------|
| `meta.node_id` | string | ID of the currently executing node |
| `meta.node_name` | string | Name of the currently executing node |
| `meta.node_type` | string | Type of the currently executing node |
| `meta.execution_id` | string | ID of the current execution |
| `meta.graph_id` | string | ID of the current graph |
| `meta.graph_name` | string | Name of the current graph |
| `meta.iteration` | integer | Current loop iteration (only set inside loops) |

---

## Literals

| Type | Syntax | Examples |
|------|--------|---------|
| String | `"..."` or `'...'` | `"hello"`, `'world'` |
| Integer | digits | `42`, `-1`, `0` |
| Float | digits with `.` | `3.14`, `-0.5` |
| Boolean | `true`, `false` | `true` |
| Null | `null` | `null` |
| Array | `[a, b, c]` | `[1, 2, 3]`, `["a", "b"]` |
| Object | `{key: value}` | `{name: "Alice", age: 30}` |

---

## Operators

### Comparison

| Operator | Description | Example |
|----------|-------------|---------|
| `==` | Equal | `input.status == "active"` |
| `!=` | Not equal | `input.type != "draft"` |
| `>` | Greater than | `input.score > 0.8` |
| `>=` | Greater than or equal | `input.count >= 10` |
| `<` | Less than | `input.retries < 3` |
| `<=` | Less than or equal | `input.confidence <= 0.5` |

Comparison is type-strict: `"5" != 5`. No implicit coercion.

### Logical

| Operator | Description | Example |
|----------|-------------|---------|
| `&&` | Logical AND | `input.ready && input.score > 0.5` |
| `\|\|` | Logical OR | `input.urgent \|\| input.priority == "high"` |
| `!` | Logical NOT | `!input.processed` |

Short-circuit evaluation: `false && expr` does not evaluate `expr`.

### Arithmetic

| Operator | Description | Example |
|----------|-------------|---------|
| `+` | Addition / string concatenation | `input.a + input.b` |
| `-` | Subtraction | `input.total - input.discount` |
| `*` | Multiplication | `input.price * input.quantity` |
| `/` | Division | `input.total / input.count` |
| `%` | Modulo | `input.index % 2` |

Division by zero returns `null`.

### Null Handling

| Operator | Description | Example |
|----------|-------------|---------|
| `??` | Null coalescing | `input.name ?? "unknown"` |
| `?.` | Optional chaining | `input.user?.address?.city` |

### Ternary

```
condition ? value_if_true : value_if_false
```

### Operator Precedence (high to low)

1. `?.` (optional chaining)
2. `!` (NOT)
3. `*`, `/`, `%`
4. `+`, `-`
5. `>`, `>=`, `<`, `<=`
6. `==`, `!=`
7. `&&`
8. `||`
9. `??`
10. `? :` (ternary)

Parentheses `()` override precedence.

---

## Property Access

```
input.user.name          -- nested field access
input.items[0]           -- array index (0-based)
input.items[-1]          -- negative index (from end)
input.data["key"]        -- bracket notation
input.user?.address      -- optional chaining (null if user is null)
```

---

## Array Operations

Array operations use pipe syntax in templates and method syntax in expressions. Both are equivalent.

### Pipe Syntax (in templates)

```
{{input.items | length}}
{{input.items | first}}
{{state.messages | last}}
```

### Method Syntax (in conditions/transforms)

```
input.items.length()
input.items.first()
input.items.last()
```

### Selection

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `first` | First element | `[1,2,3] \| first` | `1` |
| `last` | Last element | `[1,2,3] \| last` | `3` |
| `slice(start, end?)` | Sub-array | `[1,2,3,4] \| slice(1, 3)` | `[2,3]` |
| `take(n)` | First n elements | `[1,2,3,4] \| take(2)` | `[1,2]` |
| `skip(n)` | Skip first n | `[1,2,3,4] \| skip(2)` | `[3,4]` |

### Transformation

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `map(expr)` | Transform each | `[{n:1},{n:2}] \| map(x => x.n)` | `[1,2]` |
| `map(field)` | Extract field | `[{n:1},{n:2}] \| map("n")` | `[1,2]` |
| `flatten` | Flatten one level | `[[1,2],[3,4]] \| flatten` | `[1,2,3,4]` |
| `reverse` | Reverse order | `[1,2,3] \| reverse` | `[3,2,1]` |
| `sort` | Sort ascending | `[3,1,2] \| sort` | `[1,2,3]` |
| `sort(field)` | Sort by field | `[{a:3},{a:1}] \| sort("a")` | `[{a:1},{a:3}]` |
| `unique` | Deduplicate | `[1,2,2,3] \| unique` | `[1,2,3]` |

### Filtering

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `filter(expr)` | Keep matching | `[1,2,3,4] \| filter(x => x > 2)` | `[3,4]` |
| `filter(field, value)` | Keep by field | `items \| filter("status", "active")` | `[...]` |
| `reject(expr)` | Remove matching | `[1,2,3,4] \| reject(x => x > 2)` | `[1,2]` |

### Aggregation

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `length` / `count` | Count elements | `[1,2,3] \| length` | `3` |
| `sum` / `sum(field)` | Sum numbers | `[1,2,3] \| sum` | `6` |
| `min` / `min(field)` | Minimum | `[3,1,2] \| min` | `1` |
| `max` / `max(field)` | Maximum | `[3,1,2] \| max` | `3` |
| `avg` / `avg(field)` | Average | `[1,2,3] \| avg` | `2` |

### Testing

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `any(expr)` | True if any match | `[1,2,3] \| any(x => x > 2)` | `true` |
| `all(expr)` | True if all match | `[1,2,3] \| all(x => x > 0)` | `true` |
| `none(expr)` | True if none match | `[1,2,3] \| none(x => x > 5)` | `true` |
| `contains(value)` | Value exists | `[1,2,3] \| contains(2)` | `true` |
| `isEmpty` | True if empty | `[] \| isEmpty` | `true` |

### Joining / Combining

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `join(sep)` | Join to string | `["a","b","c"] \| join(", ")` | `"a, b, c"` |
| `concat(other)` | Concatenate arrays | `[1,2] \| concat([3,4])` | `[1,2,3,4]` |
| `groupBy(field)` | Group into object | `[{t:"a",v:1},{t:"b",v:2}] \| groupBy("t")` | `{"a":[...],"b":[...]}` |

### Lambda Syntax

```
x => x.score > 0.5          -- single parameter
x => x.status == "active"   -- comparison
x => x.name                 -- field extraction
```

---

## String Operations

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `length` | String length | `"hello" \| length` | `5` |
| `trim` | Remove whitespace | `" hi " \| trim` | `"hi"` |
| `upper` / `lower` | Case conversion | `"hello" \| upper` | `"HELLO"` |
| `contains(sub)` | Contains substring | `"hello" \| contains("ell")` | `true` |
| `startsWith` / `endsWith` | Prefix/suffix check | `"hello" \| startsWith("hel")` | `true` |
| `replace(old, new)` | Replace first | `"aaa" \| replace("a", "b")` | `"baa"` |
| `replaceAll(old, new)` | Replace all | `"aaa" \| replaceAll("a", "b")` | `"bbb"` |
| `split(sep)` | Split to array | `"a,b,c" \| split(",")` | `["a","b","c"]` |
| `truncate(n)` | Truncate with `...` | `"hello world" \| truncate(5)` | `"hello..."` |
| `matches(regex)` | Regex match | `"abc123" \| matches("[0-9]+")` | `true` |

---

## Object Operations

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `keys` | Get keys | `{a:1, b:2} \| keys` | `["a","b"]` |
| `values` | Get values | `{a:1, b:2} \| values` | `[1,2]` |
| `has(key)` | Check key exists | `input.data \| has("name")` | `true` |
| `merge(other)` | Merge objects | `{a:1} \| merge({b:2})` | `{a:1, b:2}` |
| `omit(keys...)` | Remove keys | `{a:1,b:2,c:3} \| omit("b","c")` | `{a:1}` |
| `pick(keys...)` | Keep only keys | `{a:1,b:2,c:3} \| pick("a","b")` | `{a:1,b:2}` |

---

## Type Operations

| Operation | Description | Example | Result |
|-----------|-------------|---------|--------|
| `type` | Get type name | `42 \| type` | `"integer"` |
| `toInt` / `toFloat` / `toString` / `toBool` | Type conversion | `"42" \| toInt` | `42` |
| `json` | Serialize to JSON | `{a:1} \| json` | `'{"a":1}'` |
| `parseJson` | Parse JSON string | `'{"a":1}' \| parseJson` | `{a:1}` |
| `round(n?)` / `ceil` / `floor` / `abs` | Numeric operations | `3.456 \| round(2)` | `3.46` |

---

## Template Syntax

Templates render strings with embedded expressions. Used in prompt templates and some node configs.

### Interpolation

```
Hello, {{input.name}}!
Your score is {{input.score | round(2)}}.
```

### Conditional Blocks

```
{{#if input.history | length > 0}}
Previous conversation:
{{input.history | map("content") | join("\n")}}
{{#else}}
No previous conversation.
{{/if}}
```

### Iteration Blocks

```
{{#each state.findings}}
- Finding {{@index}}: {{this.title}} (confidence: {{this.confidence}})
{{/each}}
```

Available inside `#each`: `{{this}}`, `{{@index}}`, `{{@first}}`, `{{@last}}`.

### Raw Output

```
{{raw}}This is not {{an expression}}{{/raw}}
```

---

## Error Handling in Expressions

- Accessing a missing field returns `null` (not an error)
- `null` propagates through operations: `null + 1` -> `null`
- Use `??` to provide defaults: `input.name ?? "anonymous"`
- Use `?.` to safely traverse: `input.user?.profile?.avatar`
- Division by zero returns `null`
- Type mismatches in comparisons return `false`

---

## Estimation Filters

### `tokenEstimate`

Estimates the token count of a string or array of message objects. Uses the approximation of characters / 4, which is a reasonable estimate for most English text and LLM tokenizers.

| Input | Output | Description |
|-------|--------|-------------|
| string | integer | Estimated token count for the string |
| array[Message] | integer | Estimated token count for all message content concatenated |

**Examples:**

```
"Hello, world!" | tokenEstimate              -- returns 3 (14 chars / 4)
state.messages | tokenEstimate               -- estimates tokens across all messages
state.history | tokenEstimate > 4000         -- check if conversation is getting long
```

**Use case:** Monitoring token usage in tool loop conversations. When using `messages_from_state` to maintain conversation history, `tokenEstimate` helps enforce context window limits in conditional logic or transform nodes.

---

## What Expressions Cannot Do

- No variable assignment
- No function definitions
- No loops (use array operations instead)
- No side effects (no HTTP calls, no state mutation, no I/O)
- No imports or external references
- No recursion

The expression language is pure: given the same input, it always produces the same output.

## See Also

- [Expressions Overview](../expressions/overview.md) -- user-facing expression reference
- [Data Model](data-model.md) -- where expressions are used in graph definitions
- [Graph Model](graph-model.md) -- expression contexts and node scheduling
- [Transform Node](../nodes/transform.md) -- using expressions in transform nodes
