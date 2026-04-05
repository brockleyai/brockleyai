# Terraform Provider

Manage Brockley graphs as infrastructure-as-code using Terraform or OpenTofu. Plan, apply, import, and destroy graphs with the same workflow you use for cloud infrastructure.

## Reading order

1. **[Provider Overview](overview.md)** -- Installation, provider configuration, and how graph resources map to Terraform state.

2. **[Resources](resources.md)** -- The `brockley_graph` resource: define graphs in HCL, manage lifecycle through Terraform.

3. **[Data Sources](data-sources.md)** -- Query existing graphs from Terraform configurations.

4. **[Importing Resources](import.md)** -- Bring existing graphs (created via CLI or API) into Terraform state.

5. **[Examples](examples.md)** -- Complete Terraform configurations for common patterns.

## Where to go next

- **[Deployment](../deployment/)** -- Deploy the Brockley platform itself (the Terraform provider manages graphs, not the platform).
- **[CLI Reference](../cli/)** -- Alternative interface for graph management.
- **[CI/CD](../cicd/)** -- Run Terraform in automated pipelines.
