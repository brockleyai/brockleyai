# Type Operations

Complete reference for type inspection, conversion, and numeric operations in Brockley's [expression language](overview.md).

## Type Inspection

### `type` (spec)

Returns the type name of a value as a string.

```
42 | type                      -- "integer"
3.14 | type                    -- "float"
"hello" | type                 -- "string"
true | type                    -- "boolean"
null | type                    -- "null"
[1, 2] | type                 -- "array"
{a: 1} | type                 -- "object"
```

## Type Conversion

### `toInt`

Converts a value to an integer. Floats are truncated (not rounded).

```
"42" | toInt                   -- 42
"-7" | toInt                   -- -7
3.7 | toInt                    -- 3
3.2 | toInt                    -- 3
true | toInt                   -- 1
false | toInt                  -- 0
```

### `toFloat`

Converts a value to a floating-point number.

```
"3.14" | toFloat               -- 3.14
"42" | toFloat                 -- 42.0
42 | toFloat                   -- 42.0
```

### `toString`

Converts a value to its string representation.

```
42 | toString                  -- "42"
3.14 | toString                -- "3.14"
true | toString                -- "true"
false | toString               -- "false"
null | toString                -- ""
[1, 2] | toString              -- "[1,2]" (JSON)
{a: 1} | toString              -- '{"a":1}' (JSON)
```

### `toBool`

Converts a value to a boolean using [truthiness rules](operators.md#truthiness-rules).

```
"hello" | toBool               -- true
"" | toBool                    -- false
42 | toBool                    -- true
0 | toBool                     -- false
null | toBool                  -- false
[] | toBool                    -- false
[1] | toBool                   -- true
{} | toBool                    -- true
```

## JSON Serialization

### `json`

Serializes any value to a JSON string.

```
{name: "Alice", age: 30} | json     -- '{"age":30,"name":"Alice"}'
[1, 2, 3] | json                    -- '[1,2,3]'
"hello" | json                      -- '"hello"'
42 | json                           -- '42'
null | json                         -- 'null'
```

Useful for embedding structured data in prompts:

```
Analyze this data: {{input.payload | json}}
```

### `parseJson` (spec)

Parses a JSON string back into a value.

```
'{"a": 1}' | parseJson              -- {a: 1}
'[1, 2, 3]' | parseJson             -- [1, 2, 3]
'"hello"' | parseJson               -- "hello"
```

Useful when an upstream node returns JSON as a string:

```
input.json_string | parseJson | keys
input.api_response | parseJson | map(x => x.name)
```

## Numeric Operations

### `round(decimals?)`

Rounds a number to the specified decimal places. Defaults to 0 (rounds to nearest integer).

```
3.14159 | round                -- 3
3.14159 | round(2)             -- 3.14
3.14159 | round(4)             -- 3.1416
2.5 | round                   -- 3
-2.5 | round                  -- -3
100.0 | round                 -- 100
```

### `ceil` (spec)

Rounds up to the nearest integer.

```
3.2 | ceil                     -- 4
3.0 | ceil                     -- 3
-3.2 | ceil                    -- -3
```

### `floor` (spec)

Rounds down to the nearest integer.

```
3.8 | floor                    -- 3
3.0 | floor                    -- 3
-3.2 | floor                   -- -4
```

### `abs` (spec)

Returns the absolute value.

```
-5 | abs                       -- 5
5 | abs                        -- 5
-3.14 | abs                    -- 3.14
0 | abs                        -- 0
```

## Utility Filters

### `default(fallback)`

Returns the fallback value if the input is `null`. Unlike the `??` operator, `default` is a pipe filter for use in chains.

```
null | default("N/A")          -- "N/A"
"hello" | default("N/A")      -- "hello"
0 | default(42)                -- 0 (0 is not null)
"" | default("empty")         -- "" (empty string is not null)
```

### `tokenEstimate`

Estimates the token count of a string or array of messages. Uses the approximation of characters / 4, which is reasonable for most English text and LLM tokenizers.

For strings:

```
"Hello, world!" | tokenEstimate              -- 3 (14 chars / 4)
```

For message arrays (concatenates `content` fields):

```
state.messages | tokenEstimate               -- token estimate for conversation
state.history | tokenEstimate > 4000         -- check if context is getting long
```

Useful for monitoring token usage in tool loop conversations or deciding when to compact context.

## Common Patterns

### Safe numeric formatting

```
input.price | toFloat | round(2) | toString
input.percentage | round(1) | toString + "%"
```

### String-to-number conversion for comparisons

```
input.score_string | toFloat > 0.5
input.count_string | toInt >= 10
```

### JSON round-trip

```
input.data | json | length           -- approximate byte size
input.json_text | parseJson | keys   -- inspect JSON structure
```

### Type-safe defaults

```
input.count | toInt ?? 0
input.threshold | toFloat ?? 0.5
input.enabled | toBool ?? false
```

### Token budget checking

```
state.conversation | tokenEstimate > 100000 ? "compact" : "continue"
```

## See Also

- [Expression Language Overview](overview.md) -- namespaces and contexts
- [Operators](operators.md) -- arithmetic operators, null coalescing
- [Filters Reference](filters.md) -- complete filter listing
- [Arrays](arrays.md) -- `sum`, `min`, `max`, `avg` for numeric arrays
- [Strings](strings.md) -- `toString` conversions
- [Transform Node](../nodes/transform.md) -- using expressions in graphs
