# Array Operations

Complete reference for array operations in Brockley's [expression language](overview.md). Array operations use pipe syntax in templates and can also use method-style dot notation.

```
input.items | length              -- pipe syntax
input.items.length()              -- method syntax (equivalent)
```

## Selection

### `first`

Returns the first element, or `null` if empty.

```
[10, 20, 30] | first              -- 10
[] | first                        -- null
```

### `last`

Returns the last element, or `null` if empty.

```
[10, 20, 30] | last               -- 30
[] | last                         -- null
```

### `slice(start, end?)`

Returns a sub-array from `start` (inclusive) to `end` (exclusive). `end` is optional -- omitting it takes everything from `start` onward.

```
[1, 2, 3, 4, 5] | slice(1, 3)    -- [2, 3]
[1, 2, 3, 4, 5] | slice(2)       -- [3, 4, 5]
[1, 2, 3, 4, 5] | slice(0, 1)    -- [1]
```

### Array Indexing

Use bracket notation for individual elements. Negative indices count from the end.

```
input.items[0]                    -- first element
input.items[2]                    -- third element
input.items[-1]                   -- last element
input.items[-2]                   -- second to last
```

Out-of-bounds indices return `null`.

## Transformation

### `map(fn)`

Transforms each element using a lambda or field name string.

```
-- Lambda: full expression
[1, 2, 3] | map(x => x * 2)                         -- [2, 4, 6]
input.users | map(u => u.first + ' ' + u.last)       -- ["Alice Smith", "Bob Jones"]
input.items | map(i => {name: i.name, total: i.price * i.qty})

-- Field name shorthand
[{name: "Alice"}, {name: "Bob"}] | map("name")       -- ["Alice", "Bob"]
```

### `flatten`

Flattens one level of nested arrays.

```
[[1, 2], [3, 4], [5]] | flatten    -- [1, 2, 3, 4, 5]
[1, [2, 3], 4] | flatten           -- [1, 2, 3, 4]
[[1, [2]], [3]] | flatten           -- [1, [2], 3] (only one level)
```

### `reverse`

Reverses element order. Does not modify the original array.

```
[1, 2, 3] | reverse                -- [3, 2, 1]
["a", "c", "b"] | reverse          -- ["b", "c", "a"]
```

### `sort`

Sorts in ascending order. Numbers sort numerically, strings lexicographically. Does not modify the original array.

```
[3, 1, 2] | sort                   -- [1, 2, 3]
["banana", "apple", "cherry"] | sort -- ["apple", "banana", "cherry"]
```

### `unique`

Removes duplicate values, preserving first occurrence order.

```
[1, 2, 2, 3, 1] | unique           -- [1, 2, 3]
["a", "b", "a", "c"] | unique      -- ["a", "b", "c"]
```

### `concat(other)`

Concatenates another array onto the end.

```
[1, 2] | concat([3, 4])            -- [1, 2, 3, 4]
[] | concat([1])                    -- [1]
```

## Filtering

### `filter(fn)`

Returns elements for which the lambda returns a truthy value.

```
[1, 2, 3, 4, 5] | filter(x => x > 3)                -- [4, 5]
input.users | filter(u => u.active)                   -- active users only
input.items | filter(i => i.status == "pending")      -- pending items
input.items | filter(i => i.price > 0 && i.in_stock)  -- in-stock items with price
```

### `reject(fn)` (spec)

Returns elements for which the lambda returns a falsy value. The inverse of `filter`.

```
[1, 2, 3, 4] | reject(x => x > 2)                   -- [1, 2]
input.items | reject(i => i.deleted)                  -- non-deleted items
```

## Aggregation

### `length` / `count`

Returns the number of elements.

```
[1, 2, 3] | length                  -- 3
[] | length                         -- 0
null | length                       -- 0
```

### `sum` / `sum(field)` (spec)

Sum of numeric elements. Non-numeric elements are skipped.

```
[1, 2, 3] | sum                     -- 6
[1.5, 2.5, 3.0] | sum               -- 7.0
```

With a field name, sums that field from each element:

```
[{v: 10}, {v: 20}] | sum("v")       -- 30
```

### `min` / `min(field)` (spec)

Returns the smallest numeric value.

```
[5, 3, 8, 1] | min                  -- 1
```

With a field name:

```
[{score: 0.8}, {score: 0.3}] | min("score")  -- 0.3
```

### `max` / `max(field)` (spec)

Returns the largest numeric value.

```
[5, 3, 8, 1] | max                  -- 8
```

### `avg` / `avg(field)` (spec)

Returns the arithmetic mean.

```
[1, 2, 3] | avg                     -- 2
[10, 20, 30] | avg                  -- 20
```

## Testing

### `any(fn)` (spec)

Returns `true` if any element matches the lambda.

```
[1, 2, 3] | any(x => x > 2)         -- true
[1, 2, 3] | any(x => x > 5)         -- false
```

### `all(fn)` (spec)

Returns `true` if all elements match the lambda.

```
[1, 2, 3] | all(x => x > 0)         -- true
[1, 2, 3] | all(x => x > 2)         -- false
```

### `none(fn)` (spec)

Returns `true` if no elements match the lambda.

```
[1, 2, 3] | none(x => x > 5)        -- true
[1, 2, 3] | none(x => x > 2)        -- false
```

### `contains(value)`

Returns `true` if the array contains the value (using equality comparison).

```
[1, 2, 3] | contains(2)              -- true
["a", "b"] | contains("c")           -- false
```

### `isEmpty`

Returns `true` if the array is empty.

```
[] | isEmpty                          -- true
[1] | isEmpty                         -- false
```

## Joining and Grouping

### `join(separator)`

Joins elements into a string with the given separator.

```
["a", "b", "c"] | join(", ")         -- "a, b, c"
[1, 2, 3] | join("-")                -- "1-2-3"
[] | join(", ")                       -- ""
```

### `groupBy(field)` (spec)

Groups elements into an object keyed by the given field.

```
[
  {type: "a", val: 1},
  {type: "b", val: 2},
  {type: "a", val: 3}
] | groupBy("type")
-- {"a": [{type: "a", val: 1}, {type: "a", val: 3}], "b": [{type: "b", val: 2}]}
```

## Lambda Syntax

Lambdas use the `param => body` syntax:

```
x => x.score > 0.5                   -- comparison
x => x.status == "active"            -- equality check
x => x.name                          -- field extraction
x => x.price * x.quantity            -- arithmetic
x => x.first + ' ' + x.last         -- string concatenation
```

The parameter name can be any valid identifier. The body is a full expression with access to all operators, filters, and the outer context (`input`, `state`, `meta`).

## Common Patterns

### Filter then transform

```
input.users | filter(u => u.active) | map(u => u.email) | sort
```

### Flatten nested arrays

```
input.departments | map(d => d.employees) | flatten | filter(e => e.role == "engineer")
```

### Deduplicate and count

```
input.tags | unique | length
```

### Top-N items

```
input.scores | sort | reverse | slice(0, 3)
```

### Combine arrays

```
input.list_a | concat(input.list_b) | unique | sort
```

### Build CSV from objects

```
input.rows | map(r => [r.name, r.email, r.score | toString] | join(",")) | join("\n")
```

## See Also

- [Expression Language Overview](overview.md) -- namespaces and contexts
- [Filters Reference](filters.md) -- complete filter listing
- [Strings](strings.md) -- string operations
- [Objects](objects.md) -- object operations
- [Type Operations](type-ops.md) -- type conversion
- [Transform Node](../nodes/transform.md) -- using expressions in graphs
