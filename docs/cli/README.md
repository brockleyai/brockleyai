# CLI Reference

The `brockley` CLI lets you validate, deploy, invoke, and inspect graphs from the command line. It's also the primary tool for CI/CD pipelines.

## Reading order

Start with setup, then learn the commands in the order you'll typically use them.

1. **[CLI Overview](overview.md)** -- Installation (from source or binary), connection configuration, and a quick tour of all commands.

2. **[Authentication](auth.md)** -- Set up API keys and server URLs for local and remote Brockley instances.

3. **[Deploy](deploy.md)** -- Push graphs to a Brockley server. Supports single files, directories, and environment variable resolution for secrets.

4. **[Validate](validate.md)** -- Run 13 structural and type-safety checks locally with zero network calls. Use this in CI on every PR.

5. **[Invoke](invoke.md)** -- Run a graph execution from the command line, in synchronous or asynchronous mode.

6. **[List](list.md)** -- List graphs, executions, and other resources on the server.

7. **[Inspect](inspect.md)** -- View execution details: step-by-step results, timing, and status.

8. **[Export](export.md)** -- Export a graph to Terraform HCL for infrastructure-as-code workflows.

9. **[Multi-File Graphs](multi-file.md)** -- Compose graphs across multiple JSON files for better organization in large projects.

## Where to go next

- **[REST API](../api/)** -- The same operations available programmatically.
- **[Terraform Provider](../terraform/)** -- Manage graphs as infrastructure-as-code.
- **[CI/CD](../cicd/)** -- Use the CLI in automated pipelines.
