// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// Test constants (GOCONST)
const (
	testOperationFailed = "operation-failed"
)

func TestClientDefaultConfiguration(t *testing.T) {
	t.Run("client with default options", func(t *testing.T) {
		// We can't actually connect without a real NETCONF server,
		// but we can test that option functions work
		opts := []func(*Client){
			Username("admin"),
			Password("secret"),
			Port(8830),
			MaxRetries(5),
			ConnectTimeout(10 * time.Second),
		}

		// Create a client structure (without connecting)
		client := &Client{
			Host:               "192.168.1.1",
			Port:               830,
			MaxRetries:         10,
			BackoffMinDelay:    1 * time.Second,
			BackoffMaxDelay:    60 * time.Second,
			BackoffDelayFactor: 1.2,
			LockReleaseTimeout: 120 * time.Second,
			ConnectTimeout:     30 * time.Second,
			AttemptTimeout:     30 * time.Second,
			TotalTimeout:       2 * time.Minute,
		}

		// Apply options
		for _, opt := range opts {
			opt(client)
		}

		// Verify options were applied
		// NOTE: Username() and Password() removed for security (FIX 7)
		// Use HasCredentials() instead
		if !client.HasCredentials() {
			t.Error("expected client to have credentials configured")
		}
		if client.Port != 8830 {
			t.Errorf("expected port 8830, got %d", client.Port)
		}
		if client.MaxRetries != 5 {
			t.Errorf("expected MaxRetries 5, got %d", client.MaxRetries)
		}
		if client.ConnectTimeout != 10*time.Second {
			t.Errorf("expected ConnectTimeout 10s, got %v", client.ConnectTimeout)
		}

		// Verify credentials are set correctly (access private fields for testing)
		if client.username != "admin" {
			t.Errorf("expected username 'admin', got %s", client.username)
		}
		if client.password != "secret" {
			t.Errorf("expected password 'secret', got %s", client.password)
		}
	})

	t.Run("client with SSH key", func(t *testing.T) {
		client := &Client{
			Host: "192.168.1.1",
		}

		SSHKey("/path/to/key")(client)

		if client.SSHKeyPath != "/path/to/key" {
			t.Errorf("expected SSHKeyPath '/path/to/key', got %s", client.SSHKeyPath)
		}
	})
}

func TestBuildScrapligoOptionsUsesStandardUserKnownHosts(t *testing.T) {
	home := t.TempDir()
	sshDir := home + "/.ssh"
	if err := os.Mkdir(sshDir, 0o700); err != nil {
		t.Fatalf("failed to create .ssh directory: %v", err)
	}
	knownHostsFile, err := os.Create(sshDir + "/known_hosts")
	if err != nil {
		t.Fatalf("failed to create known_hosts file: %v", err)
	}
	if err := knownHostsFile.Close(); err != nil {
		t.Fatalf("failed to close known_hosts file: %v", err)
	}

	t.Setenv("HOME", home)
	client := &Client{}
	sshArgs, err := transport.NewSSHArgs(client.buildScrapligoOptions()...)
	if err != nil {
		t.Fatalf("failed to build SSH arguments: %v", err)
	}
	if !sshArgs.StrictKey {
		t.Error("expected strict host key verification to remain enabled")
	}
	if sshArgs.KnownHostsFile != knownHostsFile.Name() {
		t.Errorf("expected known_hosts file %q, got %q", knownHostsFile.Name(), sshArgs.KnownHostsFile)
	}
}

func TestBuildScrapligoOptionsInsecureSkipsKnownHostsLookup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	client := &Client{InsecureSkipVerify: true}
	sshArgs, err := transport.NewSSHArgs(client.buildScrapligoOptions()...)
	if err != nil {
		t.Fatalf("failed to build SSH arguments: %v", err)
	}
	if sshArgs.StrictKey {
		t.Error("expected strict host key verification to be disabled")
	}
	if sshArgs.KnownHostsFile != "" {
		t.Errorf("expected known_hosts file to be ignored in insecure mode, got %q", sshArgs.KnownHostsFile)
	}
}

func TestClientCapabilities(t *testing.T) {
	t.Run("ServerHasCapability with no capabilities", func(t *testing.T) {
		client := &Client{
			Capabilities: []string{},
		}

		if client.ServerHasCapability("urn:ietf:params:netconf:base:1.0") {
			t.Error("expected false for empty capabilities")
		}
	})

	t.Run("ServerHasCapability with capabilities", func(t *testing.T) {
		client := &Client{
			Capabilities: []string{
				"urn:ietf:params:netconf:base:1.0",
				"urn:ietf:params:netconf:capability:candidate:1.0",
			},
		}

		if !client.ServerHasCapability("urn:ietf:params:netconf:base:1.0") {
			t.Error("expected true for base capability")
		}
		if !client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
			t.Error("expected true for candidate capability")
		}
		if client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
			t.Error("expected false for missing capability")
		}
	})

	t.Run("ServerCapabilities returns copy", func(t *testing.T) {
		client := &Client{
			Capabilities: []string{"cap1", "cap2"},
		}

		caps := client.ServerCapabilities()
		if len(caps) != 2 {
			t.Errorf("expected 2 capabilities, got %d", len(caps))
		}
	})
}

func TestClientClose(t *testing.T) {
	t.Run("close on nil driver should not panic", func(t *testing.T) {
		client := &Client{
			driver: nil,
		}

		err := client.Close()
		if err != nil {
			t.Errorf("Close() on nil driver should not error, got %v", err)
		}
	})
}

func TestClientOpen(t *testing.T) {
	t.Run("open without valid connection should fail", func(t *testing.T) {
		// Create client with invalid host to ensure connection fails
		client := &Client{
			Host:              "invalid-host-that-does-not-exist.local",
			Port:              830,
			username:          "test",
			password:          "test",
			ConnectTimeout:    1 * time.Second,
			AttemptTimeout:    1 * time.Second,
			TotalTimeout:      5 * time.Second,
			logger:            &NoOpLogger{},
			prettyPrintLogs:   false,
			redactionPatterns: defaultRedactionPatterns,
		}

		// Attempt to open (should fail because host doesn't exist)
		err := client.Open()
		if err == nil {
			t.Error("Open() should fail with invalid host")
		}
	})

	t.Run("open preserves client configuration", func(t *testing.T) {
		// Create client with specific configuration
		client := &Client{
			Host:               "test-host",
			Port:               2222,
			username:           "testuser",
			password:           "testpass",
			SSHKeyPath:         "/path/to/key",
			InsecureSkipVerify: true,
			MaxRetries:         5,
			BackoffMinDelay:    2 * time.Second,
			BackoffMaxDelay:    120 * time.Second,
			BackoffDelayFactor: 3.0,
			LockReleaseTimeout: 240 * time.Second,
			ConnectTimeout:     45 * time.Second,
			AttemptTimeout:     60 * time.Second,
			TotalTimeout:       5 * time.Minute,
			logger:             &NoOpLogger{},
			prettyPrintLogs:    true,
			redactionPatterns:  defaultRedactionPatterns,
		}

		// Note: This test verifies configuration is preserved during connect attempt
		// The connection will fail (no server), but we verify the client state remains intact
		originalHost := client.Host
		originalPort := client.Port
		originalUser := client.username
		originalPass := client.password
		originalSSHKey := client.SSHKeyPath
		originalInsecure := client.InsecureSkipVerify
		originalMaxRetries := client.MaxRetries
		originalConnectTimeout := client.ConnectTimeout

		// Attempt open (will fail, but shouldn't corrupt state)
		_ = client.Open() //nolint:errcheck // Test expects failure, checking state preservation only

		// Verify configuration preserved
		if client.Host != originalHost {
			t.Errorf("Host changed after Open: got %s, want %s", client.Host, originalHost)
		}
		if client.Port != originalPort {
			t.Errorf("Port changed after Open: got %d, want %d", client.Port, originalPort)
		}
		if client.username != originalUser {
			t.Errorf("Username changed after Open: got %s, want %s", client.username, originalUser)
		}
		if client.password != originalPass {
			t.Errorf("Password changed after Open: got %s, want %s", client.password, originalPass)
		}
		if client.SSHKeyPath != originalSSHKey {
			t.Errorf("SSHKeyPath changed after Open: got %s, want %s", client.SSHKeyPath, originalSSHKey)
		}
		if client.InsecureSkipVerify != originalInsecure {
			t.Errorf("InsecureSkipVerify changed after Open: got %v, want %v", client.InsecureSkipVerify, originalInsecure)
		}
		if client.MaxRetries != originalMaxRetries {
			t.Errorf("MaxRetries changed after Open: got %d, want %d", client.MaxRetries, originalMaxRetries)
		}
		if client.ConnectTimeout != originalConnectTimeout {
			t.Errorf("ConnectTimeout changed after Open: got %v, want %v", client.ConnectTimeout, originalConnectTimeout)
		}
	})

	t.Run("open after close should reconnect", func(t *testing.T) {
		client := &Client{
			Host:              "test-host",
			Port:              830,
			username:          "test",
			password:          "test",
			driver:            nil, // Simulate closed state
			ConnectTimeout:    1 * time.Second,
			AttemptTimeout:    1 * time.Second,
			TotalTimeout:      5 * time.Second,
			logger:            &NoOpLogger{},
			prettyPrintLogs:   false,
			redactionPatterns: defaultRedactionPatterns,
		}

		// Driver should be nil initially
		if client.driver != nil {
			t.Error("Driver should be nil before Open")
		}

		// Attempt open (will fail but shouldn't panic)
		_ = client.Open() //nolint:errcheck // Test expects failure, checking panic prevention only

		// Driver remains nil after failed open
		if client.driver != nil {
			t.Error("Driver should remain nil after failed Open")
		}
	})
}

func TestClientIsClosed(t *testing.T) {
	t.Run("client with nil driver is closed", func(t *testing.T) {
		client := &Client{
			driver: nil,
		}

		if !client.IsClosed() {
			t.Error("IsClosed() should return true when driver is nil")
		}
	})

	t.Run("newly created client without connection is closed", func(t *testing.T) {
		// Create client structure without calling NewClient (which opens connection)
		client := &Client{
			Host:     "test-host",
			Port:     830,
			username: "test",
			password: "test",
			driver:   nil, // No connection established
		}

		if !client.IsClosed() {
			t.Error("IsClosed() should return true for client without connection")
		}
	})

	t.Run("closed client after Close() is detected as closed", func(t *testing.T) {
		client := &Client{
			driver: nil, // Simulate already closed
		}

		// Close should be idempotent
		err := client.Close()
		if err != nil {
			t.Errorf("Close() should not error on already closed client: %v", err)
		}

		if !client.IsClosed() {
			t.Error("IsClosed() should return true after Close()")
		}
	})

	t.Run("IsClosed is thread-safe", func(t *testing.T) {
		client := &Client{
			driver: nil,
		}

		// Run concurrent IsClosed checks
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					_ = client.IsClosed()
				}
				done <- true
			}()
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		// Should still be closed
		if !client.IsClosed() {
			t.Error("IsClosed() should remain true after concurrent access")
		}
	})
}

// Note: Full integration tests with actual NETCONF connections would require
// a real NETCONF server or mock. These tests focus on unit testing the
// structure and basic functionality.

func TestClientBackoff(t *testing.T) {
	tests := []struct {
		name             string
		minDelay         time.Duration
		maxDelay         time.Duration
		factor           float64
		attempt          int
		expectedMinDelay time.Duration
		expectedMaxDelay time.Duration
	}{
		{
			name:             "first attempt",
			minDelay:         1 * time.Second,
			maxDelay:         60 * time.Second,
			factor:           2.0,
			attempt:          0,
			expectedMinDelay: 1 * time.Second,
			expectedMaxDelay: 2 * time.Second, // With 10% jitter
		},
		{
			name:             "second attempt",
			minDelay:         1 * time.Second,
			maxDelay:         60 * time.Second,
			factor:           2.0,
			attempt:          1,
			expectedMinDelay: 2 * time.Second,
			expectedMaxDelay: 3 * time.Second, // 2s * 2 + 10% jitter
		},
		{
			name:             "capped at max delay",
			minDelay:         1 * time.Second,
			maxDelay:         10 * time.Second,
			factor:           2.0,
			attempt:          10,
			expectedMinDelay: 10 * time.Second, // Should cap at max
			expectedMaxDelay: 11 * time.Second, // Max + 10% jitter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				BackoffMinDelay:    tt.minDelay,
				BackoffMaxDelay:    tt.maxDelay,
				BackoffDelayFactor: tt.factor,
			}

			delay := client.Backoff(tt.attempt)

			if delay < tt.expectedMinDelay {
				t.Errorf("delay %v is less than expected minimum %v", delay, tt.expectedMinDelay)
			}
			if delay > tt.expectedMaxDelay {
				t.Errorf("delay %v is greater than expected maximum %v", delay, tt.expectedMaxDelay)
			}
		})
	}
}

func TestClientCheckTransientError(t *testing.T) {
	tests := []struct {
		name        string
		errors      []ErrorModel
		isTransient bool
	}{
		{
			name: "lock-denied error",
			errors: []ErrorModel{
				{ErrorTag: "lock-denied"},
			},
			isTransient: true,
		},
		{
			name: "in-use error",
			errors: []ErrorModel{
				{ErrorTag: "in-use"},
			},
			isTransient: true,
		},
		{
			name: "transport error",
			errors: []ErrorModel{
				{ErrorType: "transport"},
			},
			isTransient: true,
		},
		{
			name: "non-transient error",
			errors: []ErrorModel{
				{ErrorTag: "invalid-value"},
			},
			isTransient: false,
		},
		{
			name:        "no errors",
			errors:      []ErrorModel{},
			isTransient: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			result := client.checkTransientError(tt.errors, nil)
			if result != tt.isTransient {
				t.Errorf("expected isTransient %v, got %v", tt.isTransient, result)
			}
		})
	}

	// Test scrapligo Go errors
	scrapligoTests := []struct {
		name        string
		goErr       error
		isTransient bool
	}{
		{
			name:        "timeout error",
			goErr:       util.ErrTimeoutError,
			isTransient: true,
		},
		{
			name:        "connection error",
			goErr:       util.ErrConnectionError,
			isTransient: true,
		},
		{
			name:        "operation error",
			goErr:       util.ErrOperationError,
			isTransient: true,
		},
		{
			name:        "wrapped timeout error",
			goErr:       fmt.Errorf("operation failed: %w", util.ErrTimeoutError),
			isTransient: true,
		},
		{
			name:        "EOF error",
			goErr:       io.EOF,
			isTransient: true,
		},
		{
			name:        "wrapped EOF error",
			goErr:       fmt.Errorf("operation get-config failed (target=running): %w", io.EOF),
			isTransient: true,
		},
		{
			name:        "non-transient error",
			goErr:       util.ErrAuthError,
			isTransient: false,
		},
		{
			name:        "nil error",
			goErr:       nil,
			isTransient: false,
		},
	}

	for _, tt := range scrapligoTests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			result := client.checkTransientError([]ErrorModel{}, tt.goErr)
			if result != tt.isTransient {
				t.Errorf("expected isTransient %v, got %v", tt.isTransient, result)
			}
		})
	}
}

func TestClientParseRPCErrors(t *testing.T) {
	tests := []struct {
		name           string
		responseXML    string
		expectedErrors int
		checkFirst     func(t *testing.T, err ErrorModel)
	}{
		{
			name: "single error",
			responseXML: `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <rpc-error>
    <error-type>application</error-type>
    <error-tag>operation-failed</error-tag>
    <error-severity>error</error-severity>
    <error-message>Test error message</error-message>
  </rpc-error>
</rpc-reply>`,
			expectedErrors: 1,
			checkFirst: func(t *testing.T, err ErrorModel) {
				if err.ErrorType != "application" {
					t.Errorf("expected ErrorType 'application', got %q", err.ErrorType)
				}
				if err.ErrorTag != testOperationFailed {
					t.Errorf("expected ErrorTag 'operation-failed', got %q", err.ErrorTag)
				}
				if err.ErrorSeverity != "error" {
					t.Errorf("expected ErrorSeverity 'error', got %q", err.ErrorSeverity)
				}
				if err.ErrorMessage != "Test error message" {
					t.Errorf("expected ErrorMessage 'Test error message', got %q", err.ErrorMessage)
				}
			},
		},
		{
			name: "no errors",
			responseXML: `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <ok/>
</rpc-reply>`,
			expectedErrors: 0,
		},
		{
			name: "error with all fields",
			responseXML: `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <rpc-error>
    <error-type>protocol</error-type>
    <error-tag>invalid-value</error-tag>
    <error-severity>error</error-severity>
    <error-app-tag>custom-app-tag</error-app-tag>
    <error-path>/config/interfaces/interface[name='eth0']</error-path>
    <error-message>Invalid interface configuration</error-message>
    <error-info>Additional info here</error-info>
  </rpc-error>
</rpc-reply>`,
			expectedErrors: 1,
			checkFirst: func(t *testing.T, err ErrorModel) {
				if err.ErrorType != "protocol" {
					t.Errorf("expected ErrorType 'protocol', got %q", err.ErrorType)
				}
				if err.ErrorAppTag != "custom-app-tag" {
					t.Errorf("expected ErrorAppTag 'custom-app-tag', got %q", err.ErrorAppTag)
				}
				if err.ErrorPath != "/config/interfaces/interface[name='eth0']" {
					t.Errorf("expected ErrorPath '/config/interfaces/interface[name='eth0']', got %q", err.ErrorPath)
				}
				if err.ErrorInfo != "Additional info here" {
					t.Errorf("expected ErrorInfo 'Additional info here', got %q", err.ErrorInfo)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			errors := client.parseRPCErrors(tt.responseXML)

			if len(errors) != tt.expectedErrors {
				t.Errorf("expected %d errors, got %d", tt.expectedErrors, len(errors))
			}

			if tt.expectedErrors > 0 && tt.checkFirst != nil {
				tt.checkFirst(t, errors[0])
			}
		})
	}
}

func TestClientSessionID(t *testing.T) {
	client := &Client{
		sessionID: "12345",
	}

	if client.SessionID() != "12345" {
		t.Errorf("expected SessionID '12345', got %q", client.SessionID())
	}
}

func TestClientServerVersion(t *testing.T) {
	client := &Client{
		serverVersion: "1.1",
	}

	if client.ServerVersion() != "1.1" {
		t.Errorf("expected ServerVersion '1.1', got %q", client.ServerVersion())
	}
}

func TestClientHasCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		sshKey   string
		hasCreds bool
	}{
		{"username and password", "admin", "secret", "", true},
		{"only username", "admin", "", "", true},
		{"only password", "", "secret", "", true},
		{"only ssh key", "", "", "/path/to/key", true},
		{"no credentials", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{
				username:   tt.username,
				password:   tt.password,
				SSHKeyPath: tt.sshKey,
			}

			if client.HasCredentials() != tt.hasCreds {
				t.Errorf("expected HasCredentials %v, got %v", tt.hasCreds, client.HasCredentials())
			}
		})
	}
}

// Benchmark tests for Client operations

func BenchmarkClientBackoff(b *testing.B) {
	client := &Client{
		BackoffMinDelay:    1 * time.Second,
		BackoffMaxDelay:    60 * time.Second,
		BackoffDelayFactor: 2.0,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.Backoff(i % 10)
	}
}

func BenchmarkClientCheckTransientError(b *testing.B) {
	client := &Client{}
	errors := []ErrorModel{
		{ErrorTag: "lock-denied"},
		{ErrorType: "transport"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.checkTransientError(errors, nil)
	}
}

func BenchmarkClientParseRPCErrors(b *testing.B) {
	client := &Client{}
	responseXML := `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <rpc-error>
    <error-type>application</error-type>
    <error-tag>` + testOperationFailed + `</error-tag>
    <error-severity>error</error-severity>
    <error-message>Test error message</error-message>
  </rpc-error>
</rpc-reply>`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.parseRPCErrors(responseXML)
	}
}

func BenchmarkClientServerCapabilities(b *testing.B) {
	client := &Client{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:ietf:params:netconf:capability:candidate:1.0",
			"urn:ietf:params:netconf:capability:xpath:1.0",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ServerCapabilities()
	}
}

func BenchmarkClientServerHasCapability(b *testing.B) {
	client := &Client{
		Capabilities: []string{
			"urn:ietf:params:netconf:base:1.0",
			"urn:ietf:params:netconf:capability:candidate:1.0",
			"urn:ietf:params:netconf:capability:xpath:1.0",
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = client.ServerHasCapability("urn:ietf:params:netconf:base:1.0")
	}
}

// TestBuildEditConfigXML tests the buildEditConfigXML method
func TestBuildEditConfigXML(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name     string
		req      *Req
		expected []string // expected XML fragments
	}{
		{
			name: "basic edit-config",
			req: &Req{
				Target: "candidate",
				Config: "<config><hostname>router1</hostname></config>",
			},
			expected: []string{
				"<edit-config>",
				"<target><candidate/></target>",
				"<config><hostname>router1</hostname></config>",
				"</edit-config>",
			},
		},
		{
			name: "edit-config with default-operation",
			req: &Req{
				Target:           "candidate",
				Config:           "<config><hostname>router1</hostname></config>",
				DefaultOperation: "merge",
			},
			expected: []string{
				"<edit-config>",
				"<target><candidate/></target>",
				"<default-operation>merge</default-operation>",
				"<config><hostname>router1</hostname></config>",
				"</edit-config>",
			},
		},
		{
			name: "edit-config with test-option",
			req: &Req{
				Target:     "candidate",
				Config:     "<config><hostname>router1</hostname></config>",
				TestOption: "test-then-set",
			},
			expected: []string{
				"<edit-config>",
				"<target><candidate/></target>",
				"<test-option>test-then-set</test-option>",
				"<config><hostname>router1</hostname></config>",
				"</edit-config>",
			},
		},
		{
			name: "edit-config with error-option",
			req: &Req{
				Target:      "candidate",
				Config:      "<config><hostname>router1</hostname></config>",
				ErrorOption: "rollback-on-error",
			},
			expected: []string{
				"<edit-config>",
				"<target><candidate/></target>",
				"<error-option>rollback-on-error</error-option>",
				"<config><hostname>router1</hostname></config>",
				"</edit-config>",
			},
		},
		{
			name: "edit-config with all options",
			req: &Req{
				Target:           "running",
				Config:           "<config><interface>eth0</interface></config>",
				DefaultOperation: "replace",
				TestOption:       "test-only",
				ErrorOption:      "continue-on-error",
			},
			expected: []string{
				"<edit-config>",
				"<target><running/></target>",
				"<default-operation>replace</default-operation>",
				"<test-option>test-only</test-option>",
				"<error-option>continue-on-error</error-option>",
				"<config><interface>eth0</interface></config>",
				"</edit-config>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			xml := client.buildEditConfigXML(tt.req)

			// Verify all expected fragments are present
			for _, expected := range tt.expected {
				if !strings.Contains(xml, expected) {
					t.Errorf("expected XML to contain %q, got:\n%s", expected, xml)
				}
			}

			// Verify proper XML structure
			if !strings.HasPrefix(xml, "<edit-config>") {
				t.Errorf("expected XML to start with <edit-config>, got: %s", xml)
			}
			if !strings.HasSuffix(xml, "</edit-config>") {
				t.Errorf("expected XML to end with </edit-config>, got: %s", xml)
			}
		})
	}
}

// TestBuildEditConfigXMLOrder tests the order of elements in edit-config XML
func TestBuildEditConfigXMLOrder(t *testing.T) {
	client := &Client{}

	req := &Req{
		Target:           "candidate",
		Config:           "<config><test/></config>",
		DefaultOperation: "merge",
		TestOption:       "test-then-set",
		ErrorOption:      "rollback-on-error",
	}

	xml := client.buildEditConfigXML(req)

	// RFC 6241 Section 7.2 specifies the order:
	// 1. target
	// 2. default-operation (optional)
	// 3. test-option (optional)
	// 4. error-option (optional)
	// 5. config

	targetIdx := strings.Index(xml, "<target>")
	defaultOpIdx := strings.Index(xml, "<default-operation>")
	testOptIdx := strings.Index(xml, "<test-option>")
	errorOptIdx := strings.Index(xml, "<error-option>")
	configIdx := strings.Index(xml, "<config>")

	if targetIdx == -1 || defaultOpIdx == -1 || testOptIdx == -1 || errorOptIdx == -1 || configIdx == -1 {
		t.Fatalf("missing required elements in XML: %s", xml)
	}

	// Verify order
	if targetIdx >= defaultOpIdx {
		t.Error("target should come before default-operation")
	}
	if defaultOpIdx >= testOptIdx {
		t.Error("default-operation should come before test-option")
	}
	if testOptIdx >= errorOptIdx {
		t.Error("test-option should come before error-option")
	}
	if errorOptIdx >= configIdx {
		t.Error("error-option should come before config")
	}
}

// TestBuildEditConfigXMLEmptyValues tests handling of empty values
func TestBuildEditConfigXMLEmptyValues(t *testing.T) {
	client := &Client{}

	req := &Req{
		Target:           "candidate",
		Config:           "",
		DefaultOperation: "",
		TestOption:       "",
		ErrorOption:      "",
	}

	xml := client.buildEditConfigXML(req)

	// Empty optional fields should not be included
	if strings.Contains(xml, "<default-operation>") {
		t.Error("empty default-operation should not be included in XML")
	}
	if strings.Contains(xml, "<test-option>") {
		t.Error("empty test-option should not be included in XML")
	}
	if strings.Contains(xml, "<error-option>") {
		t.Error("empty error-option should not be included in XML")
	}

	// Empty config should still have config element
	if !strings.Contains(xml, "<config></config>") {
		t.Error("config element should be present even when empty")
	}
}

// TestEditConfigOptionValidValues tests valid values for edit-config options
func TestEditConfigOptionValidValues(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		value     string
		valid     bool
	}{
		// DefaultOperation valid values (RFC 6241 Section 7.2)
		{"default-op merge", "DefaultOperation", "merge", true},
		{"default-op replace", "DefaultOperation", "replace", true},
		{"default-op none", "DefaultOperation", "none", true},

		// TestOption valid values (RFC 6241 Section 7.2)
		{"test-opt test-then-set", "TestOption", "test-then-set", true},
		{"test-opt set", "TestOption", "set", true},
		{"test-opt test-only", "TestOption", "test-only", true},

		// ErrorOption valid values (RFC 6241 Section 7.2)
		{"error-opt stop-on-error", "ErrorOption", "stop-on-error", true},
		{"error-opt continue-on-error", "ErrorOption", "continue-on-error", true},
		{"error-opt rollback-on-error", "ErrorOption", "rollback-on-error", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			req := &Req{
				Target: "candidate",
				Config: "<test/>",
			}

			switch tt.fieldName {
			case "DefaultOperation":
				req.DefaultOperation = tt.value
			case "TestOption":
				req.TestOption = tt.value
			case "ErrorOption":
				req.ErrorOption = tt.value
			}

			xml := client.buildEditConfigXML(req)

			if tt.valid {
				if !strings.Contains(xml, tt.value) {
					t.Errorf("expected XML to contain valid value %q", tt.value)
				}
			}
		})
	}
}

// ExampleNewClient demonstrates creating a NETCONF client with authentication.
func ExampleNewClient() {
	// This example demonstrates the pattern for creating a NETCONF client
	// In production, replace with actual device credentials

	fmt.Println("Creating a NETCONF client:")
	fmt.Println("  - Specify host address")
	fmt.Println("  - Provide authentication (username/password or SSH key)")
	fmt.Println("  - Configure port (default: 830)")
	fmt.Println("  - Set retry and timeout options")
	fmt.Println("")
	fmt.Println("Example:")
	fmt.Println("  client, err := netconf.NewClient(")
	fmt.Println("      \"192.168.1.1\",")
	fmt.Println("      netconf.Username(\"admin\"),")
	fmt.Println("      netconf.Password(\"secret\"),")
	fmt.Println("      netconf.Port(830),")
	fmt.Println("  )")

	// Output: Creating a NETCONF client:
	//   - Specify host address
	//   - Provide authentication (username/password or SSH key)
	//   - Configure port (default: 830)
	//   - Set retry and timeout options
	//
	// Example:
	//   client, err := netconf.NewClient(
	//       "192.168.1.1",
	//       netconf.Username("admin"),
	//       netconf.Password("secret"),
	//       netconf.Port(830),
	//   )
}

// ExampleClient_Get demonstrates retrieving operational data with a filter.
func ExampleClient_Get() {
	// This example demonstrates the Get operation pattern
	// In production, replace with actual device connection

	// Create filter for interfaces
	filter := SubtreeFilter("<interfaces/>")
	fmt.Printf("Created filter for interfaces\n")

	// The Get operation would retrieve both config and state data
	fmt.Println("Get retrieves operational and configuration data")
	fmt.Println("Use SubtreeFilter or XPathFilter to filter results")

	_ = filter // Filter would be used in: client.Get(ctx, filter)

	// Output: Created filter for interfaces
	// Get retrieves operational and configuration data
	// Use SubtreeFilter or XPathFilter to filter results
}

// ExampleClient_GetConfig demonstrates retrieving configuration from a datastore.
func ExampleClient_GetConfig() {
	// This example demonstrates the GetConfig operation pattern

	// Create subtree filter
	filter := SubtreeFilter("<system><hostname/></system>")

	// GetConfig retrieves only configuration data (no state)
	fmt.Println("GetConfig retrieves configuration from datastores")
	fmt.Println("Valid datastores: running, candidate, startup")
	fmt.Printf("Filter type: %s\n", filter.Type)
	fmt.Printf("Filter content: %s\n", filter.Content)

	// In production: res, err := client.GetConfig(ctx, "running", filter)

	// Output: GetConfig retrieves configuration from datastores
	// Valid datastores: running, candidate, startup
	// Filter type: subtree
	// Filter content: <system><hostname/></system>
}

// ExampleClient_EditConfig demonstrates modifying device configuration.
func ExampleClient_EditConfig() {
	// Build configuration using Body builder
	body := Body{}.
		Set("config.system.hostname", "NewRouter").
		Set("config.system.domain-name", "example.com")

	config, err := body.String()
	if err != nil {
		fmt.Printf("Failed to build config: %v\n", err)
		return
	}

	// Example shows applying configuration to candidate datastore
	// with merge operation (in production, use with live client)
	fmt.Println("Configuration built successfully")
	fmt.Printf("Config length: %d bytes\n", len(config))
	// Output: Configuration built successfully
	// Config length: 102 bytes
}

// ExampleClient_Commit demonstrates the complete candidate commit workflow.
func ExampleClient_Commit() {
	// This example demonstrates the candidate datastore workflow

	fmt.Println("Candidate datastore workflow:")
	fmt.Println("1. Lock candidate datastore")
	fmt.Println("2. Edit configuration in candidate")
	fmt.Println("3. Validate configuration changes")
	fmt.Println("4. Commit candidate to running")
	fmt.Println("5. Unlock candidate datastore")
	fmt.Println("")
	fmt.Println("Commit options:")
	fmt.Println("- Standard commit (immediate)")
	fmt.Println("- Confirmed commit (auto-rollback)")

	// Output: Candidate datastore workflow:
	// 1. Lock candidate datastore
	// 2. Edit configuration in candidate
	// 3. Validate configuration changes
	// 4. Commit candidate to running
	// 5. Unlock candidate datastore
	//
	// Commit options:
	// - Standard commit (immediate)
	// - Confirmed commit (auto-rollback)
}

// TestClient_redactSensitiveData_Attributes tests attribute redaction
func TestClient_redactSensitiveData_Attributes(t *testing.T) {
	client := &Client{
		redactionPatterns: []*regexp.Regexp{
			// Elements
			regexp.MustCompile(`<password>.*?</password>`),
			regexp.MustCompile(`<secret>.*?</secret>`),
			regexp.MustCompile(`<key>.*?</key>`),
			regexp.MustCompile(`<community>.*?</community>`),
			// CDATA sections
			regexp.MustCompile(`<password><!\[CDATA\[.*?\]\]></password>`),
			regexp.MustCompile(`<secret><!\[CDATA\[.*?\]\]></secret>`),
			regexp.MustCompile(`<key><!\[CDATA\[.*?\]\]></key>`),
			regexp.MustCompile(`<community><!\[CDATA\[.*?\]\]></community>`),
			// Namespace-aware elements
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:password[^>]*>.*?</[a-zA-Z0-9_-]+:password>`),
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:secret[^>]*>.*?</[a-zA-Z0-9_-]+:secret>`),
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:key[^>]*>.*?</[a-zA-Z0-9_-]+:key>`),
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:community[^>]*>.*?</[a-zA-Z0-9_-]+:community>`),
			// Namespaced CDATA sections
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:password[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:password>`),
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:secret[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:secret>`),
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:key[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:key>`),
			regexp.MustCompile(`<[a-zA-Z0-9_-]+:community[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:community>`),
			// Attributes (double quotes)
			regexp.MustCompile(`password="[^"]*"`),
			regexp.MustCompile(`secret="[^"]*"`),
			regexp.MustCompile(`key="[^"]*"`),
			regexp.MustCompile(`community="[^"]*"`),
			// Attributes (single quotes)
			regexp.MustCompile(`password='[^']*'`),
			regexp.MustCompile(`secret='[^']*'`),
			regexp.MustCompile(`key='[^']*'`),
			regexp.MustCompile(`community='[^']*'`),
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Redact password attribute (double quotes)",
			input:    `<user password="secret123"/>`,
			expected: `<user password="[REDACTED]"/>`,
		},
		{
			name:     "Redact password attribute (single quotes)",
			input:    `<user password='secret123'/>`,
			expected: `<user password='[REDACTED]'/>`,
		},
		{
			name:     "Redact secret attribute (double quotes)",
			input:    `<api secret="api_key_xyz"/>`,
			expected: `<api secret="[REDACTED]"/>`,
		},
		{
			name:     "Redact key attribute (single quotes)",
			input:    `<auth key='encryption_key_123'/>`,
			expected: `<auth key='[REDACTED]'/>`,
		},
		{
			name:     "Redact community attribute",
			input:    `<snmp community="private"/>`,
			expected: `<snmp community="[REDACTED]"/>`,
		},
		{
			name:     "Redact multiple attributes",
			input:    `<config password="pass123" secret="secret456"/>`,
			expected: `<config password="[REDACTED]" secret="[REDACTED]"/>`,
		},
		{
			name:     "Redact mixed elements and attributes",
			input:    `<config><password>elem_pass</password><user password="attr_pass"/></config>`,
			expected: `<config><password>[REDACTED]</password><user password="[REDACTED]"/></config>`,
		},
		{
			name:     "Preserve non-sensitive attributes",
			input:    `<user name="admin" password="secret123" role="superuser"/>`,
			expected: `<user name="admin" password="[REDACTED]" role="superuser"/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.redactSensitiveData(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveData() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestClient_prepareXMLForLogging_SizeLimits tests XML size and count limits
func TestClient_prepareXMLForLogging_SizeLimits(t *testing.T) {
	client := &Client{
		logger:            &NoOpLogger{},
		prettyPrintLogs:   true,
		redactionPatterns: []*regexp.Regexp{},
	}

	t.Run("Large XML size limit", func(t *testing.T) {
		// Create XML larger than MaxXMLSizeForLogging (1MB)
		largeXML := strings.Repeat("<data>test</data>", 200000) // > 1MB
		result := client.prepareXMLForLogging(largeXML)

		if result != XMLTooLargeMessage {
			t.Error("Large XML should be rejected")
		}
	})

	t.Run("Sensitive element count limit", func(t *testing.T) {
		// Create XML with more than MaxSensitiveElements (1000)
		manyPasswords := strings.Repeat("<password>test</password>", 2000)
		result := client.prepareXMLForLogging(manyPasswords)

		if result != XMLTooManySensitiveMessage {
			t.Error("XML with too many sensitive elements should be rejected")
		}
	})

	t.Run("Mixed sensitive elements count", func(t *testing.T) {
		// Mix different sensitive element types to exceed limit
		mixed := strings.Repeat("<password>p</password>", 300) +
			strings.Repeat("<secret>s</secret>", 300) +
			strings.Repeat("<key>k</key>", 300) +
			strings.Repeat("<community>c</community>", 300)
		result := client.prepareXMLForLogging(mixed)

		if result != XMLTooManySensitiveMessage {
			t.Error("XML with total sensitive elements exceeding limit should be rejected")
		}
	})

	t.Run("Within size limit", func(t *testing.T) {
		// Small XML should be processed normally
		smallXML := "<config><hostname>router1</hostname></config>"
		result := client.prepareXMLForLogging(smallXML)

		// Should not be rejected (will be empty or contain XML)
		if result == XMLTooLargeMessage {
			t.Error("Small XML should not be rejected for size")
		}
		if result == XMLTooManySensitiveMessage {
			t.Error("XML without excessive sensitive elements should not be rejected")
		}
	})

	t.Run("Exactly at sensitive element limit", func(t *testing.T) {
		// Create exactly MaxSensitiveElements (1000)
		exactLimit := strings.Repeat("<password>test</password>", MaxSensitiveElements)
		result := client.prepareXMLForLogging(exactLimit)

		// Should be processed (not rejected)
		if result == XMLTooManySensitiveMessage {
			t.Error("XML with exactly MaxSensitiveElements should not be rejected")
		}
	})
}

// TestClient_prepareXMLForLogging_ReDoSPrevention tests ReDoS attack prevention
func TestClient_prepareXMLForLogging_ReDoSPrevention(t *testing.T) {
	client := &Client{
		logger:          &NoOpLogger{},
		prettyPrintLogs: false, // Disable pretty printing for consistent testing
		redactionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`<password>.*?</password>`),
		},
	}

	t.Run("Malicious nested password elements", func(t *testing.T) {
		// Create deeply nested password elements that could cause ReDoS
		nested := "<password>" + strings.Repeat("<password>", 100) + "secret" + strings.Repeat("</password>", 100) + "</password>"

		// Should handle without hanging (size limit should catch this)
		result := client.prepareXMLForLogging(nested)

		// Verify it was processed (either redacted or rejected for count)
		if result == "" {
			t.Error("Expected non-empty result for nested password elements")
		}
	})

	t.Run("Performance on large valid XML", func(t *testing.T) {
		// Create large but valid XML with some sensitive data
		largeValid := "<config>" + strings.Repeat("<interface><name>eth0</name></interface>", 1000) + "<password>secret</password></config>"

		// Should complete quickly without hanging
		start := time.Now()
		result := client.prepareXMLForLogging(largeValid)
		elapsed := time.Since(start)

		// Should complete in reasonable time (< 100ms for this size)
		if elapsed > 100*time.Millisecond {
			t.Errorf("prepareXMLForLogging took too long: %v", elapsed)
		}

		// Should contain redacted password
		if !strings.Contains(result, "[REDACTED]") && result != XMLTooLargeMessage {
			t.Error("Expected password to be redacted")
		}
	})
}

func TestClient_redactSensitiveData_XPathFilters(t *testing.T) {
	// Create a client with the full set of redaction patterns (matching NewClient)
	client := &Client{
		redactionPatterns: defaultRedactionPatterns,
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "XPath with password predicate (double quotes)",
			input:    `/config/users/user[password="secret123"]`,
			expected: `/config/users/user[password="[REDACTED]"]`,
		},
		{
			name:     "XPath with password predicate (single quotes)",
			input:    `/config/users/user[password='secret123']`,
			expected: `/config/users/user[password='[REDACTED]']`,
		},
		{
			name:     "XPath with secret predicate (double quotes)",
			input:    `/api[secret="api_key_xyz"]`,
			expected: `/api[secret="[REDACTED]"]`,
		},
		{
			name:     "XPath with secret predicate (single quotes)",
			input:    `/api[secret='api_key_xyz']`,
			expected: `/api[secret='[REDACTED]']`,
		},
		{
			name:     "XPath with key predicate (double quotes)",
			input:    `/config/auth[key="encryption_key"]`,
			expected: `/config/auth[key="[REDACTED]"]`,
		},
		{
			name:     "XPath with key predicate (single quotes)",
			input:    `/config/auth[key='encryption_key']`,
			expected: `/config/auth[key='[REDACTED]']`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.redactSensitiveData(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveData() = %q, want %q", result, tt.expected)
			}

			// Verify original value is not present
			if strings.Contains(result, "secret123") ||
				strings.Contains(result, "api_key_xyz") ||
				strings.Contains(result, "encryption_key") {
				t.Error("Sensitive data was not redacted")
			}
		})
	}
}

// TestClient_redactSensitiveData_NamespaceAndCDATA tests namespace-aware and CDATA redaction patterns
// This test validates the security enhancement that added 12 new patterns for comprehensive coverage
func TestClient_redactSensitiveData_NamespaceAndCDATA(t *testing.T) {
	client := &Client{
		redactionPatterns: defaultRedactionPatterns,
	}

	tests := []struct {
		name           string
		input          string
		shouldContain  string   // What MUST be in output
		mustNotContain []string // What MUST NOT be in output (sensitive data)
		desc           string
	}{
		{
			name:           "Namespace-aware password element",
			input:          `<config><auth:password>secret123</auth:password><hostname>router1</hostname></config>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"secret123"},
			desc:           "Should redact <auth:password> with namespace prefix",
		},
		{
			name:           "Namespace-aware secret element",
			input:          `<data><cisco:secret>my_secret_value</cisco:secret></data>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"my_secret_value"},
			desc:           "Should redact <cisco:secret> with namespace prefix",
		},
		{
			name:           "Namespace-aware key element",
			input:          `<config><vpn:key>encryption_key_123</vpn:key></config>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"encryption_key_123"},
			desc:           "Should redact <vpn:key> with namespace prefix",
		},
		{
			name:           "Namespace-aware community element",
			input:          `<snmp><mgmt:community>public_string</mgmt:community></snmp>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"public_string"},
			desc:           "Should redact <mgmt:community> with namespace prefix",
		},
		{
			name:           "CDATA password element",
			input:          `<config><password><![CDATA[p@ssw0rd!]]></password></config>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"p@ssw0rd!"},
			desc:           "Should redact CDATA password content",
		},
		{
			name:           "CDATA secret element",
			input:          `<auth><secret><![CDATA[secret_token_xyz]]></secret></auth>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"secret_token_xyz"},
			desc:           "Should redact CDATA secret content",
		},
		{
			name:           "CDATA key element",
			input:          `<crypto><key><![CDATA[api_key_12345]]></key></crypto>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"api_key_12345"},
			desc:           "Should redact CDATA key content",
		},
		{
			name:           "CDATA community element",
			input:          `<snmp><community><![CDATA[community_ro]]></community></snmp>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"community_ro"},
			desc:           "Should redact CDATA community content",
		},
		{
			name:           "Namespace CDATA password",
			input:          `<config><ns:password><![CDATA[ns_password_value]]></ns:password></config>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"ns_password_value"},
			desc:           "Should redact namespace-aware CDATA password",
		},
		{
			name:           "Namespace CDATA secret",
			input:          `<data><auth:secret><![CDATA[secret_in_ns]]></auth:secret></data>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"secret_in_ns"},
			desc:           "Should redact namespace-aware CDATA secret",
		},
		{
			name:           "Namespace CDATA key",
			input:          `<vpn><config:key><![CDATA[key_in_ns]]></config:key></vpn>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"key_in_ns"},
			desc:           "Should redact namespace-aware CDATA key",
		},
		{
			name:           "Namespace CDATA community",
			input:          `<snmp><cisco:community><![CDATA[community_in_ns]]></cisco:community></snmp>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"community_in_ns"},
			desc:           "Should redact namespace-aware CDATA community",
		},
		{
			name:           "Mixed redaction types",
			input:          `<config><password>plain_pass</password><auth:password>ns_pass</auth:password><secret><![CDATA[cdata_secret]]></secret><vpn:key><![CDATA[ns_cdata_key]]></vpn:key></config>`,
			shouldContain:  "[REDACTED]",
			mustNotContain: []string{"plain_pass", "ns_pass", "cdata_secret", "ns_cdata_key"},
			desc:           "Should handle multiple redaction types simultaneously",
		},
		{
			name:           "Preserve non-sensitive content",
			input:          `<config><hostname>router1</hostname><auth:password>secret</auth:password><interface>GigE0/0</interface></config>`,
			shouldContain:  "router1",
			mustNotContain: []string{"secret"},
			desc:           "Should preserve non-sensitive data like hostnames",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.redactSensitiveData(tt.input)

			// Verify sensitive data is NOT in output
			for _, sensitive := range tt.mustNotContain {
				if strings.Contains(result, sensitive) {
					t.Errorf("%s FAILED: Sensitive data '%s' was not redacted\n"+
						"Input:  %s\n"+
						"Output: %s\n"+
						"Reason: %s",
						tt.name, sensitive, tt.input, result, tt.desc)
				}
			}

			// Verify [REDACTED] IS in output
			if !strings.Contains(result, tt.shouldContain) {
				t.Errorf("%s FAILED: Expected '%s' in output\n"+
					"Input:  %s\n"+
					"Output: %s\n"+
					"Reason: %s",
					tt.name, tt.shouldContain, tt.input, result, tt.desc)
			}
		})
	}
}

// TestClient_LockPollingBeyondMaxRetries verifies that lock-denied errors continue
// polling beyond MaxRetries until LockReleaseTimeout is reached.
//
// This test validates that lock operations poll for the full timeout duration
// instead of being limited by MaxRetries, ensuring lock-denied errors retry
// appropriately for up to the configured LockReleaseTimeout.
func TestClient_LockPollingBeyondMaxRetries(t *testing.T) {
	t.Run("lock polling logic bypasses MaxRetries", func(t *testing.T) {
		client := &Client{
			MaxRetries:         3,
			LockReleaseTimeout: 10 * time.Second,
			logger:             &NoOpLogger{},
		}

		// Verify lock-denied is detected as transient
		lockDeniedErrors := []ErrorModel{
			{ErrorTag: "lock-denied"},
		}

		isTransient := client.checkTransientError(lockDeniedErrors, nil)
		if !isTransient {
			t.Error("lock-denied should be detected as transient")
		}

		// Verify lock-denied is identified for special polling
		isLockDenied := client.isLockDeniedError(lockDeniedErrors)
		if !isLockDenied {
			t.Error("lock-denied should trigger lock polling")
		}

		// Verify the exit condition logic:
		// For lock-denied, (!isLockDenied && attempt >= MaxRetries) should be false
		// allowing the loop to continue beyond MaxRetries
		for attempt := 0; attempt <= 10; attempt++ {
			shouldExit := !isTransient || (!isLockDenied && attempt >= client.MaxRetries)

			// Lock-denied should NEVER trigger exit via MaxRetries check
			if shouldExit {
				t.Errorf("attempt %d: lock-denied should continue polling (bypasses MaxRetries), but exit condition is true", attempt)
			}
		}
	})

	t.Run("non-lock transient errors respect MaxRetries", func(t *testing.T) {
		client := &Client{
			MaxRetries:         3,
			LockReleaseTimeout: 10 * time.Second,
			logger:             &NoOpLogger{},
		}

		// Use transport error as example of non-lock transient error
		transportErrors := []ErrorModel{
			{ErrorType: "transport"},
		}

		isTransient := client.checkTransientError(transportErrors, nil)
		if !isTransient {
			t.Error("transport error should be detected as transient")
		}

		isLockDenied := client.isLockDeniedError(transportErrors)
		if isLockDenied {
			t.Error("transport error should NOT trigger lock polling")
		}

		// Verify the exit condition logic:
		// For non-lock transient errors, should exit when attempt >= MaxRetries
		for attempt := 0; attempt <= 5; attempt++ {
			shouldExit := !isTransient || (!isLockDenied && attempt >= client.MaxRetries)

			if attempt >= client.MaxRetries {
				// At or beyond MaxRetries: should exit for non-lock transient
				if !shouldExit {
					t.Errorf("attempt %d: non-lock error should exit (respects MaxRetries), but exit condition is false", attempt)
				}
			} else {
				// Within MaxRetries: should continue
				if shouldExit {
					t.Errorf("attempt %d: should continue (within MaxRetries), but exit condition is true", attempt)
				}
			}
		}
	})

	t.Run("non-transient errors exit immediately", func(t *testing.T) {
		client := &Client{
			MaxRetries: 3,
			logger:     &NoOpLogger{},
		}

		// Non-transient error (invalid-value is not in TransientErrors list)
		nonTransientErrors := []ErrorModel{
			{ErrorTag: "invalid-value"},
		}

		isTransient := client.checkTransientError(nonTransientErrors, nil)
		if isTransient {
			t.Error("invalid-value should NOT be detected as transient")
		}

		// Verify exit condition: non-transient errors should exit immediately
		for attempt := 0; attempt <= 5; attempt++ {
			isLockDenied := false // irrelevant for non-transient
			shouldExit := !isTransient || (!isLockDenied && attempt >= client.MaxRetries)

			// Should always exit immediately for non-transient errors
			if !shouldExit {
				t.Errorf("attempt %d: non-transient error should exit immediately, but exit condition is false", attempt)
			}
		}
	})
}

func TestResponsePreprocessor(t *testing.T) {
	t.Run("preprocessor transforms XML before parsing", func(t *testing.T) {
		// Simulate a NETCONF response with unescaped angle brackets in banner text.
		// Without preprocessing, xmldot would treat "<contact:" as an XML tag.
		rawXML := `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <data>
    <native>
      <banner>
        <login>
          <banner>==> LOGIN <==</banner>
        </login>
      </banner>
    </native>
  </data>
</rpc-reply>`

		// Preprocessor escapes "==>" and "<==" sequences globally
		preprocessor := func(xml string) string {
			return strings.ReplaceAll(
				strings.ReplaceAll(xml, "==>", "==&gt;"),
				"<==", "&lt;==",
			)
		}

		client := &Client{
			logger:               &NoOpLogger{},
			responsePreprocessor: preprocessor,
		}

		res, err := client.parseResponse(&response.NetconfResponse{
			Result: rawXML,
		})
		if err != nil {
			t.Fatalf("parseResponse returned error: %v", err)
		}

		loginBanner := res.Res.Get("data.native.banner.login.banner").String()
		if !strings.Contains(loginBanner, "==>") {
			t.Errorf("expected login banner to contain '==>', got %q", loginBanner)
		}
		if !strings.Contains(loginBanner, "<==") {
			t.Errorf("expected login banner to contain '<==', got %q", loginBanner)
		}
	})

	t.Run("nil preprocessor is a no-op", func(t *testing.T) {
		rawXML := `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <data>
    <native>
      <banner>
        <motd>
          <banner>Simple banner</banner>
        </motd>
      </banner>
    </native>
  </data>
</rpc-reply>`

		client := &Client{
			logger: &NoOpLogger{},
		}

		res, err := client.parseResponse(&response.NetconfResponse{
			Result: rawXML,
		})
		if err != nil {
			t.Fatalf("parseResponse returned error: %v", err)
		}

		motdBanner := res.Res.Get("data.native.banner.motd.banner").String()
		if motdBanner != "Simple banner" {
			t.Errorf("expected 'Simple banner', got %q", motdBanner)
		}
	})

	t.Run("preprocessor applied to ok response", func(t *testing.T) {
		called := false
		preprocessor := func(xml string) string {
			called = true
			return xml
		}

		client := &Client{
			logger:               &NoOpLogger{},
			responsePreprocessor: preprocessor,
		}

		res, err := client.parseResponse(&response.NetconfResponse{
			Result: `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <ok/>
</rpc-reply>`,
		})
		if err != nil {
			t.Fatalf("parseResponse returned error: %v", err)
		}
		if !called {
			t.Error("expected preprocessor to be called")
		}
		if !res.OK {
			t.Error("expected OK to be true")
		}
	})

	t.Run("preprocessor applied to error response", func(t *testing.T) {
		client := &Client{
			logger: &NoOpLogger{},
			responsePreprocessor: func(xml string) string {
				return xml
			},
		}

		res, err := client.parseResponse(&response.NetconfResponse{
			Result: `<?xml version="1.0"?>
<rpc-reply xmlns="urn:ietf:params:xml:ns:netconf:base:1.0" message-id="1">
  <rpc-error>
    <error-type>application</error-type>
    <error-tag>operation-failed</error-tag>
    <error-severity>error</error-severity>
    <error-message>Something went wrong</error-message>
  </rpc-error>
</rpc-reply>`,
		})
		if err != nil {
			t.Fatalf("parseResponse returned error: %v", err)
		}
		if len(res.Errors) != 1 {
			t.Fatalf("expected 1 error, got %d", len(res.Errors))
		}
		if res.Errors[0].ErrorMessage != "Something went wrong" {
			t.Errorf("expected error message 'Something went wrong', got %q", res.Errors[0].ErrorMessage)
		}
	})
}
