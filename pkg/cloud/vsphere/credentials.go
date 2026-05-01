package vsphere

import (
	"context"
	"encoding/base64"
	"fmt"
	"gopkg.in/ini.v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ComponentCredsSecretName is the name of the component-specific credential secret
	ComponentCredsSecretName = "vsphere-cloud-controller-creds"
	// ComponentCredsSecretNamespace is the namespace where CCO provisions component credentials
	ComponentCredsSecretNamespace = "openshift-cloud-controller-manager"
	// SharedCredsSecretName is the name of the shared credential secret (fallback)
	SharedCredsSecretName = "vsphere-cloud-credentials"
	// SharedCredsSecretNamespace is the namespace where shared credentials are stored
	SharedCredsSecretNamespace = "kube-system"
)

// VCenterCredential represents credentials for a single vCenter
type VCenterCredential struct {
	VCenter  string
	Username string
	Password string
}

// CredentialReader handles reading vSphere credentials
type CredentialReader struct {
	client client.Client
}

// NewCredentialReader creates a new CredentialReader
func NewCredentialReader(c client.Client) *CredentialReader {
	return &CredentialReader{client: c}
}

// GetCredentials reads component-specific credentials or falls back to shared credentials
func (r *CredentialReader) GetCredentials(ctx context.Context) (map[string]*VCenterCredential, error) {
	// Try component-specific credentials first
	componentCreds, err := r.readCredentialsFromSecret(ctx, ComponentCredsSecretNamespace, ComponentCredsSecretName)
	if err == nil {
		return componentCreds, nil
	}

	// Fall back to shared credentials
	sharedCreds, err := r.readCredentialsFromSecret(ctx, SharedCredsSecretNamespace, SharedCredsSecretName)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials from both component and shared secrets: %w", err)
	}

	return sharedCreds, nil
}

// GetCredentialForVCenter returns credentials for a specific vCenter FQDN
func (r *CredentialReader) GetCredentialForVCenter(ctx context.Context, vcenterFQDN string) (*VCenterCredential, error) {
	creds, err := r.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}

	cred, ok := creds[vcenterFQDN]
	if !ok {
		return nil, fmt.Errorf("no credentials found for vCenter: %s", vcenterFQDN)
	}

	return cred, nil
}

// readCredentialsFromSecret reads and parses credentials from a Kubernetes secret
func (r *CredentialReader) readCredentialsFromSecret(ctx context.Context, namespace, secretName string) (map[string]*VCenterCredential, error) {
	secret := &corev1.Secret{}
	err := r.client.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      secretName,
	}, secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, secretName, err)
	}

	// Parse credentials from secret data
	// The secret format follows the multi-vCenter credential schema from the design
	// where each vCenter has its own credential data keyed by FQDN
	return parseCredentialData(secret.Data)
}

// parseCredentialData parses credential data from a secret
// Expected secret format (INI-style):
//
//	<vcenter-fqdn>.ini: base64-encoded INI file with credentials
//
// INI file format:
//
//	[Global]
//	secret-name = "vsphere-cloud-controller-creds"
//	secret-namespace = "openshift-cloud-controller-manager"
//
//	[VirtualCenter "vcenter1.example.com"]
//	user = "cloudcontroller@vsphere.local"
//	password = "password"
//	datacenters = "datacenter1"
func parseCredentialData(data map[string][]byte) (map[string]*VCenterCredential, error) {
	creds := make(map[string]*VCenterCredential)

	// Iterate over all keys in the secret data
	for key, value := range data {
		// Check if this is an INI file (ends with .ini)
		if len(key) > 4 && key[len(key)-4:] == ".ini" {
			// Extract vCenter FQDN from key (remove .ini extension)
			vcenterFQDN := key[:len(key)-4]

			// Parse the INI file
			iniData, err := parseINIFile(value)
			if err != nil {
				return nil, fmt.Errorf("failed to parse INI file for vCenter %s: %w", vcenterFQDN, err)
			}

			// Extract credentials from the parsed INI
			cred, err := extractCredentialFromINI(iniData, vcenterFQDN)
			if err != nil {
				return nil, fmt.Errorf("failed to extract credentials for vCenter %s: %w", vcenterFQDN, err)
			}

			creds[vcenterFQDN] = cred
		}
	}

	if len(creds) == 0 {
		return nil, fmt.Errorf("no valid vCenter credentials found in secret")
	}

	return creds, nil
}

// parseINIFile parses an INI file from byte data
func parseINIFile(data []byte) (*ini.File, error) {
	// Try to decode base64 first (in case the data is encoded)
	decoded, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		// If decoding fails, assume the data is already plain text
		decoded = data
	}

	cfg, err := ini.Load(decoded)
	if err != nil {
		return nil, fmt.Errorf("failed to parse INI: %w", err)
	}

	return cfg, nil
}

// extractCredentialFromINI extracts credential information from a parsed INI file
func extractCredentialFromINI(cfg *ini.File, vcenterFQDN string) (*VCenterCredential, error) {
	// Look for the VirtualCenter section with the FQDN
	sectionName := fmt.Sprintf("VirtualCenter \"%s\"", vcenterFQDN)
	section, err := cfg.GetSection(sectionName)
	if err != nil {
		return nil, fmt.Errorf("section %s not found in INI file: %w", sectionName, err)
	}

	// Extract username and password
	username, err := section.GetKey("user")
	if err != nil {
		return nil, fmt.Errorf("user not found in section %s: %w", sectionName, err)
	}

	password, err := section.GetKey("password")
	if err != nil {
		return nil, fmt.Errorf("password not found in section %s: %w", sectionName, err)
	}

	return &VCenterCredential{
		VCenter:  vcenterFQDN,
		Username: username.String(),
		Password: password.String(),
	}, nil
}
