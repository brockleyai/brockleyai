# Expression Filters

Complete reference for all pipe filters in Brockley's expression language. Filters are applied using the pipe operator (`|`) or method-style dot access.

```
input.items | length          -- pipe syntax
input.items.length()          -- method syntax (equivalent)
```

Filters that take arguments use parentheses:

```
input.items | filter(x => x.active)
input.name | truncate(50)
input.items | slice(0, 5)
```

For detailed documentation on specific categories, see:
- [Array Operations](arrays.md)
- [String Operations](strings.md)
- [Object Operations](objects.md)
- [Type Operations](type-ops.md)

---

## Array Filters

### `first`

Returns the first element of an array, or `null` if empty.

```
[10, 20, 30] | first           -- 10
[] | first                     -- null
```

### `last`

Returns the last element of an array, or `null` if empty.

```
[10, 20, 30] | last            -- 30
[] | last                      -- null
```

### `slice(start, end?)`

Returns a sub-array from index `start` (inclusive) to `end` (exclusive). `end` is optional.

```
[1, 2, 3, 4, 5] | slice(1, 3)  -- [2, 3]
[1, 2, 3, 4, 5] | slice(2)     -- [3, 4, 5]
```

### `map(fn)`

Transforms each element using a lambda or field name string.

```
input.users | map(u => u.name)          -- ["Alice", "Bob"]
input.users | map("name")               -- ["Alice", "Bob"] (field name shorthand)
input.nums | map(n => n * 2)            -- [2, 4, 6]
```

### `filter(fn)`

Returns elements for which the lambda returns a truthy value.

```
input.items | filter(i => i.active)
input.nums | filter(n => n > 10)
input.users | filter(u => u.role == "admin")
```

### `sort`

Sorts an array in ascending order. Numbers sort numerically, strings lexicographically. Does not modify the original array.

```
[3, 1, 2] | sort              -- [1, 2, 3]
["banana", "apple"] | sort    -- ["apple", "banana"]
```

### `reverse`

Reverses the order of elements.

```
[1, 2, 3] | reverse           -- [3, 2, 1]
```

### `flatten`

Flattens one level of nested arrays.

```
[[1, 2], [3, 4], [5]] | flatten   -- [1, 2, 3, 4, 5]
[1, [2, 3], 4] | flatten          -- [1, 2, 3, 4]
```

### `unique`

Removes duplicate values, preserving first occurrence order.

```
[1, 2, 2, 3, 1] | unique      -- [1, 2, 3]
["a", "b", "a"] | unique      -- ["a", "b"]
```

### `join(separator)`

Joins array elements into a string with the given separator.

```
["a", "b", "c"] | join(", ")  -- "a, b, c"
[1, 2, 3] | join("-")         -- "1-2-3"
["hello"] | join(", ")        -- "hello"
```

### `concat(other)`

Concatenates another array onto the end.

```
[1, 2] | concat([3, 4])       -- [1, 2, 3, 4]
```

### `sum`

Returns the sum of all numeric elements. Non-numeric elements are skipped.

```
[1, 2, 3, 4] | sum            -- 10
[1.5, 2.5] | sum              -- 4.0
```

### `min`

Returns the smallest numeric value in the array.

```
[5, 3, 8, 1] | min            -- 1
```

### `max`

Returns the largest numeric value in the array.

```
[5, 3, 8, 1] | max            -- 8
```

### `contains(value)`

Returns `true` if the array contains the given value using equality comparison.

```
[1, 2, 3] | contains(2)       -- true
["a", "b"] | contains("c")    -- false
```

### `isEmpty`

Returns `true` if the value is empty, null, false, or zero.

```
[] | isEmpty                   -- true
[1] | isEmpty                  -- false
"" | isEmpty                   -- true
null | isEmpty                 -- true
0 | isEmpty                    -- true
```

### `length`

Returns the number of elements in an array (or characters in a string).

```
[1, 2, 3] | length            -- 3
"hello" | length               -- 5
null | length                  -- 0
```

---

## String Filters

### `upper`

Converts a string to uppercase.

```
"hello" | upper                -- "HELLO"
```

### `lower`

Converts a string to lowercase.

```
"HELLO" | lower                -- "hello"
```

### `trim`

Removes leading and trailing whitespace.

```
"  hello  " | trim             -- "hello"
```

### `contains(substring)`

Returns `true` if the string contains the given substring.

```
"hello world" | contains("world")   -- true
"hello" | contains("xyz")           -- false
```

### `split(separator)`

Splits a string into an array by the given separator.

```
"a,b,c" | split(",")          -- ["a", "b", "c"]
"hello" | split("")            -- ["h", "e", "l", "l", "o"]
```

### `replace(old, new)`

Replaces the first occurrence of `old` with `new`.

```
"hello world" | replace("world", "there")   -- "hello there"
```

### `replaceAll(old, new)`

Replaces all occurrences of `old` with `new`.

```
"a-b-c" | replaceAll("-", "_")              -- "a_b_c"
```

### `truncate(maxLength)`

Truncates the string to `maxLength` characters and appends `...` if truncated.

```
"hello world" | truncate(5)    -- "hello..."
"hi" | truncate(5)             -- "hi" (no truncation needed)
```

### `length`

Returns the number of characters in the string.

```
"hello" | length               -- 5
```

---

## Object Filters

### `keys`

Returns an array of the object's keys, sorted alphabetically.

```
{b: 2, a: 1, c: 3} | keys    -- ["a", "b", "c"]
```

### `values`

Returns an array of the object's values, ordered by sorted keys.

```
{b: 2, a: 1, c: 3} | values   -- [1, 2, 3]
```

### `has(key)`

Returns `true` if the object contains the given key.

```
{name: "Alice"} | has("name")  -- true
{name: "Alice"} | has("age")   -- false
```

---

## Type Conversion Filters

### `toInt`

Converts a value to an integer.

```
"42" | toInt                   -- 42
3.7 | toInt                    -- 3 (truncates)
true | toInt                   -- 1
false | toInt                  -- 0
```

### `toFloat`

Converts a value to a floating-point number.

```
"3.14" | toFloat               -- 3.14
42 | toFloat                   -- 42.0
```

### `toString`

Converts a value to its string representation.

```
42 | toString                  -- "42"
true | toString                -- "true"
null | toString                -- ""
[1, 2] | toString              -- "[1,2]" (JSON representation)
```

### `toBool`

Converts a value to a boolean using [truthiness rules](operators.md#truthiness-rules).

```
"hello" | toBool               -- true
"" | toBool                    -- false
0 | toBool                     -- false
42 | toBool                    -- true
null | toBool                  -- false
```

### `json`

Serializes a value to a JSON string.

```
{name: "Alice"} | json         -- '{"name":"Alice"}'
[1, 2, 3] | json               -- '[1,2,3]'
```

### `round(decimals?)`

Rounds a number to the specified number of decimal places. Defaults to 0 (rounds to integer).

```
3.14159 | round                -- 3
3.14159 | round(2)             -- 3.14
2.5 | round                    -- 3
```

### `default(fallback)`

Returns the fallback value if the input is `null`. Unlike `??`, this is a pipe filter for use in chains.

```
null | default("N/A")          -- "N/A"
"hello" | default("N/A")      -- "hello"
input.value | default(0)       -- 0 if input.value is null
```

### `tokenEstimate`

Estimates token count of a string or message array. Uses characters / 4 approximation.

```
"Hello, world!" | tokenEstimate              -- 3 (14 chars / 4)
state.messages | tokenEstimate               -- token estimate across all messages
state.history | tokenEstimate > 4000         -- check if context is getting long
```

For message arrays, it concatenates the `content` field of each message before estimating.

---

## See Also

- [Expression Language Overview](overview.md) -- namespaces, contexts, quick examples
- [Operators](operators.md) -- comparison, logical, arithmetic, null handling
- [Arrays](arrays.md) -- detailed array operations
- [Strings](strings.md) -- detailed string operations
- [Objects](objects.md) -- detailed object operations
- [Type Operations](type-ops.md) -- type conversion and numeric operations
- [Templates](templates.md) -- template block directives
