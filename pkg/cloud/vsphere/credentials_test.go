package vsphere

import (
	"context"
	"encoding/base64"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestGetCredentials_ComponentCredentials tests reading component-specific credentials
func TestGetCredentials_ComponentCredentials(t *testing.T) {
	ctx := context.Background()

	// Create a fake secret with component credentials
	secret := createTestSecret(ComponentCredsSecretNamespace, ComponentCredsSecretName, map[string]string{
		"vcenter1.example.com": createTestINIContent("vcenter1.example.com", "cloudcontroller@vsphere.local", "password1"),
	})

	c := createFakeClient(secret)
	reader := NewCredentialReader(c)

	creds, err := reader.GetCredentials(ctx)
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	if len(creds) != 1 {
		t.Errorf("GetCredentials() returned %d credentials, want 1", len(creds))
	}

	cred, ok := creds["vcenter1.example.com"]
	if !ok {
		t.Fatalf("GetCredentials() missing credential for vcenter1.example.com")
	}

	if cred.Username != "cloudcontroller@vsphere.local" {
		t.Errorf("GetCredentials() username = %s, want cloudcontroller@vsphere.local", cred.Username)
	}

	if cred.Password != "password1" {
		t.Errorf("GetCredentials() password = %s, want password1", cred.Password)
	}
}

// TestGetCredentials_FallbackToShared tests fallback to shared credentials
func TestGetCredentials_FallbackToShared(t *testing.T) {
	ctx := context.Background()

	// Create only shared credentials (no component credentials)
	secret := createTestSecret(SharedCredsSecretNamespace, SharedCredsSecretName, map[string]string{
		"vcenter1.example.com": createTestINIContent("vcenter1.example.com", "shared@vsphere.local", "sharedpass"),
	})

	c := createFakeClient(secret)
	reader := NewCredentialReader(c)

	creds, err := reader.GetCredentials(ctx)
	if err != nil {
		t.Fatalf("GetCredentials() error = %v", err)
	}

	cred, ok := creds["vcenter1.example.com"]
	if !ok {
		t.Fatalf("GetCredentials() missing credential for vcenter1.example.com")
	}

	if cred.Username != "shared@vsphere.local" {
		t.Errorf("GetCredentials() username = %s, want shared@vsphere.local", cred.Username)
	}
}

// TestGetCredentialForVCenter_Success tests FQDN-based credential lookup
func TestGetCredentialForVCenter_Success(t *testing.T) {
	ctx := context.Background()

	// Create credentials for multiple vCenters
	secret := createTestSecret(ComponentCredsSecretNamespace, ComponentCredsSecretName, map[string]string{
		"vcenter1.example.com": createTestINIContent("vcenter1.example.com", "user1@vsphere.local", "pass1"),
		"vcenter2.example.com": createTestINIContent("vcenter2.example.com", "user2@vsphere.local", "pass2"),
	})

	c := createFakeClient(secret)
	reader := NewCredentialReader(c)

	// Test lookup for vcenter2
	cred, err := reader.GetCredentialForVCenter(ctx, "vcenter2.example.com")
	if err != nil {
		t.Fatalf("GetCredentialForVCenter() error = %v", err)
	}

	if cred.Username != "user2@vsphere.local" {
		t.Errorf("GetCredentialForVCenter() username = %s, want user2@vsphere.local", cred.Username)
	}
}

// TestGetCredentialForVCenter_NotFound tests error handling for missing vCenter
func TestGetCredentialForVCenter_NotFound(t *testing.T) {
	ctx := context.Background()

	secret := createTestSecret(ComponentCredsSecretNamespace, ComponentCredsSecretName, map[string]string{
		"vcenter1.example.com": createTestINIContent("vcenter1.example.com", "user1@vsphere.local", "pass1"),
	})

	c := createFakeClient(secret)
	reader := NewCredentialReader(c)

	_, err := reader.GetCredentialForVCenter(ctx, "missing.example.com")
	if err == nil {
		t.Fatal("GetCredentialForVCenter() expected error for missing vCenter, got nil")
	}
}

// TestCredentialRotation tests that credentials can be updated without downtime
func TestCredentialRotation(t *testing.T) {
	ctx := context.Background()

	// Initial credentials
	secret := createTestSecret(ComponentCredsSecretNamespace, ComponentCredsSecretName, map[string]string{
		"vcenter1.example.com": createTestINIContent("vcenter1.example.com", "old-user@vsphere.local", "old-pass"),
	})

	c := createFakeClient(secret)
	reader := NewCredentialReader(c)

	// Read initial credentials
	cred1, err := reader.GetCredentialForVCenter(ctx, "vcenter1.example.com")
	if err != nil {
		t.Fatalf("GetCredentialForVCenter() error = %v", err)
	}

	if cred1.Password != "old-pass" {
		t.Errorf("Initial password = %s, want old-pass", cred1.Password)
	}

	// Update the secret with new credentials
	secret.Data["vcenter1.example.com.ini"] = []byte(base64.StdEncoding.EncodeToString([]byte(createTestINIContent("vcenter1.example.com", "new-user@vsphere.local", "new-pass"))))
	err = c.Update(ctx, secret)
	if err != nil {
		t.Fatalf("Failed to update secret: %v", err)
	}

	// Read updated credentials
	cred2, err := reader.GetCredentialForVCenter(ctx, "vcenter1.example.com")
	if err != nil {
		t.Fatalf("GetCredentialForVCenter() error after rotation = %v", err)
	}

	if cred2.Password != "new-pass" {
		t.Errorf("Rotated password = %s, want new-pass", cred2.Password)
	}
}

// Helper functions

func createFakeClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func createTestSecret(namespace, name string, vcenters map[string]string) *corev1.Secret {
	data := make(map[string][]byte)
	for vcenterFQDN, iniContent := range vcenters {
		key := vcenterFQDN + ".ini"
		// Base64 encode the INI content
		data[key] = []byte(base64.StdEncoding.EncodeToString([]byte(iniContent)))
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func createTestINIContent(vcenterFQDN, username, password string) string {
	return fmt.Sprintf(`[Global]
secret-name = "vsphere-cloud-controller-creds"
secret-namespace = "openshift-cloud-controller-manager"

[VirtualCenter "%s"]
user = "%s"
password = "%s"
datacenters = "datacenter1"
`, vcenterFQDN, username, password)
}
