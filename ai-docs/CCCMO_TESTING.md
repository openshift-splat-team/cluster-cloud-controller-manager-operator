# CCCMO Testing Guide

> **Generic Testing Practices**: See [Tier 1 Testing Practices](https://github.com/openshift/enhancements/tree/master/ai-docs/practices/testing) for test pyramid philosophy (60/30/10), E2E framework patterns, and mock vs real strategies.

This guide covers **CCCMO-specific** test suites and testing practices.

## Test Organization

```text
        E2E Tests (CI-only, per-platform)
              ▲
    Unit + Controller Tests (primary, fast)
```

CCCMO does not have integration tests distinct from unit tests; controller tests use `controller-runtime/pkg/envtest` to run against a real (in-process) Kubernetes API server.

## Unit Tests

### Location

Unit tests live alongside source files:
- `pkg/cloud/*_test.go` — cloud config transformer unit tests
- `pkg/cloud/common/*_test.go` — template rendering and substitution tests
- `pkg/controllers/*_test.go` — controller reconcile logic (using envtest)
- `pkg/util/*_test.go` — utility function tests

### Running Unit Tests

```bash
# All unit tests
make unit

# Full pipeline (generate + verify + unit)
make test

# Specific package
go test -v ./pkg/cloud/openstack/...
go test -v ./pkg/controllers/...

# Disable test caching
go test -count=1 ./pkg/...

# With coverage
go test -cover ./pkg/...
go test -coverprofile=coverage.out ./pkg/...
go tool cover -html=coverage.out
```

### Unit Test Patterns

#### Controller Tests (envtest)

Controller tests use `sigs.k8s.io/controller-runtime/pkg/envtest` to run against an in-process API server. The test suite is set up in `pkg/controllers/suite_test.go`:

```go
var _ = BeforeSuite(func() {
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{
            filepath.Join("..", "..", "vendor", "github.com", "openshift", "api", ...),
        },
    }
    cfg, err = testEnv.Start()
    // ...
})
```

Individual controller tests create fake Infrastructure/Proxy resources and assert on reconcile outcomes:

```go
It("should apply CCM deployment for AWS platform", func() {
    infra := makeInfrastructure(configv1.AWSPlatformType)
    Expect(cl.Create(ctx, infra)).To(Succeed())
    // Trigger reconcile
    req := reconcile.Request{...}
    _, err := r.Reconcile(ctx, req)
    Expect(err).NotTo(HaveOccurred())
    // Assert deployment exists
    deploy := &appsv1.Deployment{}
    Expect(cl.Get(ctx, types.NamespacedName{...}, deploy)).To(Succeed())
})
```

Test fixtures (sample Infrastructure, Proxy, ConfigMap objects) are in `pkg/controllers/fixtures/`.

#### Cloud Config Transformer Tests

Each transformer is tested with table-driven tests covering:
- Normal input with expected transformation
- Missing required sections
- Proxy settings injection
- Feature gate-dependent behavior

Example pattern from `pkg/cloud/openstack/`:
```go
func TestCloudConfigTransformer(t *testing.T) {
    cases := []struct {
        name     string
        input    string
        infra    *configv1.Infrastructure
        network  *configv1.Network
        expected string
        wantErr  bool
    }{...}
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            result, err := CloudConfigTransformer(tc.input, tc.infra, tc.network, nil)
            // ...
        })
    }
}
```

#### Asset Rendering Tests

`pkg/cloud/<provider>/*_test.go` tests verify that `NewProviderAssets()` renders valid Kubernetes objects for given `OperatorConfig` inputs. They check:
- Correct image references are set
- Replica count matches `IsSingleReplica`
- Proxy env vars are injected when proxy is set
- TLS flags are passed correctly

#### Common Substitution Tests

`pkg/cloud/common/substitution_test.go` tests `SubstituteCommonPartsFromConfig` — verifying proxy env injection, TLS propagation, and replica count substitution across different Deployment specs.

### Test Fixtures

- `pkg/controllers/fixtures/` — static YAML fixtures for tests
- `test/manifest/` — additional static manifests
- `hack/example-images.json` — example images reference for local dev

## E2E Tests

### Location

E2E tests are organized per-provider and run in OpenShift CI:
- `cmd/cloud-controller-manager-aws-tests-ext/e2e/` — AWS-specific E2E (extension binary)
- CI configurations: [openshift/release: ci-operator/config/openshift/cluster-cloud-controller-manager-operator](https://github.com/openshift/release/tree/master/ci-operator/config/openshift/cluster-cloud-controller-manager-operator)

### Running E2E Tests

E2E tests require a real OpenShift cluster on the target platform:

```bash
# AWS E2E (requires KUBECONFIG pointing to AWS cluster)
export KUBECONFIG=/path/to/kubeconfig
go test -v ./cmd/cloud-controller-manager-aws-tests-ext/e2e/... -timeout 30m
```

Most E2E tests are run exclusively through OpenShift CI pull request jobs. Manual triggering via `/test e2e-aws` (or platform-specific variant) on the GitHub PR is the typical workflow.

### E2E Test Scenarios

CCCMO E2E tests typically cover:
- CCM Deployment is created and reaches `Available` state on target platform
- `ClusterOperator/cloud-controller-manager` reports `Available=true`
- Cloud config ConfigMap (`cloud-conf`) is correctly populated
- Trusted CA bundle ConfigMap (`ccm-trusted-ca`) is populated
- Node initialization: nodes have `ProviderID` set after CCM starts
- (AWS) ValidatingAdmissionPolicy for cloud provider immutability

### CI Job Naming Conventions

| Job Pattern | Platform |
|-------------|----------|
| `e2e-aws` | AWS |
| `e2e-azure` | Azure |
| `e2e-gcp` | GCP |
| `e2e-openstack` | OpenStack |
| `e2e-vsphere` | vSphere |
| `e2e-nutanix` | Nutanix |

## Debugging Failing Tests

### Unit Test Failures

```bash
# Run with verbose output
go test -v ./pkg/controllers/... -run TestCloudOperatorReconciler

# Run with race detector
go test -race ./pkg/...

# Enable klog output in tests
go test -v ./pkg/controllers/... -args -v=4
```

### Controller Test Environment Issues

If envtest fails to start:
```bash
# Check that CRD paths in suite_test.go point to the right vendor location
# Ensure KUBEBUILDER_ASSETS is set if using a custom API server binary
export KUBEBUILDER_ASSETS=$(go env GOPATH)/pkg/mod/sigs.k8s.io/controller-runtime@v*/pkg/internal/testing/integration/assets
```

### E2E Test Failures

```bash
# Check operator pod logs
oc logs -f deployment/cluster-cloud-controller-manager-operator \
    -n openshift-cloud-controller-manager-operator \
    -c cluster-cloud-controller-manager

# Check config sync logs
oc logs -f deployment/cluster-cloud-controller-manager-operator \
    -n openshift-cloud-controller-manager-operator \
    -c config-sync-controllers

# Check ClusterOperator conditions
oc describe clusteroperator/cloud-controller-manager

# Check CCM deployment
oc get deploy -n openshift-cloud-controller-manager
oc describe deploy/aws-cloud-controller-manager -n openshift-cloud-controller-manager

# Check cloud config ConfigMap
oc get configmap/cloud-conf -n openshift-cloud-controller-manager -o yaml

# Check CA bundle ConfigMap
oc get configmap/ccm-trusted-ca -n openshift-cloud-controller-manager -o yaml
```

### Known Test Considerations

- Controller tests run with `envtest` which may require specific Kubernetes API binary versions. The `go.mod` pins controller-runtime which brings in the correct envtest version.
- Some cloud config transformer tests use snapshot files in `_testdata/` directories — if transformer output changes, update snapshots intentionally.
- AWS tests require a `ValidatingAdmissionPolicy` (VAP) which was introduced in Kubernetes 1.28; tests will skip on older clusters.

## Component-Specific Test Notes

- **Platform-specific behavior**: Each platform's asset rendering and config transformer has dedicated tests. When adding a new platform, always add both asset rendering tests and config transformer tests.
- **govalidator coverage**: Template value validation via `govalidator.ValidateMap` is implicitly tested by asset rendering tests — tests that pass `OperatorConfig` without required images should return errors.
- **Proxy injection testing**: `pkg/cloud/common/substitution_test.go` covers proxy env injection. Individual provider asset tests may also verify proxy propagation.
- **Single-replica testing**: Tests should cover both `IsSingleReplica=true` (no PDB created) and `IsSingleReplica=false` (PDB created) code paths.

## See Also

- [Development Guide](./CCCMO_DEVELOPMENT.md)
- [Architecture: Components](./architecture/components.md)
- [Tier 1 Testing Practices](https://github.com/openshift/enhancements/tree/master/ai-docs/practices/testing)
