# ADR-0001: Embedded Manifests with Go embed

**Status**: Accepted
**Date**: 2021-06-01
**Deciders**: CCCMO maintainers
**Component**: Cluster Cloud Controller Manager Operator (CCCMO)

## Context

CCCMO must deploy provider-specific Kubernetes resources (Deployments, RBAC, PodDisruptionBudgets, Services) for each supported cloud platform. The operator needs to track and reconcile these resources continuously, preventing unauthorized changes.

The design question was how to store and distribute the per-provider manifests: as files on disk at runtime, as CRDs with embedded spec, as Go struct literals, or embedded into the binary.

**Scope**: This ADR is CCCMO-specific. For general operator patterns, see [Tier 1 ADRs](https://github.com/openshift/enhancements/tree/master/ai-docs/decisions).

## Decision

Use Go 1.16+ `embed.FS` to bundle all provider YAML manifests directly into the CCCMO binary. Each provider package (`pkg/cloud/<provider>/`) contains an `assets/` directory of YAML templates, declared with `//go:embed assets/*`. Rendering uses `text/template` with per-provider `TemplateValues` maps.

## Rationale

1. **Single binary distribution**: CVO manages a single operator image. Embedding manifests means no separate ConfigMap or volume mount is needed for provider manifests.
2. **Version safety**: Provider manifests are always in sync with the operator version — no skew possible between the operator binary and its manifests.
3. **Go 1.16+ availability**: The feature became available before the project reached GA; adoption was low-risk.
4. **Template flexibility**: `text/template` allows image substitution and feature-gate flag injection without requiring full Go struct definitions for each provider resource.

## Consequences

### Positive
- Adding a new provider requires only creating `pkg/cloud/<provider>/assets/*.yaml` and registering in `cloud.go`
- Manifests are verified by `make generate verify` which runs `make manifests` and checks for drift
- No runtime file system dependency; operator can run read-only

### Negative
- Provider manifests cannot be updated without rebuilding and deploying a new operator image
- YAML templates must use `text/template` syntax, which is less expressive than Helm but sufficient for current needs
- govalidator is used as a validation layer; template rendering errors surface only at reconcile time on a live cluster

### Neutral
- The `image-references` manifest in `manifests/` feeds the images ConfigMap; updating images still requires a CVO payload update

## Alternatives Considered

### Alternative 1: CRD-defined per-provider resources
**Description**: Define a CRD for each provider's desired state; the operator reconciles from the CRD spec.
**Rejected because**: Too much schema overhead for what is essentially static deployment config. Adds API surface that users shouldn't need to modify.

### Alternative 2: ConfigMap-stored manifests
**Description**: Store YAML in a ConfigMap delivered by CVO, mount into the operator pod.
**Rejected because**: Creates version skew risk; ConfigMap and operator image could diverge during upgrades. Adds volume mount complexity.

### Alternative 3: Go struct literals for all provider resources
**Description**: Define all provider manifests as Go struct initializations.
**Rejected because**: Provider manifests are complex (multi-container Deployments, RBAC, PDB) and Go struct representation is harder to read and review than YAML.

## References

- `pkg/cloud/common/templates.go` — `ReadTemplates`, `RenderTemplates`
- `pkg/cloud/aws/aws.go` — example provider using `embed.FS`
- [Go embed documentation](https://pkg.go.dev/embed)
- [Tier 1 ADRs](https://github.com/openshift/enhancements/tree/master/ai-docs/decisions)
