# CI/CD

Integrate Brockley into your continuous integration and deployment pipelines. Validate graphs on every PR, deploy on merge, and keep your agent workflows as reliable as the rest of your codebase.

## Reading order

Start with the generic guide for concepts, then follow the guide for your platform.

1. **[Generic CI](generic-ci.md)** -- CI/CD concepts and patterns that apply to any platform: validation steps, deployment strategies, and environment variable management.

2. **Pick your platform:**
   - **[GitHub Actions](github-actions.md)** -- Ready-to-use workflow files for validating and deploying Brockley graphs in GitHub Actions.
   - **[GitLab CI](gitlab-ci.md)** -- Pipeline configuration for validating and deploying in GitLab CI.

## Where to go next

- **[CLI Reference](../cli/)** -- The CLI commands used in CI pipelines (`validate`, `deploy`).
- **[Deployment](../deployment/)** -- Where CI deploys to -- local, Kubernetes, or cloud.
- **[Terraform Provider](../terraform/)** -- Use Terraform in CI for declarative graph management.
