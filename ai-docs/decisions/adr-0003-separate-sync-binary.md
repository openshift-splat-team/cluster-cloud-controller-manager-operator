# ADR-0003: Separate config-sync-controllers Binary

**Status**: Accepted
**Date**: 2022-06-01
**Deciders**: CCCMO maintainers
**Component**: Cluster Cloud Controller Manager Operator (CCCMO)

## Context

CCCMO has three reconcilers with distinct responsibilities:
1. `CloudOperatorReconciler` — deploys CCM manifests, manages ClusterOperator status. Requires a webhook server (port 9443) for admission control and metrics (port 9258).
2. `CloudConfigReconciler` — syncs cloud-conf ConfigMap. Only needs cluster API access, no webhook.
3. `TrustedCABundleReconciler` — merges CA bundles. Only needs cluster API access and filesystem access to system trust bundle.

The question was whether to run all three in a single process or split them.

**Scope**: This ADR is CCCMO-specific. For cross-repo decisions, see [Tier 1 ADRs](https://github.com/openshift/enhancements/tree/master/ai-docs/decisions).

## Decision

Split `CloudConfigReconciler` and `TrustedCABundleReconciler` into a dedicated `config-sync-controllers` binary, while `CloudOperatorReconciler` runs in the main operator binary. Both binaries run as separate containers within the **same pod** in the CCCMO Deployment. They share:
- The same `openshift-cloud-controller-manager-operator` namespace
- The same host-network pod (required for bootstrapping CCM before CNI is ready)
- The same ServiceAccount and RBAC

Each binary uses independent leader election:
- Main operator: `cluster-cloud-controller-manager-leader`
- Config sync: `cluster-cloud-config-sync-leader`

## Rationale

1. **Webhook isolation**: The main operator binary hosts the webhook server (port 9443). Config sync controllers don't need a webhook and having them in the same process would conflate health and restart semantics.
2. **Restart independence**: A config transformer bug or CA bundle failure shouldn't restart the main CCM asset reconciler, and vice versa.
3. **Independent leader election**: The config sync controllers can independently re-elect leadership without disrupting CCM asset management.
4. **Condition-based coordination**: `CloudOperatorReconciler` gates on `CloudConfigControllerAvailable` and `TrustedCABundleControllerAvailable` conditions set by the sync binary — this is a clean interface between the two processes.

## Consequences

### Positive
- Clear separation between CCM asset management (webhook-bearing) and config management (file + API only)
- Failure of one binary surfaces as a ClusterOperator condition without killing the other
- The `config-sync-controllers` binary has a simpler dependency graph (no webhook, no admission registration)

### Negative
- Two binaries to build, test, and deploy from the same repo
- Pod-level health depends on both containers; a CrashLoopBackOff in either marks the pod unhealthy
- The condition-based gate means CCM assets won't be deployed if config-sync controllers crash, which is the correct safety behavior but adds operational complexity when debugging

### Neutral
- Both binaries use the same `pkg/controllers` package; the split is at the `cmd/` level only
- The `manifests/0000_26_cloud-controller-manager-operator_11_deployment.yaml` defines both containers in the same pod spec

## Alternatives Considered

### Alternative 1: Single binary, all three reconcilers
**Description**: Run all controllers in one process with one leader election lock.
**Rejected because**: Webhook server lifecycle and config-sync lifecycle are independent. A webhook cert rotation or webhook failure shouldn't disrupt config sync.

### Alternative 2: Separate Deployments
**Description**: Run config sync as a completely separate Deployment with its own pod.
**Rejected because**: Would complicate the RBAC model, add another resource to manage via CVO, and prevent sharing the bootstrapping tolerations (e.g., `node.cloudprovider.kubernetes.io/uninitialized`).

## References

- `cmd/config-sync-controllers/main.go` — config sync binary entrypoint
- `cmd/cluster-cloud-controller-manager-operator/main.go` — main operator entrypoint
- `manifests/0000_26_cloud-controller-manager-operator_11_deployment.yaml` — pod spec with both containers
- `pkg/controllers/status.go` — condition names used for cross-binary coordination
- [ADR-0002: Cloud Config Sync](./adr-0002-cloud-config-sync.md)
- [Tier 1 ADRs](https://github.com/openshift/enhancements/tree/master/ai-docs/decisions)
