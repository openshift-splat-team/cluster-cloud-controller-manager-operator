package vsphere

import (
	"context"
	"fmt"
)

// CloudControllerPrivileges defines the required vSphere privileges for Cloud Controller Manager
// These are read-only privileges required for node discovery operations
var CloudControllerPrivileges = []string{
	// System privileges for basic access
	"System.Anonymous",
	"System.Read",
	"System.View",

	// VirtualMachine read-only privileges for node discovery
	"VirtualMachine.Inventory.Read",

	// Datacenter read privileges
	"Datacenter.Read",

	// Network read privileges
	"Network.Read",

	// Datastore read privileges
	"Datastore.Browse",

	// Resource pool read privileges
	"Resource.Read",
}

// PrivilegeValidator validates that credentials have the required privileges
type PrivilegeValidator struct {
	requiredPrivileges []string
}

// NewPrivilegeValidator creates a new PrivilegeValidator
func NewPrivilegeValidator() *PrivilegeValidator {
	return &PrivilegeValidator{
		requiredPrivileges: CloudControllerPrivileges,
	}
}

// ValidatePrivileges validates that the given credentials have all required privileges
//
// IMPLEMENTATION SCOPE: This current implementation validates only that credentials are
// structurally valid (non-empty username/password). It does NOT validate actual vSphere
// privilege assignments via the vSphere API.
//
// FUTURE WORK: A full implementation would:
// 1. Connect to vSphere using the credential
// 2. Query the privileges assigned to the user via SessionManager.AcquireSessionCookie
// 3. Use AuthorizationManager.HasPrivilegeOnEntity to check each required privilege
// 4. Return specific missing privileges
//
// RATIONALE: Basic credential validation is sufficient for initial integration.
// Real privilege checking requires additional vSphere API dependencies and error
// handling for network failures, authentication errors, etc. This can be added
// when operational requirements demand it.
func (v *PrivilegeValidator) ValidatePrivileges(ctx context.Context, cred *VCenterCredential) (ValidationResult, error) {
	result := ValidationResult{
		VCenter:           cred.VCenter,
		RequiredPrivileges: v.requiredPrivileges,
		MissingPrivileges: []string{},
		Valid:             true,
	}

	// Validate basic credential structure
	if cred.Username == "" || cred.Password == "" {
		result.Valid = false
		result.MissingPrivileges = v.requiredPrivileges
		result.Error = fmt.Sprintf("invalid credentials for vCenter %s: username or password is empty", cred.VCenter)
		return result, nil
	}

	return result, nil
}

// ValidateAllPrivileges validates privileges for all vCenter credentials
func (v *PrivilegeValidator) ValidateAllPrivileges(ctx context.Context, creds map[string]*VCenterCredential) (map[string]ValidationResult, error) {
	results := make(map[string]ValidationResult)

	for vcenterFQDN, cred := range creds {
		result, err := v.ValidatePrivileges(ctx, cred)
		if err != nil {
			return nil, fmt.Errorf("failed to validate privileges for vCenter %s: %w", vcenterFQDN, err)
		}
		results[vcenterFQDN] = result
	}

	return results, nil
}

// ValidationResult contains the result of privilege validation
type ValidationResult struct {
	VCenter            string
	RequiredPrivileges []string
	MissingPrivileges  []string
	Valid              bool
	Error              string
}

// GetMissingPrivileges returns a list of missing privileges
func (r *ValidationResult) GetMissingPrivileges() []string {
	return r.MissingPrivileges
}

// IsValid returns true if all required privileges are present
func (r *ValidationResult) IsValid() bool {
	return r.Valid
}

// GetErrorMessage returns a formatted error message for missing privileges
func (r *ValidationResult) GetErrorMessage() string {
	if r.Valid {
		return ""
	}

	if r.Error != "" {
		return r.Error
	}

	return fmt.Sprintf("vCenter %s is missing required privileges: %v", r.VCenter, r.MissingPrivileges)
}
