package manifest_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const vsphereComponentAnnotation = "cloudcredential.openshift.io/vsphere-component"

type credentialsRequest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name        string            `yaml:"name"`
		Annotations map[string]string `yaml:"annotations"`
	} `yaml:"metadata"`
}

func parseCredentialsRequests(t *testing.T, path string) []credentialsRequest {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var out []credentialsRequest
	for _, doc := range strings.Split(string(data), "\n---") {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}
		var cr credentialsRequest
		if err := yaml.Unmarshal([]byte(doc), &cr); err != nil {
			t.Fatalf("unmarshal %s: %v", path, err)
		}
		if cr.Kind == "CredentialsRequest" {
			out = append(out, cr)
		}
	}
	return out
}

func findCR(crs []credentialsRequest, name string) (credentialsRequest, bool) {
	for _, cr := range crs {
		if cr.Metadata.Name == name {
			return cr, true
		}
	}
	return credentialsRequest{}, false
}

func TestCCMVSphereCredentialsRequestAnnotation(t *testing.T) {
	path := "../../manifests/0000_26_cloud-controller-manager-operator_17_credentialsrequest-vsphere.yaml"
	crs := parseCredentialsRequests(t, path)

	cr, ok := findCR(crs, "openshift-vsphere-cloud-controller-manager")
	if !ok {
		t.Fatal("CredentialsRequest 'openshift-vsphere-cloud-controller-manager' not found in manifest")
	}

	got, present := cr.Metadata.Annotations[vsphereComponentAnnotation]
	if !present {
		t.Fatalf("annotation %q missing from openshift-vsphere-cloud-controller-manager", vsphereComponentAnnotation)
	}
	if got != "cloudController" {
		t.Errorf("annotation value: got %q, want %q", got, "cloudController")
	}
}

func TestAnnotationValueIsNotEmpty(t *testing.T) {
	path := "../../manifests/0000_26_cloud-controller-manager-operator_17_credentialsrequest-vsphere.yaml"
	crs := parseCredentialsRequests(t, path)

	cr, ok := findCR(crs, "openshift-vsphere-cloud-controller-manager")
	if !ok {
		t.Fatal("CredentialsRequest 'openshift-vsphere-cloud-controller-manager' not found in manifest")
	}
	val := cr.Metadata.Annotations[vsphereComponentAnnotation]
	if strings.TrimSpace(val) == "" {
		t.Errorf("annotation %q must not be empty", vsphereComponentAnnotation)
	}
}
