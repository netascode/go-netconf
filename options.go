// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import "time"

// Client configuration options using the functional options pattern

// Username sets the username for NETCONF authentication
func Username(username string) func(*Client) {
	return func(c *Client) {
		c.username = username
	}
}

// Password sets the password for NETCONF authentication
func Password(password string) func(*Client) {
	return func(c *Client) {
		c.password = password
	}
}

// SSHKey sets the SSH private key path for authentication
//
// The key will be loaded when NewClient is called. If the key file
// cannot be read, an error will be returned from NewClient.
func SSHKey(keyPath string) func(*Client) {
	return func(c *Client) {
		c.SSHKeyPath = keyPath
		// The actual key loading is handled by scrapligo in NewClient
	}
}

// Port sets the NETCONF port (default: 830)
func Port(port int) func(*Client) {
	return func(c *Client) {
		c.Port = port
	}
}

// ConnectTimeout sets the connection timeout (default: 30s)
func ConnectTimeout(duration time.Duration) func(*Client) {
	return func(c *Client) {
		c.ConnectTimeout = duration
	}
}

// AttemptTimeout sets the timeout for a single operation attempt (default: 30s)
// This timeout is enforced by scrapligo for each individual RPC call.
func AttemptTimeout(duration time.Duration) func(*Client) {
	return func(c *Client) {
		c.AttemptTimeout = duration
	}
}

// TotalTimeout sets the total timeout across all retry attempts (default: 2min)
// This timeout spans all retry attempts including backoff delays.
func TotalTimeout(duration time.Duration) func(*Client) {
	return func(c *Client) {
		c.TotalTimeout = duration
	}
}

// MaxRetries sets the maximum number of retry attempts for transient errors (default: 10)
func MaxRetries(retries int) func(*Client) {
	return func(c *Client) {
		c.MaxRetries = retries
	}
}

// BackoffMinDelay sets the minimum backoff delay (default: 1s)
func BackoffMinDelay(duration time.Duration) func(*Client) {
	return func(c *Client) {
		c.BackoffMinDelay = duration
	}
}

// BackoffMaxDelay sets the maximum backoff delay (default: 60s)
func BackoffMaxDelay(duration time.Duration) func(*Client) {
	return func(c *Client) {
		c.BackoffMaxDelay = duration
	}
}

// BackoffDelayFactor sets the backoff multiplication factor (default: 1.2)
func BackoffDelayFactor(factor float64) func(*Client) {
	return func(c *Client) {
		c.BackoffDelayFactor = factor
	}
}

// LockReleaseTimeout sets the timeout for waiting for lock release (default: 120s)
func LockReleaseTimeout(duration time.Duration) func(*Client) {
	return func(c *Client) {
		c.LockReleaseTimeout = duration
	}
}

// InsecureSkipHostKeyVerification disables SSH host key verification
//
// WARNING: This makes the connection vulnerable to Man-in-the-Middle attacks.
// Only use this in testing environments where security is not a concern.
//
// By default, host key verification is enabled for security.
// This option explicitly disables it.
//
// Example:
//
//	client, _ := netconf.NewClient("192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.InsecureSkipHostKeyVerification())
func InsecureSkipHostKeyVerification() func(*Client) {
	return func(c *Client) {
		c.InsecureSkipVerify = true
	}
}

// WithLogger configures a custom logger for the client
//
// By default, the client uses NoOpLogger which discards all log messages.
// Use this option to enable logging with DefaultLogger or a custom logger.
//
// All XML content logged at Debug level is automatically redacted to remove
// sensitive data (passwords, secrets, keys, community strings).
//
// Example (DefaultLogger):
//
//	logger := netconf.NewDefaultLogger(netconf.LogLevelInfo)
//	client, _ := netconf.NewClient("192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.WithLogger(logger))
//
// Example (Custom Logger):
//
//	type SlogAdapter struct {
//	    logger *slog.Logger
//	}
//
//	func (s *SlogAdapter) Debug(ctx context.Context, msg string, keysAndValues ...interface{}) {
//	    s.logger.DebugContext(ctx, msg, keysAndValues...)
//	}
//	// ... implement Info, Warn, Error (all with ctx context.Context as first parameter)
//
//	client, _ := netconf.NewClient("192.168.1.1",
//	    netconf.WithLogger(&SlogAdapter{logger: slog.Default()}))
func WithLogger(logger Logger) func(*Client) {
	return func(c *Client) {
		if logger != nil {
			c.logger = logger
		}
	}
}

// WithPrettyPrintLogs enables/disables XML pretty printing in logs
//
// When enabled (default), XML content in debug logs is formatted using
// xmldot's @pretty modifier for better readability. When disabled, raw
// XML is logged without formatting.
//
// This only affects Debug-level log output. Disabling pretty printing
// can improve performance when high-frequency operations are logged.
//
// Default: enabled (true)
//
// Example:
//
//	logger := netconf.NewDefaultLogger(netconf.LogLevelDebug)
//	client, _ := netconf.NewClient("192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.WithLogger(logger),
//	    netconf.WithPrettyPrintLogs(false))  // Disable formatting for performance
func WithPrettyPrintLogs(enabled bool) func(*Client) {
	return func(c *Client) {
		c.prettyPrintLogs = enabled
	}
}

// Request modifiers for individual operations

// Timeout returns a request modifier that sets a custom timeout for the operation
func Timeout(duration time.Duration) func(*Req) {
	return func(req *Req) {
		req.Timeout = duration
	}
}

// DefaultOperation returns a request modifier for edit-config default operation
// Valid values: "merge", "replace", "none"
func DefaultOperation(op string) func(*Req) {
	return func(req *Req) {
		req.DefaultOperation = op
	}
}

// TestOption returns a request modifier for edit-config test option
// Valid values: "test-then-set", "set", "test-only"
func TestOption(opt string) func(*Req) {
	return func(req *Req) {
		req.TestOption = opt
	}
}

// ErrorOption returns a request modifier for edit-config error option
// Valid values: "stop-on-error", "continue-on-error", "rollback-on-error"
func ErrorOption(opt string) func(*Req) {
	return func(req *Req) {
		req.ErrorOption = opt
	}
}

// Confirmed returns a request modifier for confirmed commit with timeout
//
// Per RFC 6241 Section 8.3.4.1, confirmed commits require explicit confirmation
// within the timeout period or will be automatically rolled back.
func Confirmed(timeoutSeconds int) func(*Req) {
	return func(req *Req) {
		req.ConfirmTimeout = timeoutSeconds
	}
}

// Persist returns a request modifier for persisting a commit ID
//
// Used with confirmed commits to identify the commit operation for
// later confirmation or cancellation.
func Persist(persistID string) func(*Req) {
	return func(req *Req) {
		req.PersistID = persistID
	}
}
