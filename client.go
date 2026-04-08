// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/netascode/xmldot"
	"github.com/scrapli/scrapligo/driver/netconf"
	"github.com/scrapli/scrapligo/driver/opoptions"
	"github.com/scrapli/scrapligo/driver/options"
	"github.com/scrapli/scrapligo/response"
	"github.com/scrapli/scrapligo/transport"
	"github.com/scrapli/scrapligo/util"
)

// NETCONF capability URNs and filter types
const (
	netconfBase10URN = "urn:ietf:params:netconf:base:1.0"
	transportErrType = "transport"
	filterTypeXPath  = "xpath"
)

// NETCONF operation names
const (
	opGet          = "get"
	opGetConfig    = "get-config"
	opEditConfig   = "edit-config"
	opCopyConfig   = "copy-config"
	opDeleteConfig = "delete-config"
	opLock         = "lock"
	opUnlock       = "unlock"
	opCommit       = "commit"
	opDiscard      = "discard"
	opValidate     = "validate"
	opRPC          = "rpc"
)

// Default client configuration values
const (
	DefaultPort               = 830
	DefaultMaxRetries         = 3
	DefaultBackoffMinDelay    = 1 * time.Second
	DefaultBackoffMaxDelay    = 60 * time.Second
	DefaultBackoffDelayFactor = 2
	DefaultLockReleaseTimeout = 120 * time.Second
	DefaultConnectTimeout     = 10 * time.Second
	DefaultAttemptTimeout     = 30 * time.Second
	DefaultTotalTimeout       = 2 * time.Minute
	DefaultPrettyPrintLogs    = true
)

// Security limits for XML processing and logging
const (
	MaxXMLSizeForLogging = 1 * 1024 * 1024 // 1MB limit to prevent ReDoS attacks
	MaxSensitiveElements = 1000            // Max redaction operations to prevent DoS
)

// Logging message constants
const (
	XMLTooLargeMessage         = "[XML TOO LARGE FOR LOGGING]"
	XMLTooManySensitiveMessage = "[XML CONTAINS TOO MANY SENSITIVE ELEMENTS]"
)

// defaultRedactionPatterns contains regex patterns for redacting sensitive data in logs
var defaultRedactionPatterns = []*regexp.Regexp{
	// Element content - Handle nested structures (Cisco YANG models use container/value nesting)
	// Match greedy to capture nested structures: <password>...<password>...</password></password>
	// The [\s\S] matches any character including newlines
	regexp.MustCompile(`<password>[\s\S]*?</password>`),
	regexp.MustCompile(`<secret>[\s\S]*?</secret>`),
	regexp.MustCompile(`<key>[\s\S]*?</key>`),
	regexp.MustCompile(`<community>[\s\S]*?</community>`),

	// CDATA section handling (must come before namespace-aware to avoid conflicts)
	// Matches: <password><![CDATA[value]]></password>
	regexp.MustCompile(`<password><!\[CDATA\[.*?\]\]></password>`),
	regexp.MustCompile(`<secret><!\[CDATA\[.*?\]\]></secret>`),
	regexp.MustCompile(`<key><!\[CDATA\[.*?\]\]></key>`),
	regexp.MustCompile(`<community><!\[CDATA\[.*?\]\]></community>`),

	// Namespace-aware element content
	// Matches: <prefix:password>value</prefix:password>
	// Note: Go regexp doesn't support backreferences, so we match any namespace in closing tag
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:password[^>]*>.*?</[a-zA-Z0-9_-]+:password>`),
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:secret[^>]*>.*?</[a-zA-Z0-9_-]+:secret>`),
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:key[^>]*>.*?</[a-zA-Z0-9_-]+:key>`),
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:community[^>]*>.*?</[a-zA-Z0-9_-]+:community>`),

	// Namespaced CDATA sections
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:password[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:password>`),
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:secret[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:secret>`),
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:key[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:key>`),
	regexp.MustCompile(`<[a-zA-Z0-9_-]+:community[^>]*><!\[CDATA\[.*?\]\]></[a-zA-Z0-9_-]+:community>`),

	// Attribute values (double quotes)
	regexp.MustCompile(`password="[^"]*"`),
	regexp.MustCompile(`secret="[^"]*"`),
	regexp.MustCompile(`key="[^"]*"`),
	regexp.MustCompile(`community="[^"]*"`),

	// Attribute values (single quotes)
	regexp.MustCompile(`password='[^']*'`),
	regexp.MustCompile(`secret='[^']*'`),
	regexp.MustCompile(`key='[^']*'`),
	regexp.MustCompile(`community='[^']*'`),

	// XPath predicates (within square brackets)
	regexp.MustCompile(`\[password="[^"]*"\]`),
	regexp.MustCompile(`\[password='[^']*'\]`),
	regexp.MustCompile(`\[secret="[^"]*"\]`),
	regexp.MustCompile(`\[secret='[^']*'\]`),
	regexp.MustCompile(`\[key="[^"]*"\]`),
	regexp.MustCompile(`\[key='[^']*'\]`),
}

// Client represents a NETCONF client connection to a network device
type Client struct {
	// scrapligo driver for NETCONF transport
	driver *netconf.Driver

	// RWMutex to synchronize access to mutable state
	mu sync.RWMutex

	// Connection parameters
	Host     string
	Port     int
	username string
	password string

	// SSH key authentication
	SSHKeyPath string

	// Security options
	InsecureSkipVerify bool // Disables SSH host key verification (insecure, use only for testing)

	// Configuration options
	MaxRetries         int
	BackoffMinDelay    time.Duration
	BackoffMaxDelay    time.Duration
	BackoffDelayFactor float64
	LockReleaseTimeout time.Duration
	ConnectTimeout     time.Duration
	AttemptTimeout     time.Duration // Timeout for a single operation attempt (scrapligo timeout)
	TotalTimeout       time.Duration // Total timeout across all retry attempts

	// Capability tracking
	Capabilities []string

	// Session information
	sessionID     string
	serverVersion string

	// Logging configuration
	logger            Logger
	prettyPrintLogs   bool
	redactionPatterns []*regexp.Regexp

	// ResponsePreprocessor is an optional function that transforms the raw XML
	// response string before it is parsed by xmldot. This allows callers to
	// sanitize malformed XML (e.g., unescaped angle brackets in banner text)
	// before the parser sees it.
	ResponsePreprocessor func(string) string
}

// NewClient creates a new NETCONF client with the specified host and options
//
// The client does NOT connect immediately. Connection is established either:
//   1. Automatically on first operation (lazy connect), or
//   2. Explicitly via client.Open()
//
// Use functional options to configure authentication and behavior.
//
// Example (lazy connect):
//
//	client, err := netconf.NewClient(
//	    "192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.Port(830),
//	    netconf.MaxRetries(5),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Connection happens automatically on first operation
//	res, err := client.GetConfig(ctx, "running", filter)
//
// Example (explicit connect):
//
//	client, err := netconf.NewClient("192.168.1.1", ...)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Explicit connection
//	if err := client.Open(); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
// Returns a configured Client. Connection errors are returned from Open()
// or the first operation, not from NewClient().

// buildScrapligoOptions creates scrapligo driver options from client configuration.
//
// This helper method ensures consistent driver configuration between initial
// connection (NewClient) and reconnection (reconnect). It builds the options
// list based on the client's current configuration settings.
//
// Returns a slice of scrapligo options ready to pass to netconf.NewDriver().
func (c *Client) buildScrapligoOptions() []util.Option {
	scrapliOpts := []util.Option{
		options.WithAuthUsername(c.username),
		options.WithAuthPassword(c.password),
		options.WithPort(c.Port),
		options.WithTimeoutSocket(c.ConnectTimeout),
		options.WithTimeoutOps(c.AttemptTimeout),
		options.WithTransportType(transport.StandardTransport),
	}

	// Only disable host key verification if explicitly requested
	if c.InsecureSkipVerify {
		scrapliOpts = append(scrapliOpts, options.WithAuthNoStrictKey())
	}

	// Add SSH key authentication if provided
	if c.SSHKeyPath != "" {
		// scrapligo expects key path and passphrase
		scrapliOpts = append(scrapliOpts, options.WithAuthPrivateKey(c.SSHKeyPath, ""))
	}

	return scrapliOpts
}

func NewClient(host string, opts ...func(*Client)) (*Client, error) {
	// Create client with default values
	client := &Client{
		Host:               host,
		Port:               DefaultPort,
		MaxRetries:         DefaultMaxRetries,
		BackoffMinDelay:    DefaultBackoffMinDelay,
		BackoffMaxDelay:    DefaultBackoffMaxDelay,
		BackoffDelayFactor: DefaultBackoffDelayFactor,
		LockReleaseTimeout: DefaultLockReleaseTimeout,
		ConnectTimeout:     DefaultConnectTimeout,
		AttemptTimeout:     DefaultAttemptTimeout,
		TotalTimeout:       DefaultTotalTimeout,
		logger:             &NoOpLogger{},
		prettyPrintLogs:    DefaultPrettyPrintLogs,
		redactionPatterns:  defaultRedactionPatterns,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(client)
	}

	// Lazy connect: return without establishing connection
	// Connection will be established on first operation or explicit Open() call
	return client, nil
}

// connect establishes the NETCONF connection and performs capability exchange.
// This method is called either explicitly via Open() or lazily via ensureConnected().
//
// PRECONDITION: Caller must hold c.mu.Lock() (write lock)
//
// Returns an error if connection fails.
func (c *Client) connect() error {
	// Build scrapligo options using helper method
	scrapliOpts := c.buildScrapligoOptions()

	// Create NETCONF driver
	driver, err := netconf.NewDriver(c.Host, scrapliOpts...)
	if err != nil {
		return fmt.Errorf("failed to create NETCONF driver: %w", err)
	}

	// Open connection and perform capability exchange
	err = driver.Open()
	if err != nil {
		return fmt.Errorf("failed to open NETCONF connection: %w", err)
	}

	// Store driver and capabilities
	c.driver = driver
	c.Capabilities = driver.ServerCapabilities()

	// Extract session information
	c.sessionID = fmt.Sprintf("%d", driver.SessionID())

	// Extract server version from capabilities if available
	for _, cap := range c.Capabilities {
		// Look for base capability to determine version
		if cap == netconfBase10URN ||
			cap == "urn:ietf:params:netconf:base:1.1" {
			c.serverVersion = driver.SelectedVersion
			break
		}
	}

	// Log successful connection
	c.logger.Info(context.Background(), "NETCONF connection established",
		"host", c.Host,
		"port", c.Port,
		"sessionID", c.sessionID,
		"version", c.serverVersion)

	c.logger.Debug(context.Background(), "NETCONF capabilities discovered",
		"count", len(c.Capabilities))

	return nil
}

// Open explicitly establishes the NETCONF connection
//
// This method connects to the server and performs capability exchange.
// It can be called to pre-connect before operations, or connection will
// happen automatically on first operation (lazy connect). Multiple calls
// to Open() are safe - subsequent calls are no-ops if already connected.
//
// Example:
//
//	client, err := netconf.NewClient("192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Explicit connect
//	if err := client.Open(); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
// Returns an error if connection fails.
func (c *Client) Open() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Already connected, no-op
	if c.driver != nil {
		return nil
	}

	return c.connect()
}

// ensureConnected ensures the client has an active connection.
// If no connection exists, establishes connection automatically (lazy connect).
//
// This method uses a double-check locking pattern for thread safety:
// 1. Fast path: Check with read lock (already connected)
// 2. Slow path: Acquire write lock and connect
//
// PRECONDITION: Caller must NOT hold any locks
//
// Thread Safety: This method acquires and releases locks internally. Callers
// must not hold locks when calling this method to avoid lock ordering violations.
//
// Returns an error if connection establishment fails.
func (c *Client) ensureConnected() error {
	// Fast path: check with read lock
	c.mu.RLock()
	if c.driver != nil {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	// Slow path: establish connection with write lock
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check: another goroutine may have connected between unlock and lock
	if c.driver != nil {
		return nil
	}

	// Establish connection
	return c.connect()
}

// requireDriver checks if the driver is initialized and returns an error if not.
// This helper centralizes nil driver checks and provides consistent error messages.
//
// PRECONDITION: Caller must hold c.mu (read or write lock)
func (c *Client) requireDriver(operation string) error {
	if c.driver == nil {
		return fmt.Errorf("operation %s failed: driver is nil (connection closed)", operation)
	}
	return nil
}

// Close closes the NETCONF session and cleans up resources
//
// This sends a close-session RPC to the server and closes the underlying
// transport connection. The driver reference is cleared before closing to
// prevent double-close attempts if Close() is called multiple times.
//
// Timeout Protection: Uses ConnectTimeout to prevent indefinite blocking
// due to scrapligo v1.3.3 bug where Close() can block forever.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.driver == nil {
		// Already closed or never connected
		return nil
	}

	// Clear driver reference before closing to prevent double-close
	driver := c.driver
	c.driver = nil

	// Timeout protection for driver.Close() (scrapligo v1.3.3 bug workaround)
	// The bug causes Close() to block indefinitely when the read goroutine is stuck
	ctx, cancel := context.WithTimeout(context.Background(), c.ConnectTimeout)
	defer cancel()

	closeDone := make(chan error, 1) // Buffered to prevent goroutine leak
	go func() {
		closeDone <- driver.Close()
	}()

	select {
	case err := <-closeDone:
		if err != nil {
			return fmt.Errorf("failed to close NETCONF session: %w", err)
		}
	case <-ctx.Done():
		c.logger.Warn(context.Background(), "NETCONF driver close timeout, connection may leak",
			"host", c.Host,
			"timeout", c.ConnectTimeout)
		return fmt.Errorf("failed to close NETCONF session: timeout after %v", c.ConnectTimeout)
	}

	c.logger.Info(context.Background(), "NETCONF connection closed",
		"host", c.Host,
		"sessionID", c.sessionID)

	return nil
}

// IsClosed returns true if the connection is closed
//
// This method checks the internal driver state to determine if the connection
// is active. Note that with lazy connect, a newly created client will return
// false (not closed, just not connected yet). After calling Close(), this
// returns true.
//
// Example:
//
//	if client.IsClosed() {
//	    if err := client.Open(); err != nil {
//	        log.Fatal(err)
//	    }
//	}
//	res, err := client.GetConfig(ctx, "running", filter)
//
// Returns true if the connection was explicitly closed, false otherwise.
func (c *Client) IsClosed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.driver == nil
}

// Get retrieves configuration and state data from the device
//
// The filter parameter can be a subtree filter, XPath filter, or NoFilter()
// to retrieve all data.
//
// Example:
//
//	ctx := context.Background()
//	filter := netconf.SubtreeFilter("<interfaces/>")
//	res, err := client.Get(ctx, filter)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	ifName := res.Res.Get("data.interfaces.interface.name").String()
func (c *Client) Get(ctx context.Context, filter Filter, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Acquire exclusive lock to serialize all operations
	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("get"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "get",
		Filter:    filter,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// GetConfig retrieves configuration data from the specified datastore
//
// Valid source values: "running", "candidate", "startup"
//
// Thread Safety: This method serializes all operations on the same client.
// While NETCONF supports concurrent operations via message-id multiplexing,
// serialization prevents write interleaving and simplifies reconnection logic.
//
// Example:
//
//	ctx := context.Background()
//	filter := netconf.SubtreeFilter("<interfaces/>")
//	res, err := client.GetConfig(ctx, "running", filter)
func (c *Client) GetConfig(ctx context.Context, source string, filter Filter, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Acquire exclusive lock to serialize all operations
	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("get-config"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "get-config",
		Target:    source,
		Filter:    filter,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// EditConfig modifies the configuration of the specified datastore
//
// Valid target values: "candidate", "running"
// The config parameter should contain XML configuration data.
//
// Example:
//
//	ctx := context.Background()
//	config := netconf.Body{}.
//	    Set("interfaces.interface.name", "eth0").
//	    Set("interfaces.interface.enabled", true).String()
//	res, err := client.EditConfig(ctx, "candidate", config,
//	    netconf.DefaultOperation("merge"))
func (c *Client) EditConfig(ctx context.Context, target, config string, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("edit-config"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "edit-config",
		Target:    target,
		Config:    config,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// CopyConfig copies configuration between datastores or from/to URLs
//
// Valid source/target values: "running", "candidate", "startup", or URL
//
// Example:
//
//	ctx := context.Background()
//	res, err := client.CopyConfig(ctx, "running", "startup")
func (c *Client) CopyConfig(ctx context.Context, source, target string, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("copy-config"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "copy-config",
		Target:    target,
		Config:    source, // Source is stored in Config field for copy-config
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// DeleteConfig deletes a configuration datastore
//
// Valid target values: "startup" (cannot delete running or candidate)
//
// Per RFC 6241, only the startup datastore can be deleted.
//
// Example:
//
//	ctx := context.Background()
//	res, err := client.DeleteConfig(ctx, "startup")
func (c *Client) DeleteConfig(ctx context.Context, target string, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("delete-config"); err != nil {
		return Res{}, err
	}

	// Additional check: only startup can be deleted per RFC 6241
	target = strings.TrimSpace(strings.ToLower(target))
	if target != "startup" {
		return Res{}, fmt.Errorf("only 'startup' datastore can be deleted, got: %s", target)
	}

	// Build request
	req := &Req{
		Operation: "delete-config",
		Target:    target,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// Lock locks the specified datastore to prevent other sessions from modifying it
//
// Valid target values: "running", "candidate", "startup"
//
// Per RFC 6241 Section 7.5, a lock prevents other NETCONF sessions from
// performing configuration changes. If another session holds the lock,
// Lock() will automatically retry with exponential backoff until the lock
// becomes available or the context deadline is reached.
//
// Connection Guarantee: This method ensures a stable connection exists
// before attempting to acquire the lock. This prevents race conditions in
// concurrent environments.
//
// IMPORTANT: Always use defer to ensure locks are released even if errors occur.
// Failure to unlock can cause deadlocks and prevent other sessions from operating.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	res, err := client.Lock(ctx, "candidate")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Unlock(ctx, "candidate")  // Always unlock via defer
//
//	// Perform configuration changes...
//	client.EditConfig(ctx, "candidate", config)
//	client.Commit(ctx)
func (c *Client) Lock(ctx context.Context, target string, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Build request (no lock needed - just building a struct)
	req := &Req{
		Operation: "lock",
		Target:    target,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC without holding c.mu - driver has its own synchronization.
	// This differs from other operations (Get, EditConfig, etc.) which hold
	// c.mu during sendRPC(). Lock/Unlock operations are NETCONF protocol
	// operations that don't access client state, so driver-level
	// synchronization is sufficient. Connection validation happens in
	// executeRPC() (line 1582).
	return c.sendRPC(ctx, req)
}

// Unlock unlocks the specified datastore
//
// Connection Guarantee: Like Lock(), this method ensures a stable connection exists
// before attempting to release the lock.
//
// See Lock() for complete lock/unlock documentation and proper usage with defer.
//
// Example:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	res, err := client.Lock(ctx, "candidate")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Unlock(ctx, "candidate")  // Proper defer pattern
func (c *Client) Unlock(ctx context.Context, target string, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Build request (no lock needed - just building a struct)
	req := &Req{
		Operation: "unlock",
		Target:    target,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC without holding c.mu - driver has its own synchronization.
	// See Lock() for explanation of why this differs from other operations.
	return c.sendRPC(ctx, req)
}

// Commit commits the candidate datastore to the running datastore
//
// Requires :candidate capability.
//
// Per RFC 6241 Section 8.3.4.1, confirmed commits require explicit confirmation
// within the timeout period, or the commit will be automatically rolled back.
// This prevents configuration errors from persisting if the session is lost.
//
// Important: Confirmed commits must be followed by a confirmation commit within
// the timeout period. Failure to confirm results in automatic rollback.
//
// Example (Standard Commit):
//
//	ctx := context.Background()
//	res, err := client.Commit(ctx)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// Example (Confirmed Commit with Confirmation):
//
//	ctx := context.Background()
//
//	// Step 1: Issue confirmed commit (auto-rollback after 60 seconds)
//	res, err := client.Commit(ctx, netconf.Confirmed(60))
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Step 2: Verify configuration works (application-specific tests)
//	if err := verifyConfig(); err != nil {
//	    log.Fatal("Config verification failed, will auto-rollback")
//	}
//
//	// Step 3: Confirm commit within timeout to prevent rollback
//	res, err = client.Commit(ctx)  // Confirms the previous commit
//	if err != nil {
//	    log.Fatal(err)
//	}
func (c *Client) Commit(ctx context.Context, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("commit"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "commit",
	}

	// Apply modifiers (which may add confirmed commit parameters)
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// Discard discards changes in the candidate datastore
//
// Requires :candidate capability.
//
// Example:
//
//	ctx := context.Background()
//	res, err := client.Discard(ctx)
func (c *Client) Discard(ctx context.Context, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("discard"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "discard",
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// Validate validates the configuration in the specified source
//
// Valid source values: "candidate", "running", or config URL
// Requires :validate capability.
//
// Example:
//
//	ctx := context.Background()
//	res, err := client.Validate(ctx, "candidate")
func (c *Client) Validate(ctx context.Context, source string, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Acquire exclusive lock to serialize all operations
	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("validate"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "validate",
		Target:    source,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// RPC sends a custom RPC request to the device
//
// Use RPC() for vendor-specific operations not covered by standard NETCONF methods
// (Get, GetConfig, EditConfig, etc.). Common use cases:
//   - Vendor-specific operations (Cisco "show" commands)
//   - Device-specific RPCs (firmware updates, diagnostics, operational commands)
//   - Custom YANG-modeled operations not in the standard NETCONF set
//
// Note: RPC() bypasses semantic validation performed by standard methods.
// The caller is responsible for validating XML input and understanding the
// security implications of custom operations.
//
// Use the dedicated methods for standard operations instead of RPC():
//   - Use Get() instead of RPC("<get>...")
//   - Use EditConfig() instead of RPC("<edit-config>...")
//   - etc.
//
// Example (Cisco IOS-XR):
//
//	ctx := context.Background()
//	rpc := `<get-system-info xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-shellutil-oper"/>`
//	res, err := client.RPC(ctx, rpc)
func (c *Client) RPC(ctx context.Context, rpcXML string, mods ...func(*Req)) (Res, error) {
	// Ensure connection exists (lazy connect if needed)
	if err := c.ensureConnected(); err != nil {
		return Res{}, fmt.Errorf("failed to establish connection: %w", err)
	}

	// Acquire write lock since custom RPCs may modify state
	c.mu.Lock()
	defer c.mu.Unlock()

	// Defensive check: Verify driver still valid after acquiring lock
	if err := c.requireDriver("rpc"); err != nil {
		return Res{}, err
	}

	// Build request
	req := &Req{
		Operation: "rpc",
		Config:    rpcXML,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// ServerCapabilities returns the list of capabilities supported by the server
//
// Returns an empty slice if not connected. After connecting, returns a copy
// of the capabilities slice to prevent external modification.
//
// Example:
//
//	caps := client.ServerCapabilities()
//	for _, cap := range caps {
//	    fmt.Println(cap)
//	}
func (c *Client) ServerCapabilities() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]string, len(c.Capabilities))
	copy(result, c.Capabilities)
	return result
}

// ServerHasCapability checks if the server supports a specific capability
//
// Returns false if not connected or capability not found.
//
// Example:
//
//	if client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
//	    // Use candidate datastore
//	}
func (c *Client) ServerHasCapability(capability string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for _, cap := range c.Capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

// SessionID returns the NETCONF session ID
//
// Returns empty string if not connected.
func (c *Client) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// ServerVersion returns the NETCONF server version
//
// Returns empty string if not connected.
func (c *Client) ServerVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverVersion
}

// HasCredentials returns true if credentials are configured
//
// This method only indicates if credentials exist without exposing
// the actual values.
//
// Example:
//
//	if client.HasCredentials() {
//	    fmt.Println("Client is configured with credentials")
//	}
func (c *Client) HasCredentials() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.username != "" || c.password != "" || c.SSHKeyPath != ""
}

// formatXMLForLogging adds a leading newline to separate XML from log metadata
//
// Adds a newline at the start to make multi-line XML more readable in logs.
//
// Example output:
//
//	[DEBUG] NETCONF RPC request XML operation=get-config
//	<get-config>
//	  <source>
//	    <running></running>
//	  </source>
//	</get-config>
//
// Returns the formatted XML string.
func formatXMLForLogging(xml string) string {
	// Add newline at start for visual separation from log metadata
	return "\n" + xml
}

// prepareXMLForLogging redacts sensitive data and formats XML for logging
//
// This method performs security checks and data sanitization:
//  1. Validates XML size to prevent ReDoS attacks (max 1MB)
//  2. Checks sensitive element count to prevent DoS (max 1000 elements)
//  3. Redacts sensitive data (passwords, secrets, keys, community strings)
//  4. Pretty-prints XML if prettyPrintLogs is enabled
//  5. Formats with line prefixes for readability
//
// Security Note: Size and count limits prevent regex-based DoS attacks during
// XML processing and redaction. These limits are conservative to ensure safe
// operation even with malicious or malformed input.
//
// Returns the processed XML string safe for logging.
func (c *Client) prepareXMLForLogging(xml string) string {
	// Check XML size limit to prevent ReDoS attacks
	if len(xml) > MaxXMLSizeForLogging {
		return XMLTooLargeMessage
	}

	// Count sensitive elements before processing to prevent DoS
	// This check prevents excessive regex operations on malicious input
	sensitiveCount := strings.Count(xml, "<password>") +
		strings.Count(xml, "<secret>") +
		strings.Count(xml, "<key>") +
		strings.Count(xml, "<community>")

	if sensitiveCount > MaxSensitiveElements {
		c.logger.Warn(context.Background(), "Too many sensitive elements detected",
			"count", sensitiveCount,
			"max", MaxSensitiveElements)
		return XMLTooManySensitiveMessage
	}

	// Redact sensitive data first
	redacted := c.redactSensitiveData(xml)

	// Pretty-print XML if enabled using xmldot's @pretty modifier
	if c.prettyPrintLogs {
		// Apply @pretty modifier to format the XML with indentation
		// Note: xmldot's * selector returns the first child element's content,
		// so this will format the response data (e.g., <data> contents) without
		// the RPC envelope (<rpc-reply>). This is intentional as the envelope
		// is just protocol framing and the actual configuration data is more
		// relevant for logging/debugging.
		result := xmldot.Get(redacted, "*|@pretty")
		if result.Exists() && result.Raw != "" {
			// Format with line prefixes for readability
			return formatXMLForLogging(result.Raw)
		}
		// Fallback if pretty printing fails - format raw redacted XML
		return formatXMLForLogging(redacted)
	}

	// Even without pretty printing, format with line prefixes
	return formatXMLForLogging(redacted)
}

// redactSensitiveData replaces sensitive data in XML with [REDACTED]
//
// Redacts common sensitive types in element content, attributes, and XPath predicates:
//   - <password> elements and password="" attributes
//   - <secret> elements and secret="" attributes
//   - <key> elements and key="" attributes
//   - <community> elements and community="" attributes
//   - XPath predicates like [password="value"]
//
// Supports both single-quoted and double-quoted attribute values.
//
// Security Note: This method is called after size/count validation to prevent
// ReDoS attacks from malicious input.
//
// Returns the redacted XML string.
func (c *Client) redactSensitiveData(xml string) string {
	// Custom redaction for elements that handles nested structures
	result := xml
	result = redactNestedElement(result, "password")
	result = redactNestedElement(result, "secret")
	result = redactNestedElement(result, "key")
	result = redactNestedElement(result, "community")

	// Apply regex patterns for CDATA, namespaced elements, and attributes
	replacements := []string{
		// CDATA sections
		"<password><![CDATA[[REDACTED]]]></password>",
		"<secret><![CDATA[[REDACTED]]]></secret>",
		"<key><![CDATA[[REDACTED]]]></key>",
		"<community><![CDATA[[REDACTED]]]></community>",

		// Namespace-aware elements
		"<ns:password>[REDACTED]</ns:password>",
		"<ns:secret>[REDACTED]</ns:secret>",
		"<ns:key>[REDACTED]</ns:key>",
		"<ns:community>[REDACTED]</ns:community>",

		// Namespaced CDATA sections
		"<ns:password><![CDATA[[REDACTED]]]></ns:password>",
		"<ns:secret><![CDATA[[REDACTED]]]></ns:secret>",
		"<ns:key><![CDATA[[REDACTED]]]></ns:key>",
		"<ns:community><![CDATA[[REDACTED]]]></ns:community>",

		// Attributes (double quotes)
		`password="[REDACTED]"`,
		`secret="[REDACTED]"`,
		`key="[REDACTED]"`,
		`community="[REDACTED]"`,

		// Attributes (single quotes)
		`password='[REDACTED]'`,
		`secret='[REDACTED]'`,
		`key='[REDACTED]'`,
		`community='[REDACTED]'`,

		// XPath predicates
		`[password="[REDACTED]"]`,
		`[password='[REDACTED]']`,
		`[secret="[REDACTED]"]`,
		`[secret='[REDACTED]']`,
		`[key="[REDACTED]"]`,
		`[key='[REDACTED]']`,
	}

	// Skip first 4 patterns (elements) since we handle those with redactNestedElement
	for i := 4; i < len(c.redactionPatterns); i++ {
		result = c.redactionPatterns[i].ReplaceAllString(result, replacements[i-4])
	}

	return result
}

// redactNestedElement redacts XML elements that may have nested structures with the same tag name.
// Handles Cisco YANG style nesting: <password><password>value</password></password>
// Returns XML with properly balanced tags: <password>[REDACTED]</password>
func redactNestedElement(xml, tagName string) string {
	openTag := "<" + tagName + ">"
	closeTag := "</" + tagName + ">"
	replacement := openTag + "[REDACTED]" + closeTag

	result := ""
	pos := 0

	for {
		// Find next opening tag
		start := strings.Index(xml[pos:], openTag)
		if start == -1 {
			// No more tags, append remaining
			result += xml[pos:]
			break
		}
		start += pos

		// Copy text before the tag
		result += xml[pos:start]

		// Find matching closing tag (handle nesting)
		depth := 1
		searchPos := start + len(openTag)

		for depth > 0 && searchPos < len(xml) {
			nextOpen := strings.Index(xml[searchPos:], openTag)
			nextClose := strings.Index(xml[searchPos:], closeTag)

			if nextClose == -1 {
				// Malformed XML, skip this tag
				result += xml[start:searchPos]
				pos = searchPos
				break
			}

			if nextOpen != -1 && nextOpen < nextClose {
				// Found nested opening tag
				depth++
				searchPos += nextOpen + len(openTag)
			} else {
				// Found closing tag
				depth--
				searchPos += nextClose + len(closeTag)
			}
		}

		if depth == 0 {
			// Found matching closing tag, replace entire structure
			result += replacement
			pos = searchPos
		}
	}

	return result
}

// checkTransientError checks if an error is transient and should be retried
//
// This method checks both:
// 1. NETCONF rpc-error elements against TransientErrors patterns (errors.go)
// 2. Go errors from scrapligo (timeout, connection, operation errors)
//
// Returns true if either type of error matches transient patterns.
func (c *Client) checkTransientError(errs []ErrorModel, goErr error) bool {
	// Check NETCONF rpc-error elements
	for _, err := range errs {
		for _, pattern := range TransientErrors {
			// Match error type (empty pattern matches any)
			if pattern.ErrorType != "" && err.ErrorType != pattern.ErrorType {
				continue
			}

			// Match error tag (empty pattern matches any)
			if pattern.ErrorTag != "" && err.ErrorTag != pattern.ErrorTag {
				continue
			}

			// Match error message with regex (empty pattern matches any)
			if pattern.ErrorMessage != "" {
				matched, matchErr := regexp.MatchString(pattern.ErrorMessage, err.ErrorMessage)
				if matchErr != nil || !matched {
					continue
				}
			}

			// All non-empty pattern fields matched
			return true
		}
	}

	// Check scrapligo Go errors (timeout, connection, operation errors)
	return isScrapliogoErrorTransient(goErr)
}

// isScrapliogoErrorTransient checks if a scrapligo Go error is transient
//
// These errors indicate temporary conditions that may succeed on retry:
//   - ErrTimeoutError: socket, transport, or channel (ops) timeouts
//   - ErrConnectionError: connection failures (host key verification, etc.)
//   - ErrOperationError: operation timeout issues
//   - io.EOF: connection closed by remote (device closed the session)
//
// EOF errors are particularly common when:
//   - Device closes idle connections (session timeout)
//   - Device restarts or reloads
//   - Network connectivity issues
//   - Concurrent session limits reached
//
// Returns true if the error is transient and should trigger retry.
func isScrapliogoErrorTransient(err error) bool {
	if err == nil {
		return false
	}

	// Check for EOF errors (connection closed by remote)
	if errors.Is(err, io.EOF) {
		return true
	}

	// Check for scrapligo error types
	return errors.Is(err, util.ErrTimeoutError) ||
		errors.Is(err, util.ErrConnectionError) ||
		errors.Is(err, util.ErrOperationError)
}

// hasTransportError checks if errors indicate transport/connection issues
//
// Checks both NETCONF <rpc-error> elements with ErrorType="transport" and
// scrapligo Go errors that indicate broken connections (EOF).
//
// Transport errors require session reconnection before retry to ensure a
// clean connection state.
//
// EOF errors specifically trigger reconnection because they indicate the
// device has closed the connection, and retrying on a closed connection
// will always fail. Common causes:
//   - Idle connection timeout on device
//   - Device restart or reload
//   - Session limit reached
//
// Note: Other scrapligo errors (timeout, operation) are treated as
// transient for retry purposes but do NOT trigger reconnection.
//
// Returns true if transport/connection errors are detected.
func (c *Client) hasTransportError(errs []ErrorModel, goErr error) bool {
	// Check NETCONF rpc-error elements for transport type
	for _, rpcErr := range errs {
		if rpcErr.ErrorType == transportErrType {
			return true
		}
	}

	// Check for EOF - indicates connection closed by device
	// Must reconnect to establish new session
	if goErr != nil && errors.Is(goErr, io.EOF) {
		return true
	}

	return false
}

// isLockDeniedError checks if errors indicate lock-denied or in-use conditions.
//
// Lock-denied errors require special polling behavior with fixed 1-second intervals
// instead of exponential backoff, and use LockReleaseTimeout instead of MaxRetries.
//
// This matches the NETCONF operational pattern where locks are typically released
// within seconds to minutes, and polling is more efficient than exponential backoff.
//
// Returns true if any error in the list is a lock contention error.
func (c *Client) isLockDeniedError(errs []ErrorModel) bool {
	for _, err := range errs {
		if err.ErrorTag == "lock-denied" || err.ErrorTag == "in-use" {
			return true
		}
	}
	return false
}

// Backoff calculates the backoff delay for retry attempt using exponential backoff with jitter
//
// The formula is: delay = min(minDelay * (factor ^ attempt) + jitter, maxDelay)
// where jitter is a cryptographically random value in [0, delay * 0.1].
//
// Parameters:
//   - attempt: The retry attempt number (0-indexed)
//
// Returns the duration to wait before retrying.
func (c *Client) Backoff(attempt int) time.Duration {
	// Calculate base delay: minDelay * (factor ^ attempt)
	delay := float64(c.BackoffMinDelay) * math.Pow(c.BackoffDelayFactor, float64(attempt))

	// Check for overflow and cap at max delay
	if math.IsInf(delay, 1) || delay > float64(c.BackoffMaxDelay) {
		delay = float64(c.BackoffMaxDelay)
	}

	// Add cryptographically secure jitter (0-10% of delay) to prevent thundering herd
	jitterMax := big.NewInt(int64(delay * 0.1))
	if jitterMax.Int64() > 0 {
		jitterBig, err := rand.Int(rand.Reader, jitterMax)
		if err != nil {
			// Fallback to no jitter on error (better than predictable jitter)
			jitterBig = big.NewInt(0)
		}
		delay += float64(jitterBig.Int64())
	}

	return time.Duration(delay)
}

// reconnect attempts to reconnect the NETCONF session
//
// This method closes the existing connection and establishes a new one,
// re-negotiating capabilities. Used internally when transport errors are
// detected during retry logic.
//
// Timeout Protection: The close operation uses ConnectTimeout to prevent
// indefinite blocking. Close errors are logged but do not prevent reconnection
// (connection may already be broken).
//
// PRECONDITION: Caller must NOT hold any locks (acquires write lock internally)
//
// Returns an error if reconnection fails.
func (c *Client) reconnect() error {
	c.logger.Info(context.Background(), "NETCONF reconnecting",
		"host", c.Host)

	// Acquire write lock for exclusive access to driver state
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close existing connection with timeout protection (scrapligo v1.3.3 bug workaround)
	// Errors are ignored since connection may already be broken
	if c.driver != nil {
		driver := c.driver
		c.driver = nil

		ctx, cancel := context.WithTimeout(context.Background(), c.ConnectTimeout)
		defer cancel()

		closeDone := make(chan error, 1) // Buffered to prevent goroutine leak
		go func() {
			closeDone <- driver.Close()
		}()

		select {
		case <-closeDone:
			// Close completed, ignore any errors (connection likely broken)
		case <-ctx.Done():
			c.logger.Warn(context.Background(), "NETCONF driver close timeout during reconnect",
				"host", c.Host,
				"timeout", c.ConnectTimeout)
			// Continue with reconnection even if close timed out
		}
	}

	// Reuse connect() method for DRY principle
	return c.connect()
}

// handleTransportErrorReconnect handles transport error reconnection with proper lock management.
// This helper reduces cyclomatic complexity by extracting the lock release/reacquire logic.
//
// Parameters:
//   - ctx: Context for logging
//   - req: Request that failed with transport error
//
// Returns an error if reconnection fails.
func (c *Client) handleTransportErrorReconnect(ctx context.Context, req *Req) error {
	c.logger.Info(ctx, "NETCONF transport error detected, reconnecting",
		"operation", req.Operation)

	// Determine lock type held by caller based on operation type
	// Read operations: get, get-config, validate (hold RLock)
	// Write operations: edit-config, copy-config, delete-config, commit, discard, rpc (hold Lock)
	// Lock operations: lock, unlock (hold no lock)
	isReadOp := req.Operation == opGet || req.Operation == opGetConfig || req.Operation == opValidate
	isWriteOp := req.Operation == opEditConfig || req.Operation == opCopyConfig ||
		req.Operation == opDeleteConfig || req.Operation == opCommit ||
		req.Operation == opDiscard || req.Operation == opRPC

	// Release lock before reconnect (reconnect acquires its own write lock)
	if isReadOp {
		c.mu.RUnlock()
	} else if isWriteOp {
		c.mu.Unlock()
	}
	// Lock/Unlock operations don't hold c.mu, so nothing to release

	// Attempt to reconnect (acquires and releases its own write lock)
	reconnectErr := c.reconnect()

	// Reacquire original lock type
	if isReadOp {
		c.mu.RLock()
	} else if isWriteOp {
		c.mu.Lock()
	}
	// Lock/Unlock operations don't hold c.mu, so nothing to reacquire

	return reconnectErr
}

// sendRPC sends a NETCONF RPC request via scrapligo and parses the response
//
// This method handles all NETCONF operations by dispatching to the appropriate
// scrapligo driver method based on the request operation type. It includes
// automatic retry logic for transient errors and session reconnection for
// transport errors.
//
// Thread Safety:
//   - Read operations (get, get-config): Caller MUST hold c.mu.RLock()
//   - Write operations (edit-config, copy-config, delete-config): Caller MUST hold c.mu.Lock()
//   - Rationale: Write operations modify device state and must be serialized.
//     Read operations can execute concurrently.
//
// Lock Ordering:
//
//	The lock must be acquired in the calling method (Get, GetConfig, EditConfig, etc.)
//	BEFORE calling sendRPC to prevent deadlocks and ensure consistent access patterns.
//
// Context Handling:
//   - Request-specific timeout (req.Timeout) overrides any existing context deadline
//   - If no req.Timeout and context has no deadline, applies c.OperationTimeout
//   - Cancellation checked before operation start and between retries
//
// Error Handling & Retry Logic:
//   - Transient errors (lock-denied, in-use, transport) trigger automatic retry
//   - Transport errors trigger session reconnection before retry
//   - Non-transient errors fail immediately without retry
//   - Retry count and transient status included in NetconfError
//   - Exponential backoff with jitter applied between retries
//
// Note: This is an internal method. All input validation must be performed
// by the caller before invoking sendRPC.
//
// Returns a parsed Res or an error if the RPC fails.
func (c *Client) sendRPC(ctx context.Context, req *Req) (Res, error) {
	// Apply timeout with proper priority:
	// 1. Request-specific timeout (highest priority)
	// 2. Context deadline (if already set)
	// 3. Default total timeout (fallback - spans all retry attempts)
	if req.Timeout > 0 {
		// Request-specific timeout takes precedence
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	} else if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		// Only apply default if no deadline already set
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.TotalTimeout)
		defer cancel()
	}

	// Track lock-denied polling state
	var lockDeniedStart time.Time
	isLockPolling := false

	// Retry loop - unbounded to allow lock-denied polling until LockReleaseTimeout
	// Safety mechanisms prevent infinite loops:
	// 1. Lock operations: time-bound by LockReleaseTimeout check and context cancellation
	// 2. Non-lock operations: attempt-bound by MaxRetries check in exit condition below
	// 3. Non-transient errors: exit immediately via exit condition
	// 4. Context cancellation: checked before each attempt and during backoff
	for attempt := 0; ; attempt++ {
		// Check context before attempt
		select {
		case <-ctx.Done():
			return Res{}, fmt.Errorf("operation canceled: %w", ctx.Err())
		default:
		}

		// Log retry attempts
		if attempt > 0 {
			c.logger.Warn(ctx, "NETCONF operation retry",
				"operation", req.Operation,
				"attempt", attempt,
				"maxRetries", c.MaxRetries)
		}

		// Log operation start (Debug level)
		c.logger.Debug(ctx, "NETCONF RPC request",
			"operation", req.Operation,
			"target", req.Target,
			"sessionID", c.sessionID)

		// Execute RPC
		res, err := c.executeRPC(ctx, req)

		// Log operation completion (Debug level)
		if err == nil {
			c.logger.Debug(ctx, "NETCONF RPC response",
				"operation", req.Operation,
				"ok", res.OK,
				"errorCount", len(res.Errors),
				"messageID", res.MessageID)
		}

		// Success case
		if err == nil && len(res.Errors) == 0 {
			return res, nil
		}

		// Check if transient (checks both NETCONF rpc-errors and scrapligo Go errors)
		isTransient := c.checkTransientError(res.Errors, err)

		// Check if this is a lock-denied error (requires special polling behavior)
		isLockDenied := c.isLockDeniedError(res.Errors)

		// Log transient detection for debugging
		if err != nil && isTransient {
			c.logger.Debug(ctx, "NETCONF transient error detected",
				"operation", req.Operation,
				"attempt", attempt,
				"lockDenied", isLockDenied,
				"error", err.Error())
		}

		// Check for transport/connection errors that require reconnection
		// Both NETCONF <rpc-error> transport errors and EOF errors trigger reconnection
		// IMPORTANT: Handle transport errors BEFORE lock polling to ensure reconnection happens first
		hasTransportError := c.hasTransportError(res.Errors, err)

		// Handle transport errors with reconnection (PRIORITY: Handle before lock polling)
		if hasTransportError {
			// Use helper to handle lock release/reacquire around reconnection
			if reconnectErr := c.handleTransportErrorReconnect(ctx, req); reconnectErr != nil {
				// Reconnection failed, return original error
				return res, &NetconfError{
					Operation:   req.Operation,
					Errors:      res.Errors,
					Message:     "operation failed and reconnection failed",
					InternalMsg: reconnectErr.Error(),
					Retries:     attempt,
					IsTransient: true,
				}
			}

			c.logger.Info(ctx, "NETCONF reconnection successful",
				"operation", req.Operation,
				"sessionID", c.sessionID)

			// Reset lock polling state after reconnection
			isLockPolling = false
			lockDeniedStart = time.Time{}
			// Reconnection succeeded, continue to backoff and retry
		}

		// Handle lock-denied with special timeout-based polling (AFTER transport error handling)
		if isLockDenied {
			// Check context cancellation before lock timeout (defensive check)
			select {
			case <-ctx.Done():
				return Res{}, fmt.Errorf("operation canceled during lock polling: %w", ctx.Err())
			default:
			}

			// Start lock polling timer on first lock-denied
			if !isLockPolling {
				isLockPolling = true
				lockDeniedStart = time.Now()
				c.logger.Info(ctx, "NETCONF waiting for lock release",
					"operation", req.Operation,
					"target", req.Target,
					"timeout", c.LockReleaseTimeout.String())
			}

			// Check if lock polling has exceeded timeout - return ErrLockReleaseTimeout
			lockPollingElapsed := time.Since(lockDeniedStart)
			if lockPollingElapsed >= c.LockReleaseTimeout {
				// Lock timeout exceeded, return error
				c.logger.Error(ctx, "NETCONF lock release timeout exceeded",
					"operation", req.Operation,
					"elapsed", lockPollingElapsed.String(),
					"timeout", c.LockReleaseTimeout.String())

				// Log each RPC error for context
				for i, rpcErr := range res.Errors {
					c.logger.Error(ctx, "NETCONF RPC error",
						"index", i,
						"errorType", rpcErr.ErrorType,
						"errorTag", rpcErr.ErrorTag,
						"errorMessage", rpcErr.ErrorMessage)
				}

				return res, &NetconfError{
					Operation:   req.Operation,
					Errors:      res.Errors,
					Message:     ErrLockReleaseTimeout.Error(),
					InternalMsg: fmt.Sprintf("waited %s for lock release", lockPollingElapsed),
					Retries:     attempt,
					IsTransient: true,
				}
			}
		}

		// Not transient or (non-lock transient and max retries reached)
		if !isTransient || (!isLockDenied && attempt >= c.MaxRetries) {
			// Log operation failure with error details
			if err != nil {
				c.logger.Error(ctx, "NETCONF operation failed",
					"operation", req.Operation,
					"retries", attempt,
					"transient", isTransient,
					"errorCount", len(res.Errors),
					"error", err.Error())
			} else {
				c.logger.Error(ctx, "NETCONF operation failed",
					"operation", req.Operation,
					"retries", attempt,
					"transient", isTransient,
					"errorCount", len(res.Errors))
			}

			// Log each RPC error
			for i, rpcErr := range res.Errors {
				c.logger.Error(ctx, "NETCONF RPC error",
					"index", i,
					"errorType", rpcErr.ErrorType,
					"errorTag", rpcErr.ErrorTag,
					"errorMessage", rpcErr.ErrorMessage)
			}

			// Return error with retry information
			if len(res.Errors) > 0 {
				return res, &NetconfError{
					Operation:   req.Operation,
					Errors:      res.Errors,
					Message:     "operation failed",
					Retries:     attempt,
					IsTransient: isTransient,
				}
			}
			return res, fmt.Errorf("operation %s failed after %d retries: %w", req.Operation, attempt, err)
		}

		// Apply backoff before next retry
		var delay time.Duration
		if isLockDenied {
			// Use fixed 1-second interval for lock polling
			delay = 1 * time.Second
		} else {
			// Use exponential backoff for other transient errors
			delay = c.Backoff(attempt)
		}

		c.logger.Debug(ctx, "NETCONF retry backoff",
			"operation", req.Operation,
			"attempt", attempt,
			"delay", delay.String(),
			"lockPolling", isLockPolling)

		select {
		case <-ctx.Done():
			return Res{}, fmt.Errorf("operation canceled during backoff: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	// Unreachable: All exit paths return earlier via error conditions or context cancellation
	panic(fmt.Sprintf("BUG: retry loop exited without return for operation %s (attempt %d)", req.Operation, 0))
}

// executeRPC executes a single RPC operation without retry logic
//
// This method dispatches to the appropriate scrapligo driver method based
// on the request operation type and returns the parsed response.
//
// NOTE: The ctx parameter is accepted for future compatibility but is not
// currently used because scrapligo driver methods do not accept context.
// Timeout enforcement is handled by the caller (sendRPC).
//
// PRECONDITION: Caller must hold appropriate lock (RLock for reads, Lock for writes).
// PRECONDITION: Context must have timeout applied by caller.
//
// Returns a parsed Res or an error if the RPC fails.
func (c *Client) executeRPC(ctx context.Context, req *Req) (Res, error) {
	_ = ctx // Accepted for future compatibility, timeout enforced by caller

	// Check for nil driver before operation
	if c.driver == nil {
		return Res{}, fmt.Errorf("operation %s failed: driver is nil (connection closed)", req.Operation)
	}

	// Log request XML before sending
	c.logRequestXML(ctx, req)

	// Dispatch operation to scrapligo driver
	scrapligoRes, err := c.dispatchOperation(req)

	// Handle operation error
	if err != nil {
		return Res{}, c.formatOperationError(req, err)
	}

	// Check for nil response
	if scrapligoRes == nil {
		return Res{}, fmt.Errorf("operation %s: received nil response from driver", req.Operation)
	}

	// Log response XML
	c.logResponseXML(ctx, req.Operation, scrapligoRes.Result)

	// Parse response XML
	return c.parseResponse(scrapligoRes)
}

// getRequestXMLForLogging determines what XML content to log based on operation type.
func (c *Client) getRequestXMLForLogging(req *Req) string {
	switch req.Operation {
	case opEditConfig:
		// For edit-config, ensure <config> wrapper is present (matching what will be sent)
		if !xmldot.Get(req.Config, "config").Exists() {
			return "<config>" + req.Config + "</config>"
		}
		return req.Config
	case opGetConfig, opGet:
		return req.Filter.Content
	case opLock, opUnlock, opCommit, opDiscard, opValidate:
		return "" // Simple operations have no XML content
	default:
		return req.Config // For other operations (rpc, copy-config, etc.)
	}
}

// logRequestXML logs the request XML content if present.
func (c *Client) logRequestXML(ctx context.Context, req *Req) {
	xmlToLog := c.getRequestXMLForLogging(req)
	if xmlToLog == "" {
		return
	}

	if len(xmlToLog) > MaxXMLSizeForLogging {
		c.logger.Debug(ctx, "NETCONF RPC request XML (truncated)",
			"operation", req.Operation,
			"size", len(xmlToLog),
			"limit", MaxXMLSizeForLogging,
			"xml", "[XML TOO LARGE FOR LOGGING]")
		return
	}

	if !utf8.ValidString(xmlToLog) {
		c.logger.Warn(ctx, "Invalid UTF-8 in NETCONF request XML",
			"operation", req.Operation,
			"size", len(xmlToLog))
		return
	}

	requestXML := c.prepareXMLForLogging(xmlToLog)
	c.logger.Debug(ctx, "NETCONF RPC request XML",
		"operation", req.Operation,
		"xml", requestXML)
}

// logResponseXML logs the response XML content.
func (c *Client) logResponseXML(ctx context.Context, operation string, result string) {
	if result == "" {
		return
	}

	if len(result) > MaxXMLSizeForLogging {
		c.logger.Debug(ctx, "NETCONF RPC response XML (truncated)",
			"operation", operation,
			"size", len(result),
			"limit", MaxXMLSizeForLogging,
			"xml", "[XML TOO LARGE FOR LOGGING]")
		return
	}

	if !utf8.ValidString(result) {
		c.logger.Warn(ctx, "Invalid UTF-8 in NETCONF response XML",
			"operation", operation,
			"size", len(result))
		return
	}

	responseXML := c.prepareXMLForLogging(result)
	c.logger.Debug(ctx, "NETCONF RPC response XML",
		"operation", operation,
		"xml", responseXML)
}

// formatOperationError formats an operation error with context.
func (c *Client) formatOperationError(req *Req, err error) error {
	if req.Operation == opEditConfig {
		return fmt.Errorf("operation %s failed (target=%s, configSize=%d): %w",
			req.Operation, req.Target, len(req.Config), err)
	}
	return fmt.Errorf("operation %s failed (target=%s): %w", req.Operation, req.Target, err)
}

// dispatchOperation dispatches the operation to the appropriate scrapligo driver method.
func (c *Client) dispatchOperation(req *Req) (*response.NetconfResponse, error) {
	switch req.Operation {
	case opGet:
		return c.driver.Get(req.Filter.Content)
	case opGetConfig:
		return c.executeGetConfig(req)
	case opEditConfig:
		return c.executeEditConfig(req)
	case opCopyConfig:
		return c.driver.CopyConfig(req.Config, req.Target)
	case opDeleteConfig:
		return c.driver.DeleteConfig(req.Target)
	case opLock:
		return c.driver.Lock(req.Target)
	case opUnlock:
		return c.driver.Unlock(req.Target)
	case opCommit:
		return c.executeCommit(req)
	case opDiscard:
		return c.driver.Discard()
	case opValidate:
		return c.driver.Validate(req.Target)
	case opRPC:
		return c.driver.RPC(opoptions.WithFilter(req.Config))
	default:
		return nil, fmt.Errorf("unsupported operation: %s", req.Operation)
	}
}

// executeGetConfig executes a get-config operation with filter options.
func (c *Client) executeGetConfig(req *Req) (*response.NetconfResponse, error) {
	var opts []util.Option
	if req.Filter.Content != "" {
		opts = append(opts, opoptions.WithFilter(req.Filter.Content))
		if req.Filter.Type == filterTypeXPath {
			opts = append(opts, opoptions.WithFilterType(filterTypeXPath))
		}
	}
	return c.driver.GetConfig(req.Target, opts...)
}

// executeEditConfig executes an edit-config operation.
func (c *Client) executeEditConfig(req *Req) (*response.NetconfResponse, error) {
	// Build edit-config XML and send via RPC
	rpcXML := c.buildEditConfigXML(req)
	return c.driver.RPC(opoptions.WithFilter(rpcXML))
}

// executeCommit executes a commit operation with optional confirmed commit.
func (c *Client) executeCommit(req *Req) (*response.NetconfResponse, error) {
	var opts []util.Option
	if req.ConfirmTimeout > 0 {
		opts = append(opts, opoptions.WithCommitConfirmed())
		opts = append(opts, opoptions.WithCommitConfirmTimeout(uint(req.ConfirmTimeout)))
	}
	if req.PersistID != "" {
		opts = append(opts, opoptions.WithCommitConfirmedPersistID(req.PersistID))
	}
	return c.driver.Commit(opts...)
}

// parseResponse parses a NETCONF response from scrapligo
//
// This method:
//   - Parses XML with xmldot for efficient querying
//   - Checks for <ok/> response
//   - Extracts <rpc-error> elements
//   - Extracts message-id attribute
//
// PRECONDITION: scrapligoRes must not be nil (checked by caller).
//
// Returns a Res struct with parsed data or an error if parsing fails.
func (c *Client) parseResponse(scrapligoRes *response.NetconfResponse) (Res, error) {

	// Apply response preprocessor if configured (e.g., to escape malformed XML)
	rawXML := scrapligoRes.Result
	if c.ResponsePreprocessor != nil {
		rawXML = c.ResponsePreprocessor(rawXML)
	}

	// Parse XML with xmldot
	result := xmldot.Get(rawXML, "rpc-reply")

	// Check for <ok/> response
	okResult := xmldot.Get(rawXML, "rpc-reply.ok")
	ok := okResult.Exists()

	// Check for <rpc-error> elements
	errors := c.parseRPCErrors(rawXML)

	// Extract message-id
	msgID := xmldot.Get(rawXML, "rpc-reply@message-id").String()

	return Res{
		Res:       result,
		OK:        ok,
		Errors:    errors,
		MessageID: msgID,
	}, nil
}

// parseRPCErrors extracts NETCONF rpc-error elements from xmldot result
//
// This method parses error structures according to RFC 6241 Section 4.3:
//   - error-type: transport, rpc, protocol, application
//   - error-tag: operation-failed, invalid-value, etc.
//   - error-severity: error, warning
//   - error-message: human-readable message
//   - error-info: additional error information
//
// Extracts all rpc-error elements as RFC 6241 allows multiple errors
// in a single response.
//
// Returns a slice of ErrorModel structs.
func (c *Client) parseRPCErrors(responseXML string) []ErrorModel {
	var errors []ErrorModel

	// Check if any rpc-error exists
	firstError := xmldot.Get(responseXML, "rpc-reply.rpc-error")
	if !firstError.Exists() {
		return errors
	}

	// Iterate through all rpc-error elements
	// xmldot follows gjson-like pattern where arrays can be indexed
	// Try indices 0, 1, 2, ... until no more elements exist
	for i := 0; ; i++ {
		// Build path for this error index
		basePath := fmt.Sprintf("rpc-reply.rpc-error.%d", i)

		// Check if this error exists
		errorNode := xmldot.Get(responseXML, basePath)
		if !errorNode.Exists() {
			// No more errors at this index
			break
		}

		// Extract error fields
		errorModel := ErrorModel{
			ErrorType:     xmldot.Get(responseXML, basePath+".error-type").String(),
			ErrorTag:      xmldot.Get(responseXML, basePath+".error-tag").String(),
			ErrorSeverity: xmldot.Get(responseXML, basePath+".error-severity").String(),
			ErrorAppTag:   xmldot.Get(responseXML, basePath+".error-app-tag").String(),
			ErrorPath:     xmldot.Get(responseXML, basePath+".error-path").String(),
			ErrorMessage:  xmldot.Get(responseXML, basePath+".error-message").String(),
			ErrorInfo:     xmldot.Get(responseXML, basePath+".error-info").String(),
		}

		errors = append(errors, errorModel)

		// Safety limit to prevent infinite loop (RFC 6241 doesn't specify max)
		// In practice, responses rarely have more than 100 errors
		if i >= 1000 {
			// Log warning but return what we have
			// This is a defensive measure that should never trigger in practice
			break
		}
	}

	// Fallback: If array indexing didn't work (depending on xmldot implementation),
	// ensure we at least got the first error
	if len(errors) == 0 {
		// Try extracting first error without array index
		errorModel := ErrorModel{
			ErrorType:     xmldot.Get(responseXML, "rpc-reply.rpc-error.error-type").String(),
			ErrorTag:      xmldot.Get(responseXML, "rpc-reply.rpc-error.error-tag").String(),
			ErrorSeverity: xmldot.Get(responseXML, "rpc-reply.rpc-error.error-severity").String(),
			ErrorAppTag:   xmldot.Get(responseXML, "rpc-reply.rpc-error.error-app-tag").String(),
			ErrorPath:     xmldot.Get(responseXML, "rpc-reply.rpc-error.error-path").String(),
			ErrorMessage:  xmldot.Get(responseXML, "rpc-reply.rpc-error.error-message").String(),
			ErrorInfo:     xmldot.Get(responseXML, "rpc-reply.rpc-error.error-info").String(),
		}
		errors = append(errors, errorModel)
	}

	return errors
}

// buildEditConfigXML builds a complete edit-config RPC XML with advanced options
//
// This method constructs the full edit-config RPC according to RFC 6241 Section 7.2,
// including optional default-operation, test-option, and error-option elements.
//
// Per RFC 6241, the <config> element in edit-config should contain the configuration
// data directly, not wrapped in another <config> element. If the caller provides
// configuration with an outer <config> wrapper (e.g., from Body builder), this method
// extracts the inner content automatically.
//
// Uses xmldot for safe XML building with automatic escaping.
//
// Returns the complete RPC XML string.
func (c *Client) buildEditConfigXML(req *Req) string {
	// Start with edit-config root element
	xml := "<edit-config></edit-config>"

	// Target datastore (as empty element: <candidate/>, <running/>, etc.)
	xml, _ = xmldot.SetRaw(xml, "edit-config.target", "<"+req.Target+"/>") //nolint:errcheck // XML building errors caught during validation

	// Default operation (optional) - xmldot automatically escapes the value
	if req.DefaultOperation != "" {
		xml, _ = xmldot.Set(xml, "edit-config.default-operation", req.DefaultOperation) //nolint:errcheck // XML building errors caught during validation
	}

	// Test option (optional) - xmldot automatically escapes the value
	if req.TestOption != "" {
		xml, _ = xmldot.Set(xml, "edit-config.test-option", req.TestOption) //nolint:errcheck // XML building errors caught during validation
	}

	// Error option (optional) - xmldot automatically escapes the value
	if req.ErrorOption != "" {
		xml, _ = xmldot.Set(xml, "edit-config.error-option", req.ErrorOption) //nolint:errcheck // XML building errors caught during validation
	}

	// Extract config content - strip outer <config> wrapper if present
	// Per RFC 6241, edit-config's <config> element should contain the configuration
	// data directly, not wrapped in another <config> element
	configContent := req.Config

	// Check if user provided <config>...</config> wrapper and extract inner content
	result := xmldot.Get(req.Config, "config")
	if result.Exists() {
		// User provided <config> wrapper (e.g., from Body builder), extract inner XML content
		// Use |@raw modifier to get the raw inner XML without the <config> wrapper
		innerContent := xmldot.Get(req.Config, "config|@raw").String()
		if innerContent != "" {
			configContent = innerContent
		}
	}

	// Config data (raw XML content without outer <config> wrapper)
	xml, _ = xmldot.SetRaw(xml, "edit-config.config", configContent) //nolint:errcheck // XML building errors caught during validation

	return xml
}
