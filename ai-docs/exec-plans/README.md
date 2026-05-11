# Execution Plans

> **Exec-Plans Guidance**: See [Tier 1 Exec-Plans Guide](https://github.com/openshift/enhancements/tree/master/ai-docs/workflows/exec-plans/) for:
> - What are exec-plans?
> - When to create them
> - How to use them
> - Completion workflow
> - Template

## Component-Specific Exec-Plans

Active exec-plans for CCCMO are tracked in `active/`:

```text
exec-plans/
├── active/              # Create feature-specific exec-plans here
└── completed/           # Completed exec-plans for historical reference
```

## Usage

```bash
# Get template from Tier 1
curl -O https://raw.githubusercontent.com/openshift/enhancements/master/ai-docs/workflows/exec-plans/template.md

# Create exec-plan
mv template.md active/feature-name.md

# Fill in and track during implementation
```

## CCCMO-Specific Exec-Plan Notes

When creating exec-plans for CCCMO, consider:

- **New platform support**: Requires changes in `pkg/cloud/cloud.go`, new `pkg/cloud/<provider>/` package, and a `manifests/0000_26_..._credentialsrequest-<provider>.yaml`. See [cloud-provider-integration.md](../../docs/dev/cloud-provider-integration.md).
- **Cloud config transformer changes**: Update `GetCloudConfigTransformer` in `pkg/cloud/cloud.go` and the relevant transformer function with unit tests.
- **TLS or security changes**: May affect all provider Deployment templates; plan for template updates across all providers.
- **Feature gates**: New behavior behind feature gates requires updating `OperatorConfig.FeatureGates` propagation through templates.

See [Tier 1 Exec-Plans Guide](https://github.com/openshift/enhancements/tree/master/ai-docs/workflows/exec-plans/README.md) for complete documentation.
