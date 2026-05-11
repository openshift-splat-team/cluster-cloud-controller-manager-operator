# CCCMO Architecture: Components

> **Generic Patterns**: See [Tier 1 Operator Patterns](https://github.com/openshift/enhancements/tree/master/ai-docs/platform/operator-patterns) for controller-runtime reconcile loop, ClusterOperator status conditions, and RBAC conventions.

## Package Layout

```text
cmd/
├── cluster-cloud-controller-manager-operator/  # Main operator binary
│   └── main.go                                 # CloudOperatorReconciler wiring, webhook server
├── config-sync-controllers/                    # Separate binary for config sync
│   └── main.go                                 # CloudConfigReconciler + TrustedCABundleReconciler
├── azure-config-credentials-injector/          # Azure credential injection sidecar
│   └── main.go                                 # Cobra CLI, merges cloud.conf with Workload Identity creds
└── cloud-controller-manager-aws-tests-ext/     # AWS E2E test extension binary
    └── e2e/

pkg/
├── cloud/                   # Platform detection and asset rendering
│   ├── cloud.go             # Master switch: getAssetsConstructor, GetCloudConfigTransformer
│   ├── errors.go            # platformNotFoundError type
│   ├── common/              # Shared rendering utilities
│   │   ├── interfaces.go    # CloudProviderAssets interface
│   │   ├── resources.go     # PodDisruptionBudget, Service creation
│   │   ├── substitution.go  # Proxy env-var injection, TLS propagation
│   │   ├── templates.go     # ReadTemplates, RenderTemplates (text/template)
│   │   └── config.go        # NoOpTransformer
│   ├── aws/                 # AWS assets + config transformer
│   ├── azure/               # Azure assets + config transformer
│   ├── azurestack/          # Azure Stack Hub assets + config transformer
│   ├── gcp/                 # GCP assets
│   ├── ibm/                 # IBM Cloud assets
│   ├── nutanix/             # Nutanix assets
│   ├── openstack/           # OpenStack assets + INI config transformer
│   ├── powervs/             # PowerVS assets
│   └── vsphere/             # vSphere assets + config transformer + webhook
├── config/
│   └── config.go            # OperatorConfig struct, ComposeConfig(), ImagesReference
├── controllers/
│   ├── clusteroperator_controller.go     # CloudOperatorReconciler
│   ├── cloud_config_sync_controller.go   # CloudConfigReconciler
│   ├── trusted_ca_bundle_controller.go   # TrustedCABundleReconciler
│   ├── status.go                         # ClusterOperatorStatusClient (shared status helpers)
│   ├── common_consts.go                  # Namespace constants, ConfigMap names
│   ├── cache.go                          # ObjectWatcher (dynamic resource watching)
│   ├── watch_predicates.go               # Event filtering predicates
│   └── resourceapply/                    # Server-side apply wrappers
├── restmapper/              # REST mapper utilities for config-sync binary
├── tls/                     # TLS cert watcher integration
└── util/                    # Logging, image URL parsing, testingutils

manifests/                   # CVO-delivered manifests (RBAC, Deployment, CredentialsRequests)
test/manifest/               # Test manifests for unit tests
```

## Controller Wiring

### Main Operator Binary (port 9443 webhook, :9258 metrics)

```
main.go
  └── CloudOperatorReconciler
        Watches: Infrastructure, Proxy, ConfigMap (images), ObjectWatcher events
        RBAC: ClusterOperator CRUD, Infrastructure get, Proxy get,
              Deployments/DaemonSets/RBAC in openshift-cloud-controller-manager
        Owns: ClusterOperator/cloud-controller-manager status conditions
        Emits: CloudControllerOwnershipCondition, OperatorAvailable/Degraded
```

### Config-Sync Binary (no webhook, :9440 health)

```
config-sync-controllers/main.go
  ├── CloudConfigReconciler
  │     Watches: Infrastructure, Network, ConfigMap (kube-cloud-config), ConfigMap (cloud-conf)
  │     Writes: openshift-cloud-controller-manager/cloud-conf
  │     Conditions: CloudConfigControllerAvailable/Degraded
  └── TrustedCABundleReconciler
        Watches: Proxy, ConfigMap (trusted CA in openshift-config)
        Writes: openshift-cloud-controller-manager/ccm-trusted-ca
        Conditions: TrustedCABundleControllerAvailable/Degraded
```

## Data Flow

### CCM Asset Deployment (CloudOperatorReconciler)

```
Infrastructure CR (Platform type, topology)
  │
  ├─→ config.ComposeConfig()         → OperatorConfig
  │       reads: Infrastructure, Proxy, images.json, FeatureGates, TLS config
  │
  ├─→ cloud.GetResources(config)     → []client.Object
  │       calls getAssetsConstructor → NewProviderAssets(config)
  │           reads embedded YAML templates
  │           renders with text/template + govalidator
  │       calls GetCommonResources   → PDB + Service
  │       calls SubstituteCommonPartsFromConfig → injects proxy env, replica count, TLS
  │
  └─→ resourceapply.Apply(objects)   → server-side apply each object to cluster
```

### Cloud Config Sync (CloudConfigReconciler)

```
Infrastructure.Spec.CloudConfig → source ConfigMap reference in openshift-config
Infrastructure.Status.PlatformStatus → GetCloudConfigTransformer()
Network CR → passed to transformer for CIDR/MTU context

If needsManagedConfigLookup (AWS/Azure):
  Read openshift-config-managed/kube-cloud-config (CCO output)
Else:
  Read openshift-config/<spec.cloudConfig.name>

Apply transformer(sourceData, infra, network, featureGates) → transformed data
Write openshift-cloud-controller-manager/cloud-conf
```

### Trusted CA Bundle (TrustedCABundleReconciler)

```
Proxy.Spec.TrustedCA.Name → ConfigMap name in openshift-config
Read that ConfigMap → custom CA bundle
Read /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem → system trust bundle
Merge both → write openshift-cloud-controller-manager/ccm-trusted-ca (ca-bundle.crt)
```

## Key Patterns

### Embedded Manifests
Each provider package uses Go `embed.FS` to include YAML manifests at compile time. `common.ReadTemplates` parses them into `text/template.Template` objects. `common.RenderTemplates` executes them with `TemplateValues` and decodes the output into `client.Object` using the Kubernetes codec.

```go
//go:embed assets/*
var assetsFs embed.FS

templates = []common.TemplateSource{
    {ReferenceObject: &appsv1.Deployment{}, EmbedFsPath: "assets/deployment.yaml"},
}
```

### govalidator for Template Values
Before rendering, each provider validates required fields using `govalidator.ValidateMap` against a `templateValuesValidationMap`. This catches missing image references at startup rather than silently deploying malformed manifests.

### ObjectWatcher for Dynamic Resources
`CloudOperatorReconciler` uses `ObjectWatcher` to watch resources it creates dynamically (CCM Deployments). When a watched resource changes, a `GenericEvent` triggers re-reconciliation. This avoids listing all Deployments cluster-wide.

### Provisioning Gate
Before applying CCM manifests, `CloudOperatorReconciler.provisioningAllowed()` checks:
1. Sub-controller conditions (`CloudConfigControllerAvailable`, `TrustedCABundleControllerAvailable`)
2. Platform is not `External`
3. `ExternalCloudProvider` is enabled (or cluster already owned by CCM)
This prevents CCM deployment on platforms that haven't completed their cloud-config handoff.

### Proxy Propagation
`common.SubstituteCommonPartsFromConfig` injects `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY` env vars into all CCM pod containers, reading from `OperatorConfig.ClusterProxy` (populated from `Proxy/cluster`).

### TLS Adherence
`OperatorConfig.TLSCipherSuites` and `TLSMinVersion` are derived from the cluster `APIServer` TLS profile and passed through to CCM `--tls-cipher-suites` and `--tls-min-version` flags via template rendering.

## Deployment Topology

The CCCMO Deployment runs on master nodes (`nodeSelector: node-role.kubernetes.io/master`), uses host network, and tolerates `node.cloudprovider.kubernetes.io/uninitialized` — which is the taint that uninitialized cloud nodes carry before CCM removes it. This ensures CCCMO itself can schedule even before CCM has initialized node objects.

The Deployment runs **two containers** in the same pod:
- `cluster-cloud-controller-manager` (main operator)
- `config-sync-controllers` (config sync binary)
