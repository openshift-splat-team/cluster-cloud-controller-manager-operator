# Tier 1 Ecosystem References

This document links to generic OpenShift/Kubernetes patterns in the Tier 1 ecosystem hub. CCCMO inherits these platform-wide patterns and practices.

## Operator Patterns

**Location**: [ai-docs/platform/operator-patterns/](https://github.com/openshift/enhancements/tree/master/ai-docs/platform/operator-patterns)

- **Controller Runtime**: Reconciliation loops, event handling, client patterns
- **Status Conditions**: Available, Progressing, Degraded condition semantics
- **Webhooks**: Validation and mutation patterns
- **Finalizers**: Resource cleanup patterns
- **RBAC**: Service account and permissions

**CCCMO Usage**:
- `CloudOperatorReconciler` follows the standard reconcile loop: read Infrastructure → compose config → get resources → server-side apply → set ClusterOperator conditions
- ClusterOperator conditions (`Available`, `Degraded`, `Progressing`) are managed via `ClusterOperatorStatusClient` in `pkg/controllers/status.go`
- Webhook server runs on port 9443; cert rotation via `controller-runtime-common/pkg/tls`

## Testing Practices

**Location**: [ai-docs/practices/testing/](https://github.com/openshift/enhancements/tree/master/ai-docs/practices/testing)

- **Test Pyramid**: Unit > Integration > E2E ratio (60/30/10)
- **E2E Framework**: OpenShift E2E test patterns

**CCCMO Usage**:
- See [CCCMO_TESTING.md](../CCCMO_TESTING.md) for component-specific test suites
- Unit tests use `envtest` via controller-runtime's `SetupWithManager` test helpers
- E2E tests live in `cmd/cloud-controller-manager-aws-tests-ext/e2e/` for AWS

## Security Practices

**Location**: [ai-docs/practices/security/](https://github.com/openshift/enhancements/tree/master/ai-docs/practices/security)

- **Threat Modeling**: See Tier 1 security practices for threat modeling framework
- **RBAC Guidelines**: Role and ClusterRole design

**CCCMO Usage**:
- CCCMO RBAC defined in `manifests/0000_26_..._02_rbac_operator.yaml` and `03_rbac_provider.yaml`
- Per-platform `CredentialsRequest` objects created for cloud provider credentials (Azure, GCP, IBM, OpenStack, PowerVS, vSphere, Nutanix)
- `NetworkPolicy` default-deny applied to `openshift-cloud-controller-manager-operator` namespace; explicit egress/ingress only

## Reliability Practices

**Location**: [ai-docs/practices/reliability/](https://github.com/openshift/enhancements/tree/master/ai-docs/practices/reliability)

- **SLO Framework**: Service Level Objectives and error budgets
- **Observability**: Metrics, logging, tracing patterns

**CCCMO Usage**:
- Metrics exposed on `:9258` (operator) and leader election health on `:9259`/`:9260`
- `PodDisruptionBudgets` created for CCM Deployments on multi-replica clusters (MinAvailable=1)
- Leader election with 137s lease, 107s renew deadline, 26s retry period

## Kubernetes Fundamentals

**Location**: [ai-docs/domain/kubernetes/](https://github.com/openshift/enhancements/tree/master/ai-docs/domain/kubernetes)

- **Pod**: Pod lifecycle, container specs
- **CRDs**: CustomResourceDefinition patterns
- **ConfigMaps**: Configuration distribution

**CCCMO Usage**:
- CCCMO does not define CRDs; it manages Deployments, RBAC, PDBs, and Services
- ConfigMaps are the primary data exchange mechanism (cloud-conf, ccm-trusted-ca, cloud-controller-manager-images)

## OpenShift Fundamentals

**Location**: [ai-docs/domain/openshift/](https://github.com/openshift/enhancements/tree/master/ai-docs/domain/openshift)

- **ClusterOperator**: Cluster operator status reporting
- **ClusterVersion**: Platform upgrade orchestration
- **Infrastructure**: Cluster platform detection

**CCCMO Usage**:
- `ClusterOperator/cloud-controller-manager` is the primary status surface
- CVO delivers CCCMO via `include.release.openshift.io/self-managed-high-availability` and `single-node-developer` annotations
- `Infrastructure/cluster` is the primary config input (see [domain/infrastructure-platform.md](../domain/infrastructure-platform.md))

## Cross-Repository ADRs

**Location**: [ai-docs/decisions/](https://github.com/openshift/enhancements/tree/master/ai-docs/decisions)

Platform-wide architectural decisions relevant to CCCMO:
- **CVO Orchestration**: Why CVO orchestrates upgrades — CCCMO is delivered and managed by CVO
- **Immutable Nodes**: Why RHCOS + rpm-ostree — affects how CCM node initialization works
- **External Cloud Providers (KEP-2395)**: The upstream motivation for CCCMO's existence

**Component-Specific ADRs**: See [ai-docs/decisions/](../decisions/) for CCCMO-specific decisions:
- [ADR-0001: Embedded Manifests](../decisions/adr-0001-embedded-manifests.md)
- [ADR-0002: Cloud Config Sync](../decisions/adr-0002-cloud-config-sync.md)
- [ADR-0003: Separate Sync Binary](../decisions/adr-0003-separate-sync-binary.md)

## Upstream References

| Project | Relevance |
|---------|-----------|
| [kubernetes/cloud-provider-aws](https://github.com/openshift/cloud-provider-aws) | AWS CCM binary managed by CCCMO |
| [kubernetes/cloud-provider-azure](https://github.com/openshift/cloud-provider-azure) | Azure CCM + Cloud Node Manager managed by CCCMO |
| [kubernetes/cloud-provider-gcp](https://github.com/openshift/cloud-provider-gcp) | GCP CCM managed by CCCMO |
| [kubernetes/cloud-provider-openstack](https://github.com/openshift/cloud-provider-openstack) | OpenStack CCM managed by CCCMO |
| [kubernetes/cloud-provider-vsphere](https://github.com/openshift/cloud-provider-vsphere) | vSphere CCM managed by CCCMO |
| [KEP-2395](https://github.com/kubernetes/enhancements/tree/master/keps/sig-cloud-provider/2395-removing-in-tree-cloud-providers) | The Kubernetes enhancement driving CCCMO's existence |

---

**Note**: These links point to Tier 1 (ecosystem hub) documentation. Component-specific patterns and decisions are documented in the `ai-docs/` directory of this repository.

**Last Updated**: 2026-05-11
