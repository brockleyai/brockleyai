# Terraform Import Guide

All Brockley Terraform resources support importing existing resources into Terraform state.

## Import Commands

```bash
# Import a graph
terraform import brockley_graph.my_graph graph_abc123

# Import a schema
terraform import brockley_schema.my_schema schema_001

# Import a prompt template
terraform import brockley_prompt_template.my_template pt_001

# Import a provider config
terraform import brockley_provider_config.my_config pc_001
```

## Workflow

1. Create a resource block in your `.tf` file with the desired resource name
2. Run `terraform import` with the resource address and the Brockley resource ID
3. Run `terraform plan` to verify the imported state matches your configuration
4. Adjust your `.tf` file as needed to match the imported state

## Example

```hcl
# 1. Add resource block
resource "brockley_graph" "imported" {
  name  = "customer-classifier"
  nodes = jsonencode([])
  edges = jsonencode([])
}
```

```bash
# 2. Import
terraform import brockley_graph.imported graph_abc123

# 3. Verify
terraform plan
```

## Import API Tool

```bash
terraform import brockley_api_tool.my_tool at_001
```

## See Also

- [Terraform Overview](overview.md) -- provider setup
- [Resources](resources.md) -- all resource types
- [Examples](examples.md) -- complete configurations
