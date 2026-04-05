# Template Syntax

Templates render strings with embedded expressions. They are used in LLM node prompts (`system_prompt`, `user_prompt`, and `messages[].content`) and in some other node configs that support interpolation.

## Interpolation

Embed any [expression](overview.md) inside `{{ }}` delimiters:

```
Hello, {{input.name}}!
Your score is {{input.score | round(2)}}.
The execution ID is {{meta.execution_id}}.
```

Expressions inside `{{ }}` have access to all three namespaces: `input`, `state`, and `meta`. They support the full expression language -- operators, pipe filters, ternary, null coalescing, etc.

```
Status: {{input.status | upper}}
Count: {{input.items | length}}
Display name: {{input.nickname ?? input.full_name ?? "Anonymous"}}
Summary: {{input.verbose ? input.full_text : input.short_text}}
```

If an expression evaluates to `null`, the template inserts an empty string at that position.

## Conditional Blocks: `#if` / `#else`

Conditionally include text based on an expression:

```
{{#if input.include_context}}
Context: {{input.context}}
{{/if}}
```

With an else branch:

```
{{#if input.format == "detailed"}}
Please provide a detailed analysis with examples and citations.
{{#else}}
Please provide a brief summary in 2-3 sentences.
{{/if}}
```

The condition is evaluated using [truthiness rules](operators.md#truthiness-rules): non-null, non-empty, non-zero values are truthy.

### Nested conditions

```
{{#if input.items | length > 0}}
Processing {{input.items | length}} items.
{{#if input.items | length > 100}}
Warning: large batch. This may take a while.
{{/if}}
{{#else}}
No items to process.
{{/if}}
```

### Conditions with complex expressions

```
{{#if input.score > 0.8 && input.confidence > 0.9}}
High confidence result.
{{/if}}

{{#if state.history | length > 0}}
Previous conversation:
{{state.history | map("content") | join("\n")}}
{{/if}}
```

## Iteration Blocks: `#each`

Iterate over an array, rendering the block once per element:

```
{{#each input.items}}
- {{this.name}}: {{this.description}}
{{/each}}
```

### Available variables inside `#each`

| Variable | Type | Description |
|----------|------|-------------|
| `this` | any | The current item in the array. |
| `@index` | integer | Zero-based index of the current item. |
| `@first` | boolean | `true` if this is the first item. |
| `@last` | boolean | `true` if this is the last item. |

### Examples

**Numbered list:**

```
{{#each input.tasks}}
{{@index}}. {{this.title}} ({{this.priority}})
{{/each}}
```

Output:

```
0. Fix login bug (high)
1. Update docs (medium)
2. Add tests (low)
```

**Comma-separated with special handling for first/last:**

```
{{#each input.tags}}{{#if !@first}}, {{/if}}{{this}}{{/each}}
```

Output: `python, javascript, go`

**With conditional inside:**

```
{{#each state.findings}}
- Finding {{@index}}: {{this.title}} (confidence: {{this.confidence | round(2)}})
{{#if this.confidence < 0.5}}  [LOW CONFIDENCE]{{/if}}
{{/each}}
```

**Iterating over object values:**

First use `keys` or `values` to convert to an array:

```
{{#each input.config | keys}}
- {{this}}: {{input.config[this]}}
{{/each}}
```

## Raw Blocks

Prevent expression evaluation inside the block. Useful for including literal `{{` syntax in the output:

```
{{raw}}
Template syntax uses {{double braces}} for expressions.
These are NOT evaluated.
{{/raw}}
```

Everything between `{{raw}}` and `{{/raw}}` is output as-is, with no expression processing.

### Use case: teaching the LLM about templates

```
{{raw}}
When writing templates, use {{input.variable_name}} to reference input values.
Use {{#if condition}} for conditional blocks.
{{/raw}}
```

## Nesting

Block directives can be nested:

```
{{#if input.sections | length > 0}}
## Sections

{{#each input.sections}}
### {{this.title}}

{{this.content}}

{{#if this.subsections | length > 0}}
{{#each this.subsections}}
#### {{this.name}}
{{this.body}}
{{/each}}
{{/if}}

{{/each}}
{{#else}}
No sections provided.
{{/if}}
```

## Whitespace Handling

Templates preserve whitespace exactly as written, including newlines within and around block directives. This means:

```
{{#if input.verbose}}
Extra detail here.
{{/if}}
```

produces a blank line before "Extra detail here." when the condition is true. To produce tighter output, put the directive and text on the same line:

```
{{#if input.verbose}}Extra detail here.{{/if}}
```

## Common Patterns

### Conditional context injection

```
{{#if state.conversation_history | length > 0}}
Previous conversation:
{{#each state.conversation_history}}
{{this.role}}: {{this.content}}
{{/each}}

Now respond to the following:
{{/if}}
{{input.query}}
```

### Dynamic instructions based on input

```
Analyze the following {{input.content_type}}:

{{input.content}}

{{#if input.output_format == "json"}}
Respond with valid JSON.
{{#else}}
Respond in plain text.
{{/if}}

{{#if input.language}}
Respond in {{input.language}}.
{{/if}}
```

### Building structured prompts

```
You are a {{input.role ?? "helpful assistant"}}.

{{#if input.system_preamble}}
{{input.system_preamble}}
{{/if}}

Your task: {{input.task}}

{{#if input.examples | length > 0}}
## Examples

{{#each input.examples}}
Input: {{this.input}}
Output: {{this.output}}
{{/each}}
{{/if}}
```

## See Also

- [Expression Language Overview](overview.md) -- namespaces, operators, quick examples
- [Operators](operators.md) -- comparison and logical operators used in `#if` conditions
- [Arrays](arrays.md) -- array operations used in `#each` blocks
- [Filters](filters.md) -- pipe filters used in interpolation
- [LLM Node](../nodes/llm.md) -- where templates are used in prompts
