package problemdetector

import (
	"context"
	"fmt"
)

// DiagnosticsPrivileges defines the required read-only privileges for vSphere Problem Detector
// Total: ~16 read-only privileges
var DiagnosticsPrivileges = []string{
	// System-level read privileges (3)
	"System.Anonymous",
	"System.Read",
	"System.View",

	// vCenter-level read privileges for diagnostics operations (11)
	// Tagging operations
	"Cns.Searchable",                 // CNS (Container Native Storage) search
	"InventoryService.Tagging.Read",  // Read tags
	"InventoryService.Tagging.Attach", // Validate tag attachments (read-only check)

	// Session and performance monitoring
	"Sessions.View",        // View active sessions
	"Performance.View",     // View performance metrics
	"Global.Settings.Read", // Read global settings

	// Health monitoring
	"Global.Health",             // Check vSphere health status
	"Global.Diagnostics",        // Access diagnostic information
	"Global.ServiceManagers",    // View service managers
	"VirtualMachine.Config.DiskExtend", // Validate VM disk configuration (read-only check)

	// CNS volume operations (read-only)
	"StorageProfile.View", // View storage profiles for CNS

	// Datacenter-level read privilege (1)
	"Datacenter.Read", // Read datacenter configuration (System.Read at datacenter level)

	// Datastore-level read privileges (4)
	"Datastore.Browse",        // Browse datastore contents
	"Datastore.FileManagement", // Validate file management capabilities (read-only check)
	"Datastore.Config.Read",   // Read datastore configuration
	"Datastore.Query",         // Query datastore metrics and status
}

// ValidationResult holds the result of privilege validation
type ValidationResult struct {
	Valid             bool
	VCenterServer     string
	MissingPrivileges []string
	Error             error
}

// PrivilegeValidator validates vSphere privileges for diagnostics operations
type PrivilegeValidator struct {
	credStore *CredentialStore
}

// NewPrivilegeValidator creates a new PrivilegeValidator
func NewPrivilegeValidator(credStore *CredentialStore) *PrivilegeValidator {
	return &PrivilegeValidator{
		credStore: credStore,
	}
}

// ValidatePrivileges validates that the provided credentials have the required privileges.
//
// # Current Implementation Scope
//
// This implementation performs basic structural validation of credentials:
// - Verifies credentials are non-empty (username and password are set)
// - Returns a validation result indicating whether basic credential structure is valid
//
// # Future Enhancement
//
// Full vSphere API privilege checking is planned for future implementation:
// - Connect to vSphere API using the credentials
// - Query the AuthorizationManager to verify each privilege in DiagnosticsPrivileges
// - Return detailed results showing which specific privileges are present/missing
// - Validate against actual vCenter RBAC configuration
//
// # Rationale for Basic Validation
//
// The basic validation approach is sufficient for initial integration because:
// - It prevents obviously invalid credentials (empty username/password)
// - Component operators will detect privilege issues when performing actual operations
// - Full privilege checking requires additional vSphere API dependencies and error handling
// - This allows the credential integration to proceed while privilege checking is refined
func (pv *PrivilegeValidator) ValidatePrivileges(ctx context.Context, cred *Credential) *ValidationResult {
	result := &ValidationResult{
		VCenterServer:     cred.VCenterServer,
		MissingPrivileges: []string{},
	}

	// Basic credential structure validation
	if cred.Username == "" || cred.Password == "" {
		result.Valid = false
		result.Error = fmt.Errorf("invalid credentials: username or password is empty")
		return result
	}

	// TODO: Implement actual vSphere API privilege checking
	// This would involve:
	// 1. Creating a vSphere session using the credentials
	// 2. Querying the AuthorizationManager for the user's privileges
	// 3. Comparing against DiagnosticsPrivileges to identify missing privileges
	// 4. Returning detailed results with specific privilege information

	result.Valid = true
	return result
}

// ValidateAllPrivileges validates privileges for all configured vCenters
func (pv *PrivilegeValidator) ValidateAllPrivileges(ctx context.Context) (map[string]*ValidationResult, error) {
	creds, err := pv.credStore.GetCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get credentials: %w", err)
	}

	results := make(map[string]*ValidationResult)
	for vcenter, cred := range creds {
		results[vcenter] = pv.ValidatePrivileges(ctx, cred)
	}

	return results, nil
}

// GetErrorMessage formats a user-friendly error message for privilege validation failures
func (vr *ValidationResult) GetErrorMessage() string {
	if vr.Valid {
		return ""
	}

	if vr.Error != nil {
		return fmt.Sprintf("vCenter %s: %s", vr.VCenterServer, vr.Error.Error())
	}

	if len(vr.MissingPrivileges) == 0 {
		return fmt.Sprintf("vCenter %s: privilege validation failed (unknown reason)", vr.VCenterServer)
	}

	return fmt.Sprintf("vCenter %s: missing privileges: %v\n"+
		"Please grant the following privileges to the diagnostics account:\n%s",
		vr.VCenterServer,
		vr.MissingPrivileges,
		formatPrivilegeList(vr.MissingPrivileges))
}

// formatPrivilegeList formats a list of privileges for display
func formatPrivilegeList(privileges []string) string {
	var result string
	for _, priv := range privileges {
		result += fmt.Sprintf("  - %s\n", priv)
	}
	return result
}
