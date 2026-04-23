package vsphere

import (
	"context"
	"testing"
)

// TestValidatePrivileges_Valid tests privilege validation with valid credentials
func TestValidatePrivileges_Valid(t *testing.T) {
	ctx := context.Background()
	validator := NewPrivilegeValidator()

	cred := &VCenterCredential{
		VCenter:  "vcenter1.example.com",
		Username: "cloudcontroller@vsphere.local",
		Password: "validpassword",
	}

	result, err := validator.ValidatePrivileges(ctx, cred)
	if err != nil {
		t.Fatalf("ValidatePrivileges() error = %v", err)
	}

	if !result.IsValid() {
		t.Errorf("ValidatePrivileges() valid = %v, want true. Error: %s", result.IsValid(), result.GetErrorMessage())
	}

	if len(result.GetMissingPrivileges()) > 0 {
		t.Errorf("ValidatePrivileges() missing privileges = %v, want empty", result.GetMissingPrivileges())
	}
}

// TestValidatePrivileges_InvalidCredentials tests privilege validation with empty credentials
func TestValidatePrivileges_InvalidCredentials(t *testing.T) {
	ctx := context.Background()
	validator := NewPrivilegeValidator()

	cred := &VCenterCredential{
		VCenter:  "vcenter1.example.com",
		Username: "",
		Password: "",
	}

	result, err := validator.ValidatePrivileges(ctx, cred)
	if err != nil {
		t.Fatalf("ValidatePrivileges() error = %v", err)
	}

	if result.IsValid() {
		t.Errorf("ValidatePrivileges() valid = %v, want false for empty credentials", result.IsValid())
	}

	if len(result.GetMissingPrivileges()) == 0 {
		t.Errorf("ValidatePrivileges() missing privileges = empty, want all required privileges")
	}
}

// TestValidateAllPrivileges_MultiVCenter tests privilege validation for multiple vCenters
func TestValidateAllPrivileges_MultiVCenter(t *testing.T) {
	ctx := context.Background()
	validator := NewPrivilegeValidator()

	creds := map[string]*VCenterCredential{
		"vcenter1.example.com": {
			VCenter:  "vcenter1.example.com",
			Username: "user1@vsphere.local",
			Password: "pass1",
		},
		"vcenter2.example.com": {
			VCenter:  "vcenter2.example.com",
			Username: "user2@vsphere.local",
			Password: "pass2",
		},
	}

	results, err := validator.ValidateAllPrivileges(ctx, creds)
	if err != nil {
		t.Fatalf("ValidateAllPrivileges() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("ValidateAllPrivileges() returned %d results, want 2", len(results))
	}

	for vcenter, result := range results {
		if !result.IsValid() {
			t.Errorf("ValidateAllPrivileges() vCenter %s validation failed: %s", vcenter, result.GetErrorMessage())
		}
	}
}

// TestValidateAllPrivileges_PartialFailure tests privilege validation with some invalid credentials
func TestValidateAllPrivileges_PartialFailure(t *testing.T) {
	ctx := context.Background()
	validator := NewPrivilegeValidator()

	creds := map[string]*VCenterCredential{
		"vcenter1.example.com": {
			VCenter:  "vcenter1.example.com",
			Username: "user1@vsphere.local",
			Password: "pass1",
		},
		"vcenter2.example.com": {
			VCenter:  "vcenter2.example.com",
			Username: "",
			Password: "",
		},
	}

	results, err := validator.ValidateAllPrivileges(ctx, creds)
	if err != nil {
		t.Fatalf("ValidateAllPrivileges() error = %v", err)
	}

	if len(results) != 2 {
		t.Errorf("ValidateAllPrivileges() returned %d results, want 2", len(results))
	}

	// vcenter1 should be valid
	if !results["vcenter1.example.com"].IsValid() {
		t.Errorf("ValidateAllPrivileges() vcenter1.example.com should be valid")
	}

	// vcenter2 should be invalid
	if results["vcenter2.example.com"].IsValid() {
		t.Errorf("ValidateAllPrivileges() vcenter2.example.com should be invalid")
	}
}

// TestPrivilegeCount tests that all required privileges are defined
func TestPrivilegeCount(t *testing.T) {
	// Cloud Controller Manager should have ~10 read-only privileges
	if len(CloudControllerPrivileges) < 8 || len(CloudControllerPrivileges) > 12 {
		t.Errorf("CloudControllerPrivileges count = %d, want approximately 10 (8-12)", len(CloudControllerPrivileges))
	}
}

// TestGetErrorMessage tests error message formatting
func TestGetErrorMessage(t *testing.T) {
	tests := []struct {
		name           string
		result         ValidationResult
		expectNonEmpty bool
	}{
		{
			name: "valid result has empty error message",
			result: ValidationResult{
				VCenter: "vcenter1.example.com",
				Valid:   true,
			},
			expectNonEmpty: false,
		},
		{
			name: "invalid result with missing privileges",
			result: ValidationResult{
				VCenter:           "vcenter1.example.com",
				Valid:             false,
				MissingPrivileges: []string{"System.Read", "VirtualMachine.Inventory.Register"},
			},
			expectNonEmpty: true,
		},
		{
			name: "invalid result with error message",
			result: ValidationResult{
				VCenter: "vcenter1.example.com",
				Valid:   false,
				Error:   "connection failed",
			},
			expectNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.result.GetErrorMessage()
			if tt.expectNonEmpty && msg == "" {
				t.Errorf("GetErrorMessage() = empty, want non-empty error message")
			}
			if !tt.expectNonEmpty && msg != "" {
				t.Errorf("GetErrorMessage() = %s, want empty", msg)
			}
		})
	}
}
