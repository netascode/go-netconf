// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"sync"
	"testing"
	"time"
)

// TestConcurrentServerCapabilities tests that ServerCapabilities can be called concurrently
func TestConcurrentServerCapabilities(t *testing.T) {
	client := &Client{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:ietf:params:netconf:capability:candidate:1.0",
			"urn:ietf:params:netconf:capability:xpath:1.0",
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			caps := client.ServerCapabilities()
			if len(caps) != 3 {
				t.Errorf("expected 3 capabilities, got %d", len(caps))
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentServerHasCapability tests concurrent capability checks
func TestConcurrentServerHasCapability(t *testing.T) {
	client := &Client{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:ietf:params:netconf:capability:candidate:1.0",
		},
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !client.ServerHasCapability("urn:ietf:params:netconf:base:1.0") {
				t.Error("expected base capability to be present")
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentBackoffCalculation tests concurrent backoff calculations
func TestConcurrentBackoffCalculation(t *testing.T) {
	client := &Client{
		BackoffMinDelay:    1 * time.Second,
		BackoffMaxDelay:    60 * time.Second,
		BackoffDelayFactor: 2.0,
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(attempt int) {
			defer wg.Done()
			delay := client.Backoff(attempt)
			if delay < 0 {
				t.Errorf("backoff delay should not be negative, got %v", delay)
			}
		}(i % 10) // Use different attempts
	}

	wg.Wait()
}

// TestConcurrentHasCredentials tests concurrent credential checks
func TestConcurrentHasCredentials(t *testing.T) {
	client := &Client{
		username: "admin",
		password: "secret",
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if !client.HasCredentials() {
				t.Error("expected client to have credentials")
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentSessionIDAccess tests concurrent session ID access
func TestConcurrentSessionIDAccess(t *testing.T) {
	client := &Client{
		sessionID: "12345",
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := client.SessionID()
			if id != "12345" {
				t.Errorf("expected session ID '12345', got %q", id)
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentServerVersionAccess tests concurrent server version access
func TestConcurrentServerVersionAccess(t *testing.T) {
	client := &Client{
		serverVersion: "1.1",
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			version := client.ServerVersion()
			if version != "1.1" {
				t.Errorf("expected server version '1.1', got %q", version)
			}
		}()
	}

	wg.Wait()
}

// TestConcurrentCheckTransientError tests concurrent transient error checking
func TestConcurrentCheckTransientError(t *testing.T) {
	client := &Client{}

	errors1 := []ErrorModel{{ErrorTag: "lock-denied"}}
	errors2 := []ErrorModel{{ErrorTag: "invalid-value"}}

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var errors []ErrorModel
			if idx%2 == 0 {
				errors = errors1
			} else {
				errors = errors2
			}

			result := client.checkTransientError(errors, nil)
			if idx%2 == 0 && !result {
				t.Error("expected lock-denied to be transient")
			}
			if idx%2 == 1 && result {
				t.Error("expected invalid-value to not be transient")
			}
		}(i)
	}

	wg.Wait()
}

// TestConcurrentCapabilityModification tests that modifications don't affect
// concurrent readers (copy-on-read pattern)
func TestConcurrentCapabilityModification(t *testing.T) {
	client := &Client{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
		},
	}

	var wg sync.WaitGroup
	numReaders := 50

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				caps := client.ServerCapabilities()
				// Try to modify the returned slice (shouldn't affect other readers)
				if len(caps) > 0 {
					caps[0] = "modified"
				}
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Verify original capabilities unchanged
	if client.Capabilities[0] != "urn:ietf:params:netconf:base:1.0" {
		t.Error("original capabilities should not be modified")
	}
}

// TestRaceInClose tests that Close is safe when called concurrently
func TestRaceInClose(_ *testing.T) {
	// Note: This test primarily validates that no race conditions occur
	// when Close() is called multiple times concurrently
	client := &Client{
		driver: nil, // nil driver simulates already closed connection
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.Close() //nolint:errcheck // Test cleanup
		}()
	}

	wg.Wait()
}

// TestConcurrentParseRPCErrors tests concurrent RPC error parsing
func TestConcurrentParseRPCErrors(t *testing.T) {
	client := &Client{}

	responseXML := `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <rpc-error>
    <error-type>application</error-type>
    <error-tag>operation-failed</error-tag>
    <error-severity>error</error-severity>
    <error-message>Test error</error-message>
  </rpc-error>
</rpc-reply>`

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors := client.parseRPCErrors(responseXML)
			if len(errors) != 1 {
				t.Errorf("expected 1 error, got %d", len(errors))
			}
		}()
	}

	wg.Wait()
}
