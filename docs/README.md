# Brockley Documentation

Documentation for building, deploying, and operating AI agent workflows with Brockley. These docs are organized by what you're trying to do -- start at the top if you're new, or jump to the section you need.

---

## Start here

If you're new to Brockley, the **[Getting Started](getting-started/)** section walks you from zero to a running graph in about five minutes. It covers what Brockley is, a hands-on quickstart, building your first graph from scratch, and an architecture overview of how the pieces fit together.

## Learn the model

The **[Core Concepts](concepts/)** section explains the foundational model: graphs, nodes, edges, typed ports, state, execution, and the expression language. Understanding these makes everything else click. If you're working with autonomous agents, it also covers the superagent architecture.

## Build workflows

Three sections work together here:

- **[Node Types](nodes/)** -- The complete reference for every built-in node type. Each page covers configuration fields, port schemas, behavior, and examples. Start here when you need to know exactly how a node works.

- **[Guides](guides/)** -- Hands-on tutorials that show nodes working together in real patterns: tool calling, API tools, building your first agent, customizing superagent behavior, and multi-agent workflows.

- **[Expression Language](expressions/)** -- The full reference for Brockley's expression language: template syntax, operators, filters, string/array/object operations, and type conversions. You'll use expressions in prompts, conditionals, transforms, and edge mappings.

## Connect LLM providers

The **[LLM Providers](providers/)** section covers how to configure LLM access. It includes a comparison of all supported providers (OpenAI, Anthropic, Google, OpenRouter, AWS Bedrock), per-provider setup guides, and documentation for building custom providers.

## Manage and automate

Brockley offers multiple peer interfaces for managing graphs and executions. Pick the one that fits your workflow:

- **[CLI Reference](cli/)** -- The `brockley` command-line tool for validation, deployment, invocation, and inspection. Also the primary interface for CI/CD pipelines.

- **[REST API](api/)** -- Programmatic access to all platform capabilities. Everything the CLI and web UI can do is available through this API.

- **[Terraform Provider](terraform/)** -- Manage graphs as infrastructure-as-code. Plan, apply, import, and destroy with the same workflow you use for cloud infrastructure.

- **[CI/CD](cicd/)** -- Integrate Brockley into your pipelines. Validate graphs on every PR, deploy on merge. Guides for GitHub Actions, GitLab CI, and generic CI platforms.

## Deploy and operate

The **[Deployment](deployment/)** section covers running Brockley from local development through production cloud. It includes Docker Compose setup, Kubernetes with Helm, cloud-specific Terraform modules for AWS, GCP, and Azure, configuration reference, secrets management, and monitoring with Prometheus, OpenTelemetry, and trace exporters.

## Get help

The **[Troubleshooting](troubleshooting/)** section covers common error messages and their fixes, plus a general FAQ about Brockley's capabilities, licensing, and compatibility.

## Contribute

- **[Contributing](contributing/)** -- Development setup, testing requirements, code style, and PR guidelines.
- **[Internal Specifications](specs/)** -- Technical architecture, data model, graph model, expression language specification, and API design documents. These are the authoritative reference for how Brockley is built, intended for contributors and maintainers.
