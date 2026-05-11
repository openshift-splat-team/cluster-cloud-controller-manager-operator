# ADR-0002: Cloud Config Sync via ConfigMap Transformation

**Status**: Accepted
**Date**: 2022-01-01
**Deciders**: CCCMO maintainers
**Component**: Cluster Cloud Controller Manager Operator (CCCMO)

## Context

Each cloud provider CCM requires a `cloud.conf` (or equivalent) configuration file mounted into CCM pods. In an OpenShift cluster, this config originates from different sources depending on the platform:

- For AWS and Azure: the Cloud Credentials Operator (CCO) manages the cloud config and writes it to `openshift-config-managed/kube-cloud-config`.
- For OpenStack, vSphere, and others: the config is in `openshift-config/<name>` as referenced by `Infrastructure.Spec.CloudConfig`.

Additionally, each provider may need to transform or augment the config before CCM can consume it (e.g., injecting CA bundles, normalizing INI sections, adding network CIDR hints).

The question was: where does transformation happen, and who is responsible for writing the final CCM-ready config?

**Scope**: This ADR is CCCMO-specific. For cross-repo decisions, see [Tier 1 ADRs](https://github.com/openshift/enhancements/tree/master/ai-docs/decisions).

## Decision

CCCMO owns cloud config transformation and writes the CCM-ready config to `openshift-cloud-controller-manager/cloud-conf`. A dedicated `CloudConfigReconciler` (running in the `config-sync-controllers` binary):

1. Reads the source config from either `openshift-config-managed/kube-cloud-config` (AWS/Azure, CCO-managed) or `openshift-config/<name>` (other platforms).
2. Applies a per-platform `cloudConfigTransformer` function.
3. Writes the result to the `cloud-conf` ConfigMap in the CCM namespace.

The transformer signature is: `func(source string, infra *configv1.Infrastructure, network *configv1.Network, features featuregates.FeatureGate) (string, error)`.

## Rationale

1. **Separation of concerns**: CCO manages cloud credentials; CCCMO manages cloud controller deployment and config. Each operator owns its domain.
2. **Auditability**: All config transformations are explicit Go functions with clear inputs and outputs, unit-testable in isolation.
3. **Network context**: Some transformers need `Network/cluster` (CIDR, MTU) to configure the CCM correctly — CCCMO already has access to this.
4. **Feature gate integration**: Transformers receive the `FeatureGateAccess` interface, allowing platform-specific behavior to be gated behind OpenShift feature flags.

## Consequences

### Positive
- Cloud config transformation logic is centralized, versioned, and testable
- Adding a new provider config transformer requires only implementing the function signature and registering in `cloud.go`
- The `cloud-conf` ConfigMap in the CCM namespace is always the CCCMO-owned canonical version

### Negative
- CCCMO currently has a dependency on CCO output for AWS and Azure (reads `kube-cloud-config`). This creates an implicit ordering requirement: CCO must run before CCCMO can sync config for these platforms.
- The `needsManagedConfigLookup` boolean return from `GetCloudConfigTransformer` is a code smell that will be cleaned up when AWS/Azure transformers are fully implemented in CCCMO (see FIXME comments in `cloud.go`)

### Neutral
- `openstack.CloudConfigTransformer` parses INI format using `gopkg.in/ini.v1`; this is OpenStack-specific and not reused elsewhere

## Alternatives Considered

### Alternative 1: Mount source config directly into CCM pods
**Description**: Mount `openshift-config/<name>` directly into CCM pods without transformation.
**Rejected because**: Providers need to augment the config (CA bundles, extra sections). Also, different platforms use different source namespaces, making a unified mount pattern impractical.

### Alternative 2: Cloud providers transform their own config at startup
**Description**: Each CCM binary reads the raw config and transforms it internally.
**Rejected because**: Requires forking upstream CCM binaries to add OpenShift-specific transformation logic, creating a larger maintenance burden.

## References

- `pkg/controllers/cloud_config_sync_controller.go` — `CloudConfigReconciler`
- `pkg/cloud/cloud.go` — `GetCloudConfigTransformer()`
- `pkg/cloud/openstack/` — example transformer using INI parsing
- [ADR-0003: Separate Sync Binary](./adr-0003-separate-sync-binary.md)
- [Tier 1 ADRs](https://github.com/openshift/enhancements/tree/master/ai-docs/decisions)
