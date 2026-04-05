# String Operations

Complete reference for string operations in Brockley's [expression language](overview.md).

## Length

### `length`

Returns the number of characters in the string.

```
"hello" | length               -- 5
"" | length                    -- 0
null | length                  -- 0
```

## Whitespace

### `trim`

Removes leading and trailing whitespace.

```
"  hello  " | trim             -- "hello"
"\n\tdata\t\n" | trim          -- "data"
```

## Case Conversion

### `upper`

Converts the entire string to uppercase.

```
"hello" | upper                -- "HELLO"
"Hello World" | upper          -- "HELLO WORLD"
```

### `lower`

Converts the entire string to lowercase.

```
"HELLO" | lower                -- "hello"
"Hello World" | lower          -- "hello world"
```

## Searching

### `contains(substring)`

Returns `true` if the string contains the given substring.

```
"hello world" | contains("world")   -- true
"hello world" | contains("xyz")     -- false
"hello" | contains("")              -- true (empty string is always contained)
```

### `startsWith(prefix)` (spec)

Returns `true` if the string starts with the given prefix.

```
"hello world" | startsWith("hello") -- true
"hello world" | startsWith("world") -- false
```

### `endsWith(suffix)` (spec)

Returns `true` if the string ends with the given suffix.

```
"hello world" | endsWith("world")   -- true
"hello world" | endsWith("hello")   -- false
```

### `matches(regex)` (spec)

Returns `true` if the string matches the given regular expression.

```
"abc123" | matches("[0-9]+")        -- true
"hello" | matches("^[a-z]+$")      -- true
"Hello" | matches("^[a-z]+$")      -- false
```

## Splitting and Joining

### `split(separator)`

Splits the string into an array by the given separator.

```
"a,b,c" | split(",")               -- ["a", "b", "c"]
"hello world" | split(" ")         -- ["hello", "world"]
"hello" | split("")                -- ["h", "e", "l", "l", "o"]
"a::b::c" | split("::")            -- ["a", "b", "c"]
```

Commonly combined with array operations:

```
input.csv_line | split(",") | map(s => s | trim)
input.tags_string | split(",") | map(t => t | trim) | unique
```

## Replacement

### `replace(old, new)`

Replaces the first occurrence of `old` with `new`.

```
"hello world" | replace("world", "there")    -- "hello there"
"aaa" | replace("a", "b")                   -- "baa"
```

### `replaceAll(old, new)`

Replaces all occurrences of `old` with `new`.

```
"a-b-c" | replaceAll("-", "_")               -- "a_b_c"
"aaa" | replaceAll("a", "b")                -- "bbb"
"hello world world" | replaceAll("world", "earth") -- "hello earth earth"
```

## Truncation

### `truncate(maxLength)`

Truncates the string to `maxLength` characters and appends `...` if the string was longer.

```
"hello world" | truncate(5)         -- "hello..."
"hi" | truncate(5)                  -- "hi" (no truncation)
"exactly" | truncate(7)             -- "exactly" (no truncation)
```

## String Concatenation

Use the `+` operator to concatenate strings:

```
"hello" + " " + "world"             -- "hello world"
input.first_name + ' ' + input.last_name
"Count: " + (input.items | length | toString)
```

Non-string values must be converted with `toString` before concatenation.

## Common Patterns

### Clean and normalize input

```
input.email | lower | trim
input.name | trim | truncate(100)
```

### Build formatted strings

```
input.first_name + ' ' + input.last_name
"Item " + (input.index | toString) + ": " + input.name
```

### Extract parts of a string

```
input.full_name | split(" ") | first        -- first name
input.full_name | split(" ") | last         -- last name
input.email | split("@") | last             -- domain
```

### Clean CSV data

```
input.raw_tags | split(",") | map(t => t | trim | lower) | unique
```

### Safe truncation for display

```
input.description | truncate(200)
input.title ?? "Untitled" | truncate(50)
```

## See Also

- [Expression Language Overview](overview.md) -- namespaces and contexts
- [Filters Reference](filters.md) -- complete filter listing
- [Arrays](arrays.md) -- array operations (often combined with split)
- [Type Operations](type-ops.md) -- `toString` for type conversion
- [Templates](templates.md) -- string interpolation in prompts
- [Transform Node](../nodes/transform.md) -- using expressions in graphs
