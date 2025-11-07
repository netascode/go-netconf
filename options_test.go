// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"testing"
	"time"
)

// TestClientOptions tests all client configuration options
func TestClientOptions(t *testing.T) {
	tests := []struct {
		name     string
		option   func(*Client)
		validate func(*testing.T, *Client)
	}{
		{
			name:   "Username option",
			option: Username("testuser"),
			validate: func(t *testing.T, c *Client) {
				if c.username != "testuser" {
					t.Errorf("expected username 'testuser', got %s", c.username)
				}
			},
		},
		{
			name:   "Password option",
			option: Password("testpass"),
			validate: func(t *testing.T, c *Client) {
				if c.password != "testpass" {
					t.Errorf("expected password 'testpass', got %s", c.password)
				}
			},
		},
		{
			name:   "SSHKey option",
			option: SSHKey("/path/to/key"),
			validate: func(t *testing.T, c *Client) {
				if c.SSHKeyPath != "/path/to/key" {
					t.Errorf("expected SSHKeyPath '/path/to/key', got %s", c.SSHKeyPath)
				}
			},
		},
		{
			name:   "Port option",
			option: Port(8830),
			validate: func(t *testing.T, c *Client) {
				if c.Port != 8830 {
					t.Errorf("expected Port 8830, got %d", c.Port)
				}
			},
		},
		{
			name:   "ConnectTimeout option",
			option: ConnectTimeout(15 * time.Second),
			validate: func(t *testing.T, c *Client) {
				if c.ConnectTimeout != 15*time.Second {
					t.Errorf("expected ConnectTimeout 15s, got %v", c.ConnectTimeout)
				}
			},
		},
		{
			name:   "AttemptTimeout option",
			option: AttemptTimeout(45 * time.Second),
			validate: func(t *testing.T, c *Client) {
				if c.AttemptTimeout != 45*time.Second {
					t.Errorf("expected AttemptTimeout 45s, got %v", c.AttemptTimeout)
				}
			},
		},
		{
			name:   "TotalTimeout option",
			option: TotalTimeout(3 * time.Minute),
			validate: func(t *testing.T, c *Client) {
				if c.TotalTimeout != 3*time.Minute {
					t.Errorf("expected TotalTimeout 3min, got %v", c.TotalTimeout)
				}
			},
		},
		{
			name:   "MaxRetries option",
			option: MaxRetries(5),
			validate: func(t *testing.T, c *Client) {
				if c.MaxRetries != 5 {
					t.Errorf("expected MaxRetries 5, got %d", c.MaxRetries)
				}
			},
		},
		{
			name:   "BackoffMinDelay option",
			option: BackoffMinDelay(2 * time.Second),
			validate: func(t *testing.T, c *Client) {
				if c.BackoffMinDelay != 2*time.Second {
					t.Errorf("expected BackoffMinDelay 2s, got %v", c.BackoffMinDelay)
				}
			},
		},
		{
			name:   "BackoffMaxDelay option",
			option: BackoffMaxDelay(120 * time.Second),
			validate: func(t *testing.T, c *Client) {
				if c.BackoffMaxDelay != 120*time.Second {
					t.Errorf("expected BackoffMaxDelay 120s, got %v", c.BackoffMaxDelay)
				}
			},
		},
		{
			name:   "BackoffDelayFactor option",
			option: BackoffDelayFactor(2.0),
			validate: func(t *testing.T, c *Client) {
				if c.BackoffDelayFactor != 2.0 {
					t.Errorf("expected BackoffDelayFactor 2.0, got %f", c.BackoffDelayFactor)
				}
			},
		},
		{
			name:   "LockReleaseTimeout option",
			option: LockReleaseTimeout(180 * time.Second),
			validate: func(t *testing.T, c *Client) {
				if c.LockReleaseTimeout != 180*time.Second {
					t.Errorf("expected LockReleaseTimeout 180s, got %v", c.LockReleaseTimeout)
				}
			},
		},
		{
			name:   "InsecureSkipHostKeyVerification option",
			option: InsecureSkipHostKeyVerification(),
			validate: func(t *testing.T, c *Client) {
				if !c.InsecureSkipVerify {
					t.Error("expected InsecureSkipVerify to be true")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			tt.option(client)
			tt.validate(t, client)
		})
	}
}

// TestRequestModifiers tests all request modifier options
func TestRequestModifiers(t *testing.T) {
	tests := []struct {
		name     string
		modifier func(*Req)
		validate func(*testing.T, *Req)
	}{
		{
			name:     "Timeout modifier",
			modifier: Timeout(30 * time.Second),
			validate: func(t *testing.T, req *Req) {
				if req.Timeout != 30*time.Second {
					t.Errorf("expected Timeout 30s, got %v", req.Timeout)
				}
			},
		},
		{
			name:     "DefaultOperation modifier",
			modifier: DefaultOperation("replace"),
			validate: func(t *testing.T, req *Req) {
				if req.DefaultOperation != "replace" {
					t.Errorf("expected DefaultOp 'replace', got %s", req.DefaultOperation)
				}
			},
		},
		{
			name:     "TestOption modifier",
			modifier: TestOption("test-then-set"),
			validate: func(t *testing.T, req *Req) {
				if req.TestOption != "test-then-set" {
					t.Errorf("expected TestOption 'test-then-set', got %s", req.TestOption)
				}
			},
		},
		{
			name:     "ErrorOption modifier",
			modifier: ErrorOption("rollback-on-error"),
			validate: func(t *testing.T, req *Req) {
				if req.ErrorOption != "rollback-on-error" {
					t.Errorf("expected ErrorOpt 'rollback-on-error', got %s", req.ErrorOption)
				}
			},
		},
		{
			name:     "Confirmed modifier",
			modifier: Confirmed(60),
			validate: func(t *testing.T, req *Req) {
				if req.ConfirmTimeout != 60 {
					t.Errorf("expected ConfirmTimeout 60, got %d", req.ConfirmTimeout)
				}
			},
		},
		{
			name:     "Persist modifier",
			modifier: Persist("persist-123"),
			validate: func(t *testing.T, req *Req) {
				if req.PersistID != "persist-123" {
					t.Errorf("expected PersistID 'persist-123', got %s", req.PersistID)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &Req{}
			tt.modifier(req)
			tt.validate(t, req)
		})
	}
}

// TestMultipleOptions tests that multiple options can be applied together
func TestMultipleOptions(t *testing.T) {
	client := &Client{
		Host: "192.168.1.1",
		Port: 830,
	}

	options := []func(*Client){
		Username("admin"),
		Password("secret"),
		Port(8830),
		MaxRetries(3),
		ConnectTimeout(20 * time.Second),
		AttemptTimeout(45 * time.Second),
		TotalTimeout(3 * time.Minute),
		BackoffMinDelay(2 * time.Second),
		BackoffMaxDelay(90 * time.Second),
		BackoffDelayFactor(1.5),
		LockReleaseTimeout(150 * time.Second),
	}

	for _, opt := range options {
		opt(client)
	}

	// Validate all options were applied
	if client.username != "admin" {
		t.Errorf("expected username 'admin', got %s", client.username)
	}
	if client.password != "secret" {
		t.Errorf("expected password 'secret', got %s", client.password)
	}
	if client.Port != 8830 {
		t.Errorf("expected Port 8830, got %d", client.Port)
	}
	if client.MaxRetries != 3 {
		t.Errorf("expected MaxRetries 3, got %d", client.MaxRetries)
	}
	if client.ConnectTimeout != 20*time.Second {
		t.Errorf("expected ConnectTimeout 20s, got %v", client.ConnectTimeout)
	}
	if client.AttemptTimeout != 45*time.Second {
		t.Errorf("expected AttemptTimeout 45s, got %v", client.AttemptTimeout)
	}
	if client.TotalTimeout != 3*time.Minute {
		t.Errorf("expected TotalTimeout 3min, got %v", client.TotalTimeout)
	}
	if client.BackoffMinDelay != 2*time.Second {
		t.Errorf("expected BackoffMinDelay 2s, got %v", client.BackoffMinDelay)
	}
	if client.BackoffMaxDelay != 90*time.Second {
		t.Errorf("expected BackoffMaxDelay 90s, got %v", client.BackoffMaxDelay)
	}
	if client.BackoffDelayFactor != 1.5 {
		t.Errorf("expected BackoffDelayFactor 1.5, got %f", client.BackoffDelayFactor)
	}
	if client.LockReleaseTimeout != 150*time.Second {
		t.Errorf("expected LockReleaseTimeout 150s, got %v", client.LockReleaseTimeout)
	}
}

// TestMultipleRequestModifiers tests combining multiple request modifiers
func TestMultipleRequestModifiers(t *testing.T) {
	req := &Req{
		Operation: "edit-config",
	}

	modifiers := []func(*Req){
		Timeout(45 * time.Second),
		DefaultOperation("merge"),
		TestOption("test-only"),
		ErrorOption("continue-on-error"),
		Confirmed(120),
		Persist("test-persist"),
	}

	for _, mod := range modifiers {
		mod(req)
	}

	// Validate all modifiers were applied
	if req.Timeout != 45*time.Second {
		t.Errorf("expected Timeout 45s, got %v", req.Timeout)
	}
	if req.DefaultOperation != "merge" {
		t.Errorf("expected DefaultOp 'merge', got %s", req.DefaultOperation)
	}
	if req.TestOption != "test-only" {
		t.Errorf("expected TestOption 'test-only', got %s", req.TestOption)
	}
	if req.ErrorOption != "continue-on-error" {
		t.Errorf("expected ErrorOpt 'continue-on-error', got %s", req.ErrorOption)
	}
	if req.ConfirmTimeout != 120 {
		t.Errorf("expected ConfirmTimeout 120, got %d", req.ConfirmTimeout)
	}
	if req.PersistID != "test-persist" {
		t.Errorf("expected PersistID 'test-persist', got %s", req.PersistID)
	}
}

// TestZeroValueOptions tests options with zero/empty values
func TestZeroValueOptions(t *testing.T) {
	t.Run("empty username", func(t *testing.T) {
		client := &Client{}
		Username("")(client)
		if client.username != "" {
			t.Error("expected empty username")
		}
	})

	t.Run("empty password", func(t *testing.T) {
		client := &Client{}
		Password("")(client)
		if client.password != "" {
			t.Error("expected empty password")
		}
	})

	t.Run("zero port", func(t *testing.T) {
		client := &Client{}
		Port(0)(client)
		if client.Port != 0 {
			t.Errorf("expected Port 0, got %d", client.Port)
		}
	})

	t.Run("zero timeout", func(t *testing.T) {
		client := &Client{}
		ConnectTimeout(0)(client)
		if client.ConnectTimeout != 0 {
			t.Error("expected zero ConnectTimeout")
		}
	})

	t.Run("zero max retries", func(t *testing.T) {
		client := &Client{}
		MaxRetries(0)(client)
		if client.MaxRetries != 0 {
			t.Errorf("expected MaxRetries 0, got %d", client.MaxRetries)
		}
	})
}

// TestOptionOverwriting tests that options can be overwritten
func TestOptionOverwriting(t *testing.T) {
	client := &Client{}

	// Apply first set of options
	Port(830)(client)
	MaxRetries(10)(client)

	if client.Port != 830 {
		t.Errorf("expected initial Port 830, got %d", client.Port)
	}
	if client.MaxRetries != 10 {
		t.Errorf("expected initial MaxRetries 10, got %d", client.MaxRetries)
	}

	// Overwrite with new values
	Port(8830)(client)
	MaxRetries(5)(client)

	if client.Port != 8830 {
		t.Errorf("expected overwritten Port 8830, got %d", client.Port)
	}
	if client.MaxRetries != 5 {
		t.Errorf("expected overwritten MaxRetries 5, got %d", client.MaxRetries)
	}
}
