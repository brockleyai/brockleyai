# Expression Operators

Complete reference for all operators in Brockley's expression language.

## Comparison Operators

Compare two values. Returns a boolean.

| Operator | Name | Example | Result |
|----------|------|---------|--------|
| `==` | Equal | `input.status == "active"` | `true` if equal |
| `!=` | Not equal | `input.count != 0` | `true` if not equal |
| `>` | Greater than | `input.score > 0.5` | `true` if left > right |
| `<` | Less than | `input.age < 18` | `true` if left < right |
| `>=` | Greater or equal | `input.score >= 0.8` | `true` if left >= right |
| `<=` | Less or equal | `input.retries <= 3` | `true` if left <= right |

Numeric comparison works across integer and float types. String comparison uses lexicographic order.

```
42 == 42.0             -- true (cross-type numeric comparison)
"abc" < "def"          -- true (lexicographic)
null == null           -- true
null == false          -- false (null is only equal to null)
"5" == 5               -- false (no implicit coercion)
```

## Logical Operators

Combine boolean expressions. Both `&&` and `||` use short-circuit evaluation.

| Operator | Name | Example | Description |
|----------|------|---------|-------------|
| `&&` | AND | `input.a > 0 && input.b > 0` | Returns left if falsy, otherwise returns right |
| `\|\|` | OR | `input.a \|\| input.b` | Returns left if truthy, otherwise returns right |
| `!` | NOT | `!input.disabled` | Returns the logical negation |

Short-circuit behavior:

```
true && "hello"        -- "hello" (returns right operand)
false && "hello"       -- false (right not evaluated)
"" || "default"        -- "default" (empty string is falsy)
"value" || "default"   -- "value" (non-empty string is truthy)
!true                  -- false
!null                  -- true (null is falsy)
!""                    -- true (empty string is falsy)
```

### Truthiness Rules

| Value | Truthy? |
|-------|---------|
| `true` | Yes |
| `false` | No |
| Non-empty string (`"hello"`) | Yes |
| Empty string `""` | No |
| Non-zero number (`1`, `-3.14`) | Yes |
| `0` / `0.0` | No |
| Non-empty array (`[1, 2]`) | Yes |
| Empty array `[]` | No |
| Any object, including empty `{}` | Yes |
| `null` | No |

These rules apply to `&&`, `||`, `!`, ternary `? :`, `#if` blocks, and conditional node branch evaluation.

## Arithmetic Operators

| Operator | Name | Example | Result |
|----------|------|---------|--------|
| `+` | Add / Concat | `input.a + input.b` | Sum (numbers) or concatenation (strings) |
| `-` | Subtract | `input.total - input.discount` | Difference |
| `*` | Multiply | `input.price * input.qty` | Product |
| `/` | Divide | `input.total / input.count` | Quotient |
| `%` | Modulo | `input.index % 2` | Remainder |

**Type behavior:**

- When both operands are integers and the result is a whole number, an integer is returned.
- When either operand is a float, a float is returned.
- `+` with two strings concatenates them.
- Division by zero returns `null`.
- Modulo by zero returns `null`.
- `null` in any arithmetic operation returns `null`.

```
2 + 3                   -- 5 (integer)
10 / 3                  -- 3 (integer division, both ints)
10.0 / 3                -- 3.333... (float)
"hello" + " " + "world" -- "hello world"
7 % 3                   -- 1
null + 1                -- null
5 / 0                   -- null
```

## Null Handling Operators

### Null Coalesce (`??`)

Returns the left operand if it is not `null`, otherwise returns the right operand. Unlike `||`, this only falls through on `null`, not on other falsy values.

```
input.name ?? "Anonymous"       -- "Anonymous" if name is null
input.config?.timeout ?? 30     -- 30 if config or timeout is null
null ?? "fallback"              -- "fallback"
"value" ?? "fallback"           -- "value"
0 ?? "fallback"                 -- 0 (0 is not null)
"" ?? "fallback"                -- "" (empty string is not null)
false ?? "fallback"             -- false (false is not null)
```

### Optional Chaining (`?.`)

Accesses a property on an object, returning `null` if the object is `null` instead of causing an error.

```
input.user?.address?.city       -- null if user or address is null
input.response?.data            -- null if response is null
input.items?.[0]                -- null if items is null
```

Without optional chaining, accessing a property on `null` also returns `null` (the engine does not throw). However, `?.` makes the intent explicit and documents that the value may be absent.

## Ternary Operator (`? :`)

Evaluates the condition using [truthiness rules](#truthiness-rules). Returns the "then" value if truthy, the "else" value if falsy.

```
input.age >= 18 ? "adult" : "minor"
input.items | length > 0 ? input.items | first : null
```

Nesting is allowed but can hurt readability:

```
input.score > 0.8 ? "high" : input.score > 0.5 ? "medium" : "low"
```

## Property Access

### Dot Access (`.`)

```
input.user.name                 -- nested field
input.response.data.items       -- deep nesting
```

Returns `null` if the object is `null` or the property does not exist.

### Bracket Access (`[]`)

```
input.items[0]                  -- first element
input.items[-1]                 -- last element
input.items[-2]                 -- second to last
input.data["field-with-dashes"] -- bracket notation for special chars
```

Negative indices count from the end. Out-of-bounds indices return `null`.

### Method-Style Access

Filters can be called with dot notation:

```
input.items.length()            -- same as input.items | length
input.name.upper()              -- same as input.name | upper
```

## Pipe Syntax (`|`)

The pipe operator passes the left-hand value as the first argument to the right-hand filter:

```
input.items | length
input.items | filter(x => x.active) | map(x => x.name)
input.name | upper | trim
input.scores | sort | reverse | first
```

See the [Filters reference](filters.md) for all available filters.

## Lambda Expressions (`=>`)

Used as arguments to filter functions:

```
input.users | map(u => u.name)
input.items | filter(item => item.price > 100)
input.users | map(u => u.first_name + ' ' + u.last_name)
```

Lambdas create a new scope. The parameter name can be any valid identifier. The body is a full expression that can use all operators and access the outer context (`input`, `state`, `meta`).

## Array and Object Literals

### Arrays

```
[1, 2, 3]
["a", "b", "c"]
[input.x, input.y, input.z]
[]
```

### Objects

```
{name: "Alice", age: 30}
{key: input.value, count: input.items | length}
{result: input.score > 0.5 ? "pass" : "fail"}
{}
```

Object keys can be unquoted identifiers or quoted strings. Values are any expression.

## See Also

- [Expression Language Overview](overview.md) -- namespaces, contexts, quick examples
- [Filters Reference](filters.md) -- all pipe filters
- [Arrays](arrays.md) -- array operations
- [Strings](strings.md) -- string operations
- [Type Operations](type-ops.md) -- type conversion
- [Expression Language Spec](../specs/expression-language.md) -- complete specification
