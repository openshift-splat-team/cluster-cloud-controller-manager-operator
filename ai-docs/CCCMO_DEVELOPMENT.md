# CCCMO Development Guide

> **Generic Development Practices**: See [Tier 1 Development Practices](https://github.com/openshift/enhancements/tree/master/ai-docs/practices/development) for Go standards, controller-runtime patterns, and CI/CD workflows.

This guide covers **CCCMO-specific** development practices.

## Quick Start

### Prerequisites

- Go 1.25+ (see `go.mod`)
- GNU Make
- Access to OpenShift cluster (for on-cluster testing)
- `KUBECONFIG` environment variable set
- Podman or Docker (for container builds)

### Build Binaries

```bash
# Build all binaries (operator, config-sync, azure-injector, aws-tests-ext)
make build

# Build operator binary only
make operator

# Build config-sync binary only
make config-sync-controllers

# Build azure credentials injector only
make azure-config-credentials-injector

# Run with linting
make all      # same as: make build
```

**Binaries output**: `./bin/`
- `bin/cluster-controller-manager-operator`
- `bin/config-sync-controllers`
- `bin/azure-config-credentials-injector`

## Repository Structure

```text
cmd/
├── cluster-cloud-controller-manager-operator/  # Main operator binary
├── config-sync-controllers/                    # Cloud config + CA bundle sync binary
├── azure-config-credentials-injector/          # Azure Workload Identity credential injector
└── cloud-controller-manager-aws-tests-ext/     # AWS E2E test extension binary

pkg/
├── cloud/                   # Platform detection and asset rendering
│   ├── cloud.go             # Master switch for platform selection
│   ├── common/              # Shared template rendering, substitution helpers
│   ├── aws/ azure/ azurestack/ gcp/ ibm/ nutanix/ openstack/ powervs/ vsphere/
├── config/                  # OperatorConfig, ImagesReference, ComposeConfig
├── controllers/             # All three reconcilers + ObjectWatcher + status helpers
├── restmapper/              # REST mapper for config-sync binary
├── tls/                     # TLS cert watcher integration
└── util/                    # Logging, image URL utilities, test helpers

manifests/                   # CVO-delivered Kubernetes manifests
docs/dev/                    # Developer guides (hacking, cloud-provider-integration)
hack/                        # Build scripts, example images.json
test/manifest/               # Static manifests used by unit tests
```

## Development Workflow

### 1. Local Development

Run unit tests first:

```bash
make unit
```

Run full verification (generate + verify + manifests + unit):

```bash
make test
```

Lint and format:

```bash
make verify    # runs fmt + vet + lint
make fmt
make vet
make lint
```

### 2. Testing on Cluster (Hacking Mode)

Scale down CVO to prevent it from reverting your changes:

```bash
oc scale --replicas=0 deployment/cluster-version-operator -n openshift-cluster-version
```

Scale down CCCMO:

```bash
oc scale --replicas=0 deployment/cluster-cloud-controller-manager-operator -n openshift-cloud-controller-manager-operator
```

Build and run locally with a custom images file:

```bash
make build
# Edit hack/example-images.json to point to your CCM images
./bin/cluster-controller-manager-operator --images-json=hack/example-images.json
```

### 3. Testing with Custom Container Image

Build and push your image:

```bash
export IMG="quay.io/your-repo/cluster-cloud-controller-manager-operator:dev"
make image
podman push $IMG
```

Update the operator Deployment to use your image:

```bash
oc edit deployment/cluster-cloud-controller-manager-operator -n openshift-cloud-controller-manager-operator
# Change the image field for cluster-cloud-controller-manager container
```

Update the images ConfigMap if testing a custom CCM image:

```bash
oc edit configmap/cloud-controller-manager-images -n openshift-cloud-controller-manager-operator
```

Force CCM re-deployment (CCCMO will recreate it):

```bash
oc delete deployment/aws-cloud-controller-manager -n openshift-cloud-controller-manager
```

### 4. Viewing Logs

```bash
# Main operator logs
oc logs -f deployment/cluster-cloud-controller-manager-operator -n openshift-cloud-controller-manager-operator -c cluster-cloud-controller-manager

# Config sync logs
oc logs -f deployment/cluster-cloud-controller-manager-operator -n openshift-cloud-controller-manager-operator -c config-sync-controllers

# CCM logs (e.g., AWS)
oc logs -f deployment/aws-cloud-controller-manager -n openshift-cloud-controller-manager
```

## Common Tasks

### Add Support for a New Cloud Platform

See [cloud-provider-integration.md](../docs/dev/cloud-provider-integration.md) for the full guide. Summary:

1. Create `pkg/cloud/<provider>/` with:
   - `<provider>.go` — `NewProviderAssets()` constructor using `embed.FS`
   - `assets/` — YAML templates for Deployment, RBAC, etc.
   - `<provider>_config_transformer.go` — transformer function (or use `common.NoOpTransformer`)
2. Register in `pkg/cloud/cloud.go`:
   - Add case to `getAssetsConstructor()`
   - Add case to `GetCloudConfigTransformer()`
3. Add `manifests/0000_26_..._credentialsrequest-<provider>.yaml` if credentials are needed
4. Add image reference to `pkg/config/config.go` `ImagesReference` struct
5. Update `manifests/0000_26_..._01_images.configmap.yaml`
6. Run `make manifests generate verify`

### Add a New Cloud Config Transformer

Implement the `cloudConfigTransformer` function type:

```go
func MyTransformer(
    source string,
    infra *configv1.Infrastructure,
    network *configv1.Network,
    features featuregates.FeatureGate,
) (string, error) {
    // Parse source, apply transformations, return result
    return result, nil
}
```

Register it in `pkg/cloud/cloud.go`:

```go
case configv1.MyPlatformType:
    return myprovider.CloudConfigTransformer, false, nil  // false = don't look in CCO namespace
```

Write unit tests in `<provider>_config_transformer_test.go` using table-driven tests.

### Update Dependencies

```bash
go get <module>@<version>
go mod tidy
make vendor    # runs go mod vendor
make verify    # verify no drift
```

### Regenerate Manifests

After modifying RBAC markers (`// +kubebuilder:rbac:...`) or CRD types:

```bash
make manifests   # regenerates RBAC and CRD manifests
make generate    # runs controller-gen for deepcopy
make verify      # checks for drift
```

## Build and Release

### Container Image

```bash
# Build image (uses Dockerfile)
make image

# Push
make push
```

### CI Build

The operator image is built by OpenShift CI (`release` repository) on PR merge. See the CI config in:
`https://github.com/openshift/release/tree/master/ci-operator/config/openshift/cluster-cloud-controller-manager-operator`

### Image References

CCCMO reads CCM image references from a ConfigMap mounted at `/etc/cloud-controller-manager-config/images.json`. The `ImagesReference` struct in `pkg/config/config.go` defines the JSON schema. The images are injected by CVO from the `image-references` file in `manifests/`.

## CCCMO-Specific Notes

- **Leader election**: Main operator uses `cluster-cloud-controller-manager-leader`; config-sync uses `cluster-cloud-config-sync-leader`. Both in `openshift-cloud-controller-manager-operator` namespace.
- **Host network**: The CCCMO pod runs with `hostNetwork: true` and tolerates `node.cloudprovider.kubernetes.io/uninitialized` — required because CCM itself must initialize node objects before CNI is ready.
- **Webhook cert**: The operator webhook cert is provisioned by `service-ca-operator` after the `cloud-controller-manager-operator-tls` Service is annotated. The pod starts with a self-signed cert and hot-reloads when the CA-signed cert arrives.
- **TLS adherence**: Changes to the cluster `APIServer` TLS profile (cipher suites, min version) propagate to CCM `--tls-cipher-suites` and `--tls-min-version` flags via `OperatorConfig.TLSCipherSuites`/`TLSMinVersion`.

## See Also

- [Testing Guide](./CCCMO_TESTING.md)
- [Architecture: Components](./architecture/components.md)
- [Domain: Infrastructure Platform](./domain/infrastructure-platform.md)
- [Tier 1 Development Practices](https://github.com/openshift/enhancements/tree/master/ai-docs/practices/development)
