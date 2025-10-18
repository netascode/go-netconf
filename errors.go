// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"errors"
	"fmt"
)

// Standard errors that can be returned by the library
var (
	// ErrLockReleaseTimeout indicates that waiting for a lock release timed out
	ErrLockReleaseTimeout = errors.New("netconf: lock release timeout")
)

// NetconfError represents a structured NETCONF error with operation context
//
//nolint:revive // NetconfError name is intentional for domain-specific error type
type NetconfError struct {
	// Operation name that failed
	Operation string

	// Errors from NETCONF rpc-error elements
	Errors []ErrorModel

	// Human-readable error message (sanitized for client display)
	Message string

	// InternalMsg contains detailed error information for internal logging (MAJ-2)
	// This may include sensitive information and should only be used in secure logging contexts
	InternalMsg string

	// Number of retry attempts made
	Retries int

	// IsTransient indicates if the error is transient and was retried
	IsTransient bool
}

// Error implements the error interface and returns a sanitized error message
// suitable for display to clients or in logs that may be exposed.
func (e *NetconfError) Error() string {
	if e.Retries > 0 {
		return fmt.Sprintf("netconf: %s failed: %s (retries: %d)", e.Operation, e.Message, e.Retries)
	}
	return fmt.Sprintf("netconf: %s failed: %s", e.Operation, e.Message)
}

// DetailedError returns the full error message including internal details (MAJ-2)
// This should only be used in secure logging contexts where sensitive information
// disclosure is acceptable (e.g., server-side logs, debug output).
//
// Example:
//
//	if err != nil {
//	    if netconfErr, ok := err.(*NetconfError); ok {
//	        log.Debug(netconfErr.DetailedError()) // internal logging
//	        return netconfErr.Error() // client-facing error
//	    }
//	}
func (e *NetconfError) DetailedError() string {
	if e.InternalMsg == "" {
		return e.Error()
	}
	if e.Retries > 0 {
		return fmt.Sprintf("netconf: %s failed: %s (internal: %s, retries: %d)",
			e.Operation, e.Message, e.InternalMsg, e.Retries)
	}
	return fmt.Sprintf("netconf: %s failed: %s (internal: %s)",
		e.Operation, e.Message, e.InternalMsg)
}

// ErrorModel represents a NETCONF rpc-error element as defined in RFC 6241
type ErrorModel struct {
	// ErrorType indicates the conceptual layer where the error occurred
	// Valid values: transport, rpc, protocol, application
	ErrorType string

	// ErrorTag contains a string identifying the error condition
	// Examples: in-use, invalid-value, missing-attribute, etc.
	ErrorTag string

	// ErrorSeverity indicates the severity of the error
	// Valid values: error, warning
	ErrorSeverity string

	// ErrorAppTag contains an application-specific error tag
	ErrorAppTag string

	// ErrorPath contains the absolute XPath expression identifying the element in error
	ErrorPath string

	// ErrorMessage contains a human-readable error message
	ErrorMessage string

	// ErrorInfo contains additional error information
	ErrorInfo string
}

// TransientError defines patterns for detecting transient errors that should be retried
type TransientError struct {
	// ErrorType to match (empty matches any)
	ErrorType string

	// ErrorTag to match (empty matches any)
	ErrorTag string

	// ErrorMessage regex pattern to match (empty matches any)
	ErrorMessage string
}

// TransientErrors defines the list of error patterns that should trigger automatic retry
//
// These errors are typically caused by temporary conditions such as:
//   - Datastore locks held by other sessions (lock-denied, in-use)
//   - Transport-level failures (connection drops, session timeouts)
//
// Note: Only includes patterns confirmed by RFC 6241 and observed device behavior.
// Additional patterns can be added as they are confirmed with real devices.
var TransientErrors = []TransientError{
	// Lock conflicts (RFC 6241 Section 4.3)
	{ErrorTag: "lock-denied"},
	{ErrorTag: "in-use"},

	// Transport errors (RFC 6241 Section 4.3)
	{ErrorType: "transport"},
}
