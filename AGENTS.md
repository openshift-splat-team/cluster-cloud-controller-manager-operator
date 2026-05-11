# Cluster Cloud Controller Manager Operator - Agentic Documentation

**Component**: Cluster Cloud Controller Manager Operator (CCCMO)
**Repository**: openshift-splat-team/cluster-cloud-controller-manager-operator
**Documentation Tier**: 2 (Component-specific)

> **Retrieval-First**: Read `ai-docs/` before writing code. Check `ai-docs/decisions/` before
> proposing architectural changes. Consult `ai-docs/domain/` for platform-specific resource behavior.

> **Generic Platform Patterns**: See [Tier 1 Ecosystem Hub](https://github.com/openshift/enhancements/tree/master/ai-docs) for operator patterns, testing practices, security guidelines, and cross-repo ADRs.

## What is CCCMO?

The Cluster Cloud Controller Manager Operator (CCCMO) manages and deploys cloud-provider-specific Cloud Controller Managers (CCMs) on OpenShift clusters. It reads `Infrastructure` and `Proxy` cluster config, selects the correct cloud provider implementation (AWS, Azure, GCP, OpenStack, vSphere, IBM, PowerVS, Nutanix), renders provider-specific manifests from embedded YAML templates, and applies them to the `openshift-cloud-controller-manager` namespace. The operator also syncs cloud configuration ConfigMaps and trusted CA bundles to CCM pods.

**Key Principle**: Platform detection at reconcile time — all provider selection uses a single `switch` on `Infrastructure.Status.PlatformStatus.Type`; adding a platform touches one file (`pkg/cloud/cloud.go`).

## Core Components

- **CloudOperatorReconciler** (`pkg/controllers/clusteroperator_controller.go`): Owns ClusterOperator status, applies CCM manifests, guards provisioning with feature-gate and ownership checks
- **CloudConfigReconciler** (`pkg/controllers/cloud_config_sync_controller.go`): Syncs cloud-conf ConfigMap from `openshift-config-managed` → `openshift-cloud-controller-manager`, applying platform-specific transformers
- **TrustedCABundleReconciler** (`pkg/controllers/trusted_ca_bundle_controller.go`): Merges system CA bundle with cluster proxy trusted CA into `ccm-trusted-ca` ConfigMap
- **CloudProviderAssets** (`pkg/cloud/`): Per-provider embedded manifest templates rendered at reconcile time
- **AzureConfigCredentialsInjector** (`cmd/azure-config-credentials-injector/`): Sidecar injecting Azure Workload Identity credentials into cloud.conf at pod start
- **ConfigSyncControllers** (`cmd/config-sync-controllers/`): Separate binary running CloudConfigReconciler + TrustedCABundleReconciler

**Quick Start**: `oc describe clusteroperator/cloud-controller-manager` | `oc get deploy -n openshift-cloud-controller-manager`

## Documentation Structure

```text
ai-docs/
├── domain/                          # Platform-level concepts consumed by CCCMO
│   └── infrastructure-platform.md   # Infrastructure CR and platform detection
├── architecture/
│   └── components.md                # Controller wiring, data flow, package layout
├── decisions/                       # CCCMO-specific ADRs
│   ├── adr-template.md
│   ├── adr-0001-embedded-manifests.md
│   ├── adr-0002-cloud-config-sync.md
│   └── adr-0003-separate-sync-binary.md
├── exec-plans/                      # Feature planning
│   └── active/
├── references/
│   └── ecosystem.md                 # Links to Tier 1
├── CCCMO_DEVELOPMENT.md             # Build, dev workflow, adding a platform
└── CCCMO_TESTING.md                 # Unit, E2E test locations and commands
```

## Knowledge Graph

```text
                         [AGENTS.md] ← Start here
                              │
              ┌───────────────┼───────────────┐
              │               │               │
   [domain/infrastructure]  [architecture/] [decisions/]
   Platform detection         Controller      Embed vs CRD,
   Infrastructure CR          wiring,         sync binary,
   Cloud config flow          data flow       CA bundle design
              │               │               │
              └───────────────┼───────────────┘
                              │
                    [references/ecosystem.md]
                    Tier 1 operator patterns,
                    testing, security links
```

## Platform / Driver Table

| Platform     | Provider Package          | Config Transformer     | Tested in CI |
|--------------|---------------------------|------------------------|--------------|
| AWS          | `pkg/cloud/aws`           | `aws.CloudConfigTransformer` (via CCO) | Yes |
| Azure        | `pkg/cloud/azure`         | `azure.CloudConfigTransformer` (via CCO) | Yes |
| Azure Stack  | `pkg/cloud/azurestack`    | `azurestack.CloudConfigTransformer` | No |
| GCP          | `pkg/cloud/gcp`           | `NoOpTransformer`      | Yes |
| OpenStack    | `pkg/cloud/openstack`     | `openstack.CloudConfigTransformer` | Yes |
| vSphere      | `pkg/cloud/vsphere`       | `vsphere.CloudConfigTransformer` | Yes |
| IBM          | `pkg/cloud/ibm`           | `NoOpTransformer`      | No |
| PowerVS      | `pkg/cloud/powervs`       | `NoOpTransformer`      | No |
| Nutanix      | `pkg/cloud/nutanix`       | `NoOpTransformer`      | Yes |

**AI Agent Path**: domain/ → architecture/ → decisions/ → CCCMO_DEVELOPMENT.md

## External References

- [Cloud Controller Manager KEP-2395](https://github.com/kubernetes/enhancements/tree/master/keps/sig-cloud-provider/2395-removing-in-tree-cloud-providers) | [OpenShift Enhancement #463](https://github.com/openshift/enhancements/pull/463/) | [Hacking Guide](./docs/dev/hacking-guide.md) | [New Provider Guide](./docs/dev/cloud-provider-integration.md)

---

**Tier 1 Hub**: https://github.com/openshift/enhancements/tree/master/ai-docs
