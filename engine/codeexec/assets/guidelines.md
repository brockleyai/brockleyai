# Code Execution Guidelines

You have access to a Python 3 execution environment via `_code_execute`. Use it for:
- **Data transformation**: reshaping, filtering, or aggregating data
- **Computation**: math, string manipulation, algorithm execution
- **Large output assembly**: building documents, reports, or structured data
- **Batching tool calls**: calling multiple tools in a loop efficiently

## The `brockley` Module

Your code has access to a built-in `brockley` module:

### `brockley.output(value)`
Set the structured result that will be returned to you. Use this for results you need to see.
- `value` can be any JSON-serializable Python object (dict, list, string, number, etc.)
- Only the last call to `brockley.output()` is kept.
- This is the primary way to return data from code execution.

### `brockley.tools.call(name, **kwargs)`
Call an approved tool synchronously from code.
- Returns the tool result as a string.
- Raises `brockley.ToolError` if the call fails.
- Only tools listed in the "Available Tools" section below can be called.
- Tools requiring approval cannot be called from code.

Example:
```python
result = brockley.tools.call("echo", message="hello")
```

### `brockley.tools.batch(calls)`
Call multiple tools and collect results. Each entry is `(name, kwargs_dict)`.
Returns a list of `{"result": ..., "error": ...}` dicts.

Example:
```python
results = brockley.tools.batch([
    ("echo", {"message": "first"}),
    ("echo", {"message": "second"}),
])
```

## Important Rules

1. **Use `brockley.output()` for results** the LLM should see. `print()` output is captured but truncated.
2. **Only Python stdlib** is available. No `pip install`, no external packages.
3. **No network access.** Use `brockley.tools.call()` to interact with external services.
4. **Timeouts apply.** Long-running code will be terminated.
5. **Output size limits apply.** Keep results concise. Use `brockley.output()` for structured data.
6. **Errors are returned** as structured results -- you can see them and retry with corrected code.
