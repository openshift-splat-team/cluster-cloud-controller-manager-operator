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

	// VirtualMachine privileges for node discovery
	"VirtualMachine.Inventory.Register",
	"VirtualMachine.Inventory.Unregister",
	"VirtualMachine.Config.AddExistingDisk",
	"VirtualMachine.Config.AddNewDisk",
	"VirtualMachine.Config.RemoveDisk",
	"VirtualMachine.Config.EditDevice",

	// Resource pool and network privileges
	"Resource.AssignVMToPool",
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
// This is a placeholder implementation - in a real system, this would use the vSphere API
// to query the actual privileges assigned to the credential
func (v *PrivilegeValidator) ValidatePrivileges(ctx context.Context, cred *VCenterCredential) (ValidationResult, error) {
	result := ValidationResult{
		VCenter:           cred.VCenter,
		RequiredPrivileges: v.requiredPrivileges,
		MissingPrivileges: []string{},
		Valid:             true,
	}

	// In a real implementation, this would:
	// 1. Connect to vSphere using the credential
	// 2. Query the privileges assigned to the user
	// 3. Compare with the required privileges
	// 4. Return any missing privileges
	//
	// For this implementation, we simulate validation
	// by checking basic credential fields are present
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
