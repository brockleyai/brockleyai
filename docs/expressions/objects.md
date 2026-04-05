# Object Operations

Complete reference for object operations in Brockley's [expression language](overview.md).

## Field Access

### Dot Notation

Access object fields with `.`:

```
input.user.name                      -- "Alice"
input.response.data.items            -- nested access
input.config.timeout                 -- field value
```

Returns `null` if the object is `null` or the field does not exist.

### Bracket Notation

Use brackets for fields with special characters:

```
input.data["field-name"]             -- hyphenated field
input.headers["Content-Type"]        -- header name
input.data["123"]                    -- numeric-looking key
```

### Optional Chaining

Use `?.` to safely access fields that may be `null`:

```
input.user?.address?.city            -- null if user or address is null
input.response?.data                 -- null if response is null
```

## Inspection

### `keys`

Returns an array of the object's keys, sorted alphabetically.

```
{b: 2, a: 1, c: 3} | keys          -- ["a", "b", "c"]
{name: "Alice", age: 30} | keys     -- ["age", "name"]
{} | keys                           -- []
```

### `values`

Returns an array of the object's values, ordered by sorted keys.

```
{b: 2, a: 1, c: 3} | values        -- [1, 2, 3]
{name: "Alice", age: 30} | values   -- [30, "Alice"]
```

### `has(key)`

Returns `true` if the object contains the given key.

```
{name: "Alice", age: 30} | has("name")   -- true
{name: "Alice", age: 30} | has("email")  -- false
```

## Construction

### Object Literals

Build objects inline with `{key: value}` syntax:

```
{name: input.user.name, count: input.items | length}
{status: "active", timestamp: meta.execution_id}
{result: input.score > 0.5 ? "pass" : "fail"}
{}
```

Keys can be unquoted identifiers or quoted strings. Values are any expression.

### Merging

#### `merge(other)` (spec)

Shallow-merges another object into the current one. Keys from `other` overwrite keys in the original.

```
{a: 1, b: 2} | merge({b: 3, c: 4})   -- {a: 1, b: 3, c: 4}
{} | merge({name: "Alice"})            -- {name: "Alice"}
{x: 1} | merge({})                     -- {x: 1}
```

## Filtering

### `omit(keys...)` (spec)

Returns a new object with the specified keys removed.

```
{a: 1, b: 2, c: 3} | omit("b", "c")     -- {a: 1}
{name: "Alice", secret: "xyz"} | omit("secret")  -- {name: "Alice"}
```

### `pick(keys...)` (spec)

Returns a new object with only the specified keys.

```
{a: 1, b: 2, c: 3} | pick("a", "b")     -- {a: 1, b: 2}
{name: "Alice", age: 30, email: "a@b.c"} | pick("name", "email")
-- {name: "Alice", email: "a@b.c"}
```

## Common Patterns

### Reshape data for downstream nodes

```
{
  user: input.raw_data.user_info.name,
  email: input.raw_data.user_info.email | lower,
  item_count: input.raw_data.orders | length
}
```

### Merge config with defaults

```
{timeout: 30, retries: 3} | merge(input.user_config)
```

### Strip sensitive fields before logging

```
input.request | omit("api_key", "password", "token")
```

### Extract subset for an API call

```
input.full_record | pick("id", "name", "email")
```

### Safe nested access with defaults

```
input.config?.database?.host ?? "localhost"
input.user?.preferences?.theme ?? "light"
```

### Build response objects

```
{
  success: true,
  data: input.result,
  metadata: {
    execution: meta.execution_id,
    node: meta.node_name,
    items_processed: input.items | length
  }
}
```

## See Also

- [Expression Language Overview](overview.md) -- namespaces and contexts
- [Operators](operators.md) -- optional chaining, null coalescing
- [Filters Reference](filters.md) -- complete filter listing
- [Arrays](arrays.md) -- array operations (often combined with object access)
- [Type Operations](type-ops.md) -- `json` and `parseJson` for serialization
- [Transform Node](../nodes/transform.md) -- using expressions in graphs
