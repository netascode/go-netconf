// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"strings"
	"testing"
)

// TestStandardErrors tests the standard error variables
func TestStandardErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrLockReleaseTimeout", ErrLockReleaseTimeout, "lock release timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Error("expected non-nil error")
			}
			if !strings.Contains(tt.err.Error(), tt.msg) {
				t.Errorf("expected error message to contain %q, got %q", tt.msg, tt.err.Error())
			}
		})
	}
}

// TestNetconfErrorError tests the Error() method
func TestNetconfErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      *NetconfError
		contains []string
	}{
		{
			name: "simple error",
			err: &NetconfError{
				Operation: "get",
				Message:   "operation failed",
			},
			contains: []string{"netconf:", "get", "failed"},
		},
		{
			name: "error with retries",
			err: &NetconfError{
				Operation: "edit-config",
				Message:   "timeout",
				Retries:   3,
			},
			contains: []string{"netconf:", "edit-config", "timeout", "retries: 3"},
		},
		{
			name: "error without retries",
			err: &NetconfError{
				Operation: "commit",
				Message:   "configuration error",
				Retries:   0,
			},
			contains: []string{"netconf:", "commit", "configuration error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			for _, substr := range tt.contains {
				if !strings.Contains(errMsg, substr) {
					t.Errorf("expected error message to contain %q, got %q", substr, errMsg)
				}
			}
		})
	}
}

// TestNetconfErrorDetailedError tests the DetailedError() method
func TestNetconfErrorDetailedError(t *testing.T) {
	tests := []struct {
		name     string
		err      *NetconfError
		contains []string
	}{
		{
			name: "error with internal message",
			err: &NetconfError{
				Operation:   "get",
				Message:     "operation failed",
				InternalMsg: "connection reset by peer",
			},
			contains: []string{"netconf:", "get", "failed", "internal:", "connection reset"},
		},
		{
			name: "error with internal message and retries",
			err: &NetconfError{
				Operation:   "lock",
				Message:     "lock denied",
				InternalMsg: "resource in use by session 1234",
				Retries:     5,
			},
			contains: []string{"netconf:", "lock", "denied", "internal:", "session 1234", "retries: 5"},
		},
		{
			name: "error without internal message",
			err: &NetconfError{
				Operation: "commit",
				Message:   "configuration error",
			},
			contains: []string{"netconf:", "commit", "configuration error"},
		},
		{
			name: "error with empty internal message",
			err: &NetconfError{
				Operation:   "validate",
				Message:     "validation failed",
				InternalMsg: "",
			},
			contains: []string{"netconf:", "validate", "validation failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.DetailedError()
			for _, substr := range tt.contains {
				if !strings.Contains(errMsg, substr) {
					t.Errorf("expected detailed error to contain %q, got %q", substr, errMsg)
				}
			}
		})
	}
}

// TestErrorModelStructure tests the ErrorModel struct
func TestErrorModelStructure(t *testing.T) {
	model := ErrorModel{
		ErrorType:     "application",
		ErrorTag:      "operation-failed",
		ErrorSeverity: "error",
		ErrorAppTag:   "custom-error",
		ErrorPath:     "/config/interfaces",
		ErrorMessage:  "Invalid configuration",
		ErrorInfo:     "Additional details",
	}

	if model.ErrorType != "application" {
		t.Errorf("expected ErrorType 'application', got %q", model.ErrorType)
	}
	if model.ErrorTag != "operation-failed" {
		t.Errorf("expected ErrorTag 'operation-failed', got %q", model.ErrorTag)
	}
	if model.ErrorSeverity != "error" {
		t.Errorf("expected ErrorSeverity 'error', got %q", model.ErrorSeverity)
	}
	if model.ErrorAppTag != "custom-error" {
		t.Errorf("expected ErrorAppTag 'custom-error', got %q", model.ErrorAppTag)
	}
	if model.ErrorPath != "/config/interfaces" {
		t.Errorf("expected ErrorPath '/config/interfaces', got %q", model.ErrorPath)
	}
	if model.ErrorMessage != "Invalid configuration" {
		t.Errorf("expected ErrorMessage 'Invalid configuration', got %q", model.ErrorMessage)
	}
	if model.ErrorInfo != "Additional details" {
		t.Errorf("expected ErrorInfo 'Additional details', got %q", model.ErrorInfo)
	}
}

// TestTransientErrorPatterns tests the TransientError pattern matching
func TestTransientErrorPatterns(t *testing.T) {
	// Verify TransientErrors is not empty
	if len(TransientErrors) == 0 {
		t.Error("expected TransientErrors to have patterns defined")
	}

	// Verify expected patterns exist (only confirmed patterns)
	expectedPatterns := map[string]bool{
		"lock-denied":      false,
		"in-use":           false,
		"transport-errors": false,
	}

	for _, pattern := range TransientErrors {
		if pattern.ErrorTag == "lock-denied" {
			expectedPatterns["lock-denied"] = true
		}
		if pattern.ErrorTag == "in-use" {
			expectedPatterns["in-use"] = true
		}
		if pattern.ErrorType == "transport" {
			expectedPatterns["transport-errors"] = true
		}
	}

	for pattern, found := range expectedPatterns {
		if !found {
			t.Errorf("expected pattern %q to be in TransientErrors", pattern)
		}
	}

	// Verify we have exactly 3 patterns (no speculative patterns)
	if len(TransientErrors) != 3 {
		t.Errorf("expected exactly 3 transient error patterns, got %d", len(TransientErrors))
	}
}

// TestNetconfErrorWithMultipleErrors tests NetconfError with multiple error models
func TestNetconfErrorWithMultipleErrors(t *testing.T) {
	netconfErr := &NetconfError{
		Operation: "edit-config",
		Message:   "multiple errors occurred",
		Errors: []ErrorModel{
			{
				ErrorType:    "application",
				ErrorTag:     "invalid-value",
				ErrorMessage: "Invalid hostname",
			},
			{
				ErrorType:    "application",
				ErrorTag:     "missing-attribute",
				ErrorMessage: "Required attribute missing",
			},
		},
	}

	if len(netconfErr.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(netconfErr.Errors))
	}
}

// TestNetconfErrorAsError tests that NetconfError implements error interface
func TestNetconfErrorAsError(t *testing.T) {
	var err error = &NetconfError{
		Operation: "test",
		Message:   "test error",
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestNetconfErrorTransientFlag tests the IsTransient flag
func TestNetconfErrorTransientFlag(t *testing.T) {
	tests := []struct {
		name        string
		isTransient bool
	}{
		{"transient error", true},
		{"non-transient error", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &NetconfError{
				Operation:   "test",
				Message:     "test",
				IsTransient: tt.isTransient,
			}

			if err.IsTransient != tt.isTransient {
				t.Errorf("expected IsTransient %v, got %v", tt.isTransient, err.IsTransient)
			}
		})
	}
}

// TestErrorModelZeroValues tests ErrorModel with zero values
func TestErrorModelZeroValues(t *testing.T) {
	model := ErrorModel{}

	if model.ErrorType != "" {
		t.Error("expected empty ErrorType")
	}
	if model.ErrorTag != "" {
		t.Error("expected empty ErrorTag")
	}
	if model.ErrorSeverity != "" {
		t.Error("expected empty ErrorSeverity")
	}
	if model.ErrorMessage != "" {
		t.Error("expected empty ErrorMessage")
	}
}

// TestTransientErrorStructure tests the TransientError struct
func TestTransientErrorStructure(t *testing.T) {
	pattern := TransientError{
		ErrorType:    "transport",
		ErrorTag:     "operation-failed",
		ErrorMessage: ".*timeout.*",
	}

	if pattern.ErrorType != "transport" {
		t.Errorf("expected ErrorType 'transport', got %q", pattern.ErrorType)
	}
	if pattern.ErrorTag != "operation-failed" {
		t.Errorf("expected ErrorTag 'operation-failed', got %q", pattern.ErrorTag)
	}
	if pattern.ErrorMessage != ".*timeout.*" {
		t.Errorf("expected ErrorMessage '.*timeout.*', got %q", pattern.ErrorMessage)
	}
}

// TestNetconfErrorWithEmptyFields tests NetconfError with empty fields
func TestNetconfErrorWithEmptyFields(t *testing.T) {
	err := &NetconfError{}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "netconf:") {
		t.Errorf("expected error message to contain 'netconf:', got %q", errMsg)
	}

	detailedMsg := err.DetailedError()
	if !strings.Contains(detailedMsg, "netconf:") {
		t.Errorf("expected detailed error to contain 'netconf:', got %q", detailedMsg)
	}
}

// TestNetconfErrorRetryCount tests various retry counts
func TestNetconfErrorRetryCount(t *testing.T) {
	tests := []struct {
		name    string
		retries int
	}{
		{"no retries", 0},
		{"one retry", 1},
		{"multiple retries", 5},
		{"many retries", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &NetconfError{
				Operation: "test",
				Message:   "test",
				Retries:   tt.retries,
			}

			errMsg := err.Error()
			if tt.retries > 0 {
				if !strings.Contains(errMsg, "retries:") {
					t.Error("expected error message to contain retry count")
				}
			} else {
				if strings.Contains(errMsg, "retries:") {
					t.Error("expected error message not to contain retry count for zero retries")
				}
			}
		})
	}
}

// TestErrorModelRFC6241Fields tests ErrorModel fields match RFC 6241
func TestErrorModelRFC6241Fields(t *testing.T) {
	// Test valid error types per RFC 6241
	validErrorTypes := []string{"transport", "rpc", "protocol", "application"}
	for _, errorType := range validErrorTypes {
		model := ErrorModel{ErrorType: errorType}
		if model.ErrorType != errorType {
			t.Errorf("expected ErrorType %q, got %q", errorType, model.ErrorType)
		}
	}

	// Test valid severities per RFC 6241
	validSeverities := []string{"error", "warning"}
	for _, severity := range validSeverities {
		model := ErrorModel{ErrorSeverity: severity}
		if model.ErrorSeverity != severity {
			t.Errorf("expected ErrorSeverity %q, got %q", severity, model.ErrorSeverity)
		}
	}
}

// TestIsLockDeniedError tests the isLockDeniedError helper method
func TestIsLockDeniedError(t *testing.T) {
	tests := []struct {
		name          string
		errors        []ErrorModel
		expectLockDenied bool
	}{
		{
			name: "lock-denied error",
			errors: []ErrorModel{
				{ErrorTag: "lock-denied"},
			},
			expectLockDenied: true,
		},
		{
			name: "in-use error",
			errors: []ErrorModel{
				{ErrorTag: "in-use"},
			},
			expectLockDenied: true,
		},
		{
			name: "lock-denied with other errors",
			errors: []ErrorModel{
				{ErrorTag: "invalid-value"},
				{ErrorTag: "lock-denied"},
			},
			expectLockDenied: true,
		},
		{
			name: "non-lock error",
			errors: []ErrorModel{
				{ErrorTag: "invalid-value"},
			},
			expectLockDenied: false,
		},
		{
			name: "transport error",
			errors: []ErrorModel{
				{ErrorType: "transport"},
			},
			expectLockDenied: false,
		},
		{
			name:          "empty errors",
			errors:        []ErrorModel{},
			expectLockDenied: false,
		},
		{
			name:          "nil errors",
			errors:        nil,
			expectLockDenied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			result := client.isLockDeniedError(tt.errors)
			if result != tt.expectLockDenied {
				t.Errorf("expected isLockDeniedError %v, got %v", tt.expectLockDenied, result)
			}
		})
	}
}
