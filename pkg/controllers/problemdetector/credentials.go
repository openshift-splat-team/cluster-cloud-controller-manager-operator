package problemdetector

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// DiagnosticsCredentialsSecretName is the name of the secret containing diagnostics component credentials
	DiagnosticsCredentialsSecretName = "vsphere-diagnostics-creds"
	// DiagnosticsCredentialsNamespace is the namespace where diagnostics credentials are stored
	DiagnosticsCredentialsNamespace = "openshift-config"
	// SharedCredentialsSecretName is the fallback shared credentials secret
	SharedCredentialsSecretName = "vsphere-cloud-credentials"
	// SharedCredentialsNamespace is the namespace for shared credentials
	SharedCredentialsNamespace = "kube-system"
)

// Credential represents vSphere credentials for a specific vCenter
type Credential struct {
	VCenterServer string
	Username      string
	Password      string
}

// CredentialStore manages vSphere credentials for diagnostics operations
type CredentialStore struct {
	client client.Client
}

// NewCredentialStore creates a new CredentialStore
func NewCredentialStore(c client.Client) *CredentialStore {
	return &CredentialStore{
		client: c,
	}
}

// GetCredentials retrieves diagnostics credentials from openshift-config namespace.
// Falls back to shared credentials if component credentials are not available.
func (cs *CredentialStore) GetCredentials(ctx context.Context) (map[string]*Credential, error) {
	// Try component-specific credentials first
	creds, err := cs.getCredentialsFromSecret(ctx, DiagnosticsCredentialsSecretName, DiagnosticsCredentialsNamespace)
	if err == nil && len(creds) > 0 {
		return creds, nil
	}

	// Fallback to shared credentials
	creds, err = cs.getCredentialsFromSecret(ctx, SharedCredentialsSecretName, SharedCredentialsNamespace)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials from %s/%s and fallback %s/%s: %w",
			DiagnosticsCredentialsNamespace, DiagnosticsCredentialsSecretName,
			SharedCredentialsNamespace, SharedCredentialsSecretName, err)
	}

	return creds, nil
}

// GetCredentialForVCenter retrieves the credential for a specific vCenter FQDN
func (cs *CredentialStore) GetCredentialForVCenter(ctx context.Context, vcenterFQDN string) (*Credential, error) {
	creds, err := cs.GetCredentials(ctx)
	if err != nil {
		return nil, err
	}

	cred, ok := creds[vcenterFQDN]
	if !ok {
		return nil, fmt.Errorf("credential not found for vCenter: %s", vcenterFQDN)
	}

	return cred, nil
}

// getCredentialsFromSecret reads credentials from a Kubernetes secret
func (cs *CredentialStore) getCredentialsFromSecret(ctx context.Context, name, namespace string) (map[string]*Credential, error) {
	secret := &corev1.Secret{}
	key := client.ObjectKey{
		Name:      name,
		Namespace: namespace,
	}

	if err := cs.client.Get(ctx, key, secret); err != nil {
		return nil, fmt.Errorf("failed to get secret %s/%s: %w", namespace, name, err)
	}

	return parseCredentialData(secret.Data)
}

// parseCredentialData parses credential data from secret into a map of credentials by vCenter FQDN
func parseCredentialData(data map[string][]byte) (map[string]*Credential, error) {
	credentials := make(map[string]*Credential)

	// Check if this is INI format (legacy) or key-value format
	if credentialsINI, ok := data["credentials"]; ok {
		return parseINIFormat(credentialsINI)
	}

	// Parse key-value format: {vcenter-fqdn}.username, {vcenter-fqdn}.password
	usernameKeys := make(map[string]string)
	passwordKeys := make(map[string]string)

	for key, value := range data {
		if strings.HasSuffix(key, ".username") {
			vcenter := strings.TrimSuffix(key, ".username")
			usernameKeys[vcenter] = string(value)
		} else if strings.HasSuffix(key, ".password") {
			vcenter := strings.TrimSuffix(key, ".password")
			passwordKeys[vcenter] = string(value)
		}
	}

	// Match username and password pairs
	for vcenter, username := range usernameKeys {
		password, ok := passwordKeys[vcenter]
		if !ok {
			return nil, fmt.Errorf("missing password for vCenter: %s", vcenter)
		}

		credentials[vcenter] = &Credential{
			VCenterServer: vcenter,
			Username:      username,
			Password:      password,
		}
	}

	if len(credentials) == 0 {
		return nil, fmt.Errorf("no credentials found in secret data")
	}

	return credentials, nil
}

// parseINIFormat parses INI-formatted credentials (legacy format)
func parseINIFormat(data []byte) (map[string]*Credential, error) {
	credentials := make(map[string]*Credential)
	lines := strings.Split(string(data), "\n")

	var currentVCenter string
	var currentCred *Credential

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Section header [vcenter.example.com]
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			// Save previous credential if exists
			if currentCred != nil && currentVCenter != "" {
				credentials[currentVCenter] = currentCred
			}

			currentVCenter = strings.Trim(line, "[]")
			currentCred = &Credential{
				VCenterServer: currentVCenter,
			}
			continue
		}

		// Key-value pair
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if currentCred != nil {
			switch key {
			case "username":
				currentCred.Username = value
			case "password":
				currentCred.Password = value
			}
		}
	}

	// Save last credential
	if currentCred != nil && currentVCenter != "" {
		credentials[currentVCenter] = currentCred
	}

	if len(credentials) == 0 {
		return nil, fmt.Errorf("no credentials found in INI data")
	}

	return credentials, nil
}
