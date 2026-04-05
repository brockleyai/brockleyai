# Expression Language

Brockley's expression language lets you reference data, transform values, and build dynamic templates throughout your graphs -- in prompts, conditionals, transforms, and edge mappings. For a conceptual introduction, see [Expressions](../concepts/expressions.md).

## Reading order

Start with the overview, then the template syntax, then reference the operators and functions you need.

1. **[Overview](overview.md)** -- Where expressions are used, available namespaces (`input`, `state`, `output`), and how evaluation works.

2. **[Templates](templates.md)** -- Prompt template syntax: `{{expression}}` interpolation, conditionals, and loops inside templates.

3. **[Operators](operators.md)** -- Arithmetic, comparison, logical, and assignment operators.

4. **[Filters](filters.md)** -- Built-in filter functions: `upper`, `lower`, `length`, `default`, `join`, and more.

5. **[Strings](strings.md)** -- String operations: concatenation, slicing, replacement, and formatting.

6. **[Arrays](arrays.md)** -- Array operations: indexing, slicing, iteration, and aggregation.

7. **[Objects](objects.md)** -- Object property access, nested paths, and dynamic keys.

8. **[Type Operations](type-ops.md)** -- Type checking and conversion: `int`, `float`, `string`, `bool`, and type guards.

## Where to go next

- **[Transform Node](../nodes/transform.md)** -- The node type where expressions are used most heavily.
- **[LLM Node](../nodes/llm.md)** -- Prompt templates use expression interpolation.
- **[Conditional Node](../nodes/conditional.md)** -- Branch conditions are expressions.
