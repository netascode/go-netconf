// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"context"
	"crypto/rand"
	"fmt"
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
	"github.com/scrapli/scrapligo/util"
)

// NETCONF capability URNs and filter types
const (
	netconfBase10URN = "urn:ietf:params:netconf:base:1.0"
	transportErrType = "transport"
	filterTypeXPath  = "xpath"
)

// Default client configuration values
const (
	DefaultPort               = 830
	DefaultMaxRetries         = 3
	DefaultBackoffMinDelay    = 1 * time.Second
	DefaultBackoffMaxDelay    = 60 * time.Second
	DefaultBackoffDelayFactor = 2
	DefaultLockReleaseTimeout = 120 * time.Second
	DefaultConnectTimeout     = 30 * time.Second
	DefaultOperationTimeout   = 60 * time.Second
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
	// Element content
	regexp.MustCompile(`<password>.*?</password>`),
	regexp.MustCompile(`<secret>.*?</secret>`),
	regexp.MustCompile(`<key>.*?</key>`),
	regexp.MustCompile(`<community>.*?</community>`),

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
	OperationTimeout   time.Duration

	// Capability tracking
	Capabilities []string

	// Session information
	sessionID     string
	serverVersion string

	// Logging configuration
	logger            Logger
	prettyPrintLogs   bool
	redactionPatterns []*regexp.Regexp
}

// NewClient creates a new NETCONF client with the specified host and options
//
// The client establishes a connection to the NETCONF server and performs
// capability exchange. Use functional options to configure authentication
// and behavior.
//
// Example:
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
// Returns a configured Client or an error if connection fails.
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
		OperationTimeout:   DefaultOperationTimeout,
		logger:             &NoOpLogger{},
		prettyPrintLogs:    DefaultPrettyPrintLogs,
		redactionPatterns:  defaultRedactionPatterns,
	}

	// Apply functional options
	for _, opt := range opts {
		opt(client)
	}

	// Build scrapligo options
	scrapliOpts := []util.Option{
		options.WithAuthUsername(client.username),
		options.WithAuthPassword(client.password),
		options.WithPort(client.Port),
		options.WithTimeoutSocket(client.ConnectTimeout),
	}

	// Only disable host key verification if explicitly requested
	if client.InsecureSkipVerify {
		scrapliOpts = append(scrapliOpts, options.WithAuthNoStrictKey())
	}

	// Add SSH key authentication if provided
	if client.SSHKeyPath != "" {
		// scrapligo expects key path and passphrase
		scrapliOpts = append(scrapliOpts, options.WithAuthPrivateKey(client.SSHKeyPath, ""))
	}

	// Create NETCONF driver
	driver, err := netconf.NewDriver(host, scrapliOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create NETCONF driver: %w", err)
	}

	// Open connection and perform capability exchange
	err = driver.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open NETCONF connection: %w", err)
	}

	// Store driver and capabilities
	client.driver = driver
	client.Capabilities = driver.ServerCapabilities()

	// Extract session information
	client.sessionID = fmt.Sprintf("%d", driver.SessionID())

	// Extract server version from capabilities if available
	for _, cap := range client.Capabilities {
		// Look for base capability to determine version
		if cap == netconfBase10URN ||
			cap == "urn:ietf:params:netconf:base:1.1" {
			client.serverVersion = driver.SelectedVersion
			break
		}
	}

	// Log successful connection
	client.logger.Info(context.Background(), "NETCONF connection established",
		"host", client.Host,
		"port", client.Port,
		"sessionID", client.sessionID,
		"version", client.serverVersion)

	client.logger.Debug(context.Background(), "NETCONF capabilities discovered",
		"count", len(client.Capabilities))

	return client, nil
}

// Close closes the NETCONF session and cleans up resources
//
// This sends a close-session RPC to the server and closes the underlying
// transport connection. The driver reference is cleared before closing to
// prevent double-close attempts if Close() is called multiple times.
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

	err := driver.Close()
	if err != nil {
		return fmt.Errorf("failed to close NETCONF session: %w", err)
	}

	c.logger.Info(context.Background(), "NETCONF connection closed",
		"host", c.Host,
		"sessionID", c.sessionID)

	return nil
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
	// Acquire read lock before accessing driver
	c.mu.RLock()
	defer c.mu.RUnlock()

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
// Example:
//
//	ctx := context.Background()
//	filter := netconf.SubtreeFilter("<interfaces/>")
//	res, err := client.GetConfig(ctx, "running", filter)
func (c *Client) GetConfig(ctx context.Context, source string, filter Filter, mods ...func(*Req)) (Res, error) {
	// Acquire read lock before accessing driver
	c.mu.RLock()
	defer c.mu.RUnlock()

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
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.Lock()
	defer c.mu.Unlock()

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
// Lock() will block until the lock becomes available or the operation times out.
//
// IMPORTANT: Always use defer to ensure locks are released even if errors occur.
// Failure to unlock can cause deadlocks and prevent other sessions from operating.
//
// Example:
//
//	ctx := context.Background()
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
	// Acquire write lock since this modifies device state
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build request
	req := &Req{
		Operation: "lock",
		Target:    target,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
	return c.sendRPC(ctx, req)
}

// Unlock unlocks the specified datastore
//
// See Lock() for complete lock/unlock documentation and proper usage with defer.
//
// Example:
//
//	ctx := context.Background()
//	res, err := client.Lock(ctx, "candidate")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Unlock(ctx, "candidate")  // Proper defer pattern
func (c *Client) Unlock(ctx context.Context, target string, mods ...func(*Req)) (Res, error) {
	// Acquire write lock since this modifies device state
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build request
	req := &Req{
		Operation: "unlock",
		Target:    target,
	}

	// Apply modifiers
	for _, mod := range mods {
		mod(req)
	}

	// Send RPC and parse response
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
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.Lock()
	defer c.mu.Unlock()

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
	// Acquire read lock - validation is read-only and doesn't modify
	// datastore state, so concurrent validations are safe
	c.mu.RLock()
	defer c.mu.RUnlock()

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
	// Acquire write lock since custom RPCs may modify state
	c.mu.Lock()
	defer c.mu.Unlock()

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
// Returns a copy of the capabilities slice to prevent external modification.
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
func (c *Client) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// ServerVersion returns the NETCONF server version
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

// prepareXMLForLogging redacts sensitive data and formats XML for logging
//
// This method performs security checks and data sanitization:
//  1. Validates XML size to prevent ReDoS attacks (max 1MB)
//  2. Checks sensitive element count to prevent DoS (max 1000 elements)
//  3. Redacts sensitive data (passwords, secrets, keys, community strings)
//  4. Pretty-prints XML if prettyPrintLogs is enabled
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

	// Format with xmldot's @pretty modifier (or return as-is if disabled)
	if c.prettyPrintLogs {
		// Use xmldot to parse and pretty-print the XML
		// Get the root element first, then apply @pretty
		result := xmldot.Get(redacted, "@pretty")
		if result.Exists() {
			return result.Raw
		}
		// Fallback if @pretty doesn't work - return redacted as-is
	}

	return redacted
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
	replacements := []string{
		// Elements
		"<password>[REDACTED]</password>",
		"<secret>[REDACTED]</secret>",
		"<key>[REDACTED]</key>",
		"<community>[REDACTED]</community>",

		// CDATA sections (must match pattern order)
		"<password><![CDATA[[REDACTED]]]></password>",
		"<secret><![CDATA[[REDACTED]]]></secret>",
		"<key><![CDATA[[REDACTED]]]></key>",
		"<community><![CDATA[[REDACTED]]]></community>",

		// Namespace-aware elements (generic replacement works for any namespace)
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

	result := xml
	for i, pattern := range c.redactionPatterns {
		result = pattern.ReplaceAllString(result, replacements[i])
	}

	return result
}

// checkTransientError checks if an error is transient and should be retried
//
// This method matches error patterns against the TransientErrors list defined
// in errors.go. A match on any pattern field (ErrorType, ErrorTag, ErrorMessage)
// indicates a transient error.
//
// Returns true if the error matches any transient pattern.
func (c *Client) checkTransientError(errs []ErrorModel) bool {
	if len(errs) == 0 {
		return false
	}

	// Check each error against transient patterns
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

// waitForLockRelease waits for a datastore lock to be released
//
// This method polls the device to check if the lock is still held and waits
// for it to be released. Used when a lock-denied error is encountered.
//
// Parameters:
//   - ctx: Context for cancellation
//   - target: Datastore name ("running", "candidate", "startup")
//
// Returns an error if the lock is not released within LockReleaseTimeout.
func (c *Client) waitForLockRelease(ctx context.Context, target string) error {
	c.logger.Info(ctx, "NETCONF waiting for lock release",
		"target", target,
		"timeout", c.LockReleaseTimeout.String())

	// Apply lock release timeout
	ctx, cancel := context.WithTimeout(ctx, c.LockReleaseTimeout)
	defer cancel()

	// Poll interval (1 second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ErrLockReleaseTimeout
		case <-ticker.C:
			// Try to acquire lock
			res, err := c.Lock(ctx, target)
			if err == nil && res.OK {
				// Lock acquired, release it immediately to verify availability
				// Note: ignoring unlock errors is intentional - we proved lock availability
				_, _ = c.Unlock(ctx, target) //nolint:errcheck // Intentional: verifying lock availability only

				c.logger.Info(ctx, "NETCONF lock acquired",
					"target", target)

				return nil
			}
			// Lock still held, continue waiting
		}
	}
}

// reconnect attempts to reconnect the NETCONF session
//
// This method closes the existing connection and establishes a new one,
// re-negotiating capabilities. Used when transport errors are detected.
//
// Returns an error if reconnection fails.
func (c *Client) reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Warn(context.Background(), "NETCONF reconnecting",
		"host", c.Host,
		"reason", "transport error")

	// Close existing connection (ignore errors - connection may already be broken)
	if c.driver != nil {
		_ = c.driver.Close() //nolint:errcheck // Explicitly ignore error (connection likely already broken)
		c.driver = nil
	}

	// Build scrapligo options (same as NewClient)
	scrapliOpts := []util.Option{
		options.WithAuthUsername(c.username),
		options.WithAuthPassword(c.password),
		options.WithPort(c.Port),
		options.WithTimeoutSocket(c.ConnectTimeout),
	}

	if c.InsecureSkipVerify {
		scrapliOpts = append(scrapliOpts, options.WithAuthNoStrictKey())
	}

	if c.SSHKeyPath != "" {
		scrapliOpts = append(scrapliOpts, options.WithAuthPrivateKey(c.SSHKeyPath, ""))
	}

	// Create new driver
	driver, err := netconf.NewDriver(c.Host, scrapliOpts...)
	if err != nil {
		c.logger.Error(context.Background(), "NETCONF reconnection failed",
			"host", c.Host,
			"error", err.Error())
		return fmt.Errorf("failed to create driver during reconnect: %w", err)
	}

	// Open connection
	err = driver.Open()
	if err != nil {
		c.logger.Error(context.Background(), "NETCONF reconnection failed",
			"host", c.Host,
			"error", err.Error())
		return fmt.Errorf("failed to open connection during reconnect: %w", err)
	}

	// Store new driver and capabilities
	c.driver = driver
	c.Capabilities = driver.ServerCapabilities()
	c.sessionID = fmt.Sprintf("%d", driver.SessionID())

	// Re-extract server version
	for _, cap := range c.Capabilities {
		if cap == netconfBase10URN ||
			cap == "urn:ietf:params:netconf:base:1.1" {
			c.serverVersion = driver.SelectedVersion
			break
		}
	}

	c.logger.Info(context.Background(), "NETCONF reconnected",
		"host", c.Host,
		"sessionID", c.sessionID)

	return nil
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
	// 3. Default operation timeout (fallback)
	if req.Timeout > 0 {
		// Request-specific timeout takes precedence
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	} else if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		// Only apply default if no deadline already set
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.OperationTimeout)
		defer cancel()
	}

	// Retry loop
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
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

		// Check if transient
		isTransient := c.checkTransientError(res.Errors)

		// Handle lock-denied errors with polling before general backoff
		if isTransient {
			// Check if this is a lock-denied error
			hasLockDenied := false
			var lockTarget string
			for _, rpcErr := range res.Errors {
				if rpcErr.ErrorTag == "lock-denied" || rpcErr.ErrorTag == "in-use" {
					hasLockDenied = true
					// Extract target from request
					lockTarget = req.Target
					break
				}
			}

			if hasLockDenied && lockTarget != "" {
				// Use lock-specific polling instead of exponential backoff
				if waitErr := c.waitForLockRelease(ctx, lockTarget); waitErr != nil {
					// Timeout waiting for lock, return error
					return res, &NetconfError{
						Operation:   req.Operation,
						Errors:      res.Errors,
						Message:     "lock wait timeout",
						InternalMsg: waitErr.Error(),
						Retries:     attempt,
						IsTransient: true,
					}
				}
				// Lock released, retry immediately without backoff
				continue
			}
		}

		// Check for transport errors (connection issues)
		hasTransportError := false
		for _, rpcErr := range res.Errors {
			if rpcErr.ErrorType == transportErrType {
				hasTransportError = true
				break
			}
		}

		// Not transient or max retries reached
		if !isTransient || attempt >= c.MaxRetries {
			// Log operation failure
			c.logger.Error(ctx, "NETCONF operation failed",
				"operation", req.Operation,
				"retries", attempt,
				"transient", isTransient,
				"errorCount", len(res.Errors))

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

		// Handle transport errors with reconnection
		if hasTransportError {
			// Attempt to reconnect
			if reconnectErr := c.reconnect(); reconnectErr != nil {
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
			// Reconnection succeeded, continue to backoff and retry
		}

		// Apply backoff before next retry
		delay := c.Backoff(attempt)
		c.logger.Debug(ctx, "NETCONF retry backoff",
			"operation", req.Operation,
			"attempt", attempt,
			"delay", delay.String())

		select {
		case <-ctx.Done():
			return Res{}, fmt.Errorf("operation canceled during backoff: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next retry
		}
	}

	// Should never reach here, but return error for safety
	return Res{}, fmt.Errorf("operation %s: exceeded maximum retries (%d)", req.Operation, c.MaxRetries)
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

	// Delegate to scrapligo driver based on operation
	var scrapligoRes *response.NetconfResponse
	var err error

	// Check for nil driver before operation
	if c.driver == nil {
		return Res{}, fmt.Errorf("operation %s failed: driver is nil (connection closed)", req.Operation)
	}

	switch req.Operation {
	case "get":
		// Get method signature: Get(filter string, opts ...util.Option)
		// The filter string is the XML subtree for subtree filters, or empty string
		// For XPath filters, the XPath expression is passed as the filter string
		// Note: scrapligo driver/netconf may require filter type to be indicated via options
		// for XPath filters (this is validated but needs integration testing)
		filterStr := req.Filter.Content
		scrapligoRes, err = c.driver.Get(filterStr)

	case "get-config":
		// GetConfig method signature: GetConfig(source string, opts ...util.Option)
		//
		// Filter options are provided via driver/opoptions package:
		//   - opoptions.WithFilter(content) sets the filter XML or XPath expression
		//   - opoptions.WithFilterType(type) sets "subtree" or "xpath" (default is "subtree")
		var opts []util.Option
		if req.Filter.Content != "" {
			// Add filter content
			opts = append(opts, opoptions.WithFilter(req.Filter.Content))

			// Set filter type if XPath (subtree is default)
			if req.Filter.Type == filterTypeXPath {
				opts = append(opts, opoptions.WithFilterType(filterTypeXPath))
			}
		}
		scrapligoRes, err = c.driver.GetConfig(req.Target, opts...)

	case "edit-config":
		// EditConfig method signature: EditConfig(target, config string)
		// If advanced edit-config options are set, build custom XML
		if req.DefaultOperation != "" || req.TestOption != "" || req.ErrorOption != "" {
			rpcXML := c.buildEditConfigXML(req)
			scrapligoRes, err = c.driver.RPC(opoptions.WithFilter(rpcXML))
		} else {
			scrapligoRes, err = c.driver.EditConfig(req.Target, req.Config)
		}

	case "copy-config":
		// CopyConfig method signature: CopyConfig(source, target string)
		scrapligoRes, err = c.driver.CopyConfig(req.Config, req.Target)

	case "delete-config":
		// DeleteConfig method signature: DeleteConfig(target string)
		scrapligoRes, err = c.driver.DeleteConfig(req.Target)

	case "lock":
		// Lock method signature: Lock(target string)
		scrapligoRes, err = c.driver.Lock(req.Target)

	case "unlock":
		// Unlock method signature: Unlock(target string)
		scrapligoRes, err = c.driver.Unlock(req.Target)

	case "commit":
		// Commit method signature: Commit(opts ...util.Option)
		// Support confirmed commit parameters via scrapligo options
		var opts []util.Option
		if req.ConfirmTimeout > 0 {
			// Confirmed commit with timeout
			opts = append(opts, opoptions.WithCommitConfirmed())
			opts = append(opts, opoptions.WithCommitConfirmTimeout(uint(req.ConfirmTimeout)))
		}
		if req.PersistID != "" {
			// Persist ID for commit operations
			opts = append(opts, opoptions.WithCommitConfirmedPersistID(req.PersistID))
		}
		scrapligoRes, err = c.driver.Commit(opts...)

	case "discard":
		// Discard method signature: Discard()
		scrapligoRes, err = c.driver.Discard()

	case "validate":
		// Validate method signature: Validate(target string)
		scrapligoRes, err = c.driver.Validate(req.Target)

	case "rpc":
		// RPC method signature: RPC(opts ...util.Option)
		// Pass the RPC XML content via WithFilter option
		scrapligoRes, err = c.driver.RPC(opoptions.WithFilter(req.Config))

	default:
		return Res{}, fmt.Errorf("unsupported operation: %s", req.Operation)
	}

	if err != nil {
		return Res{}, fmt.Errorf("operation %s failed: %w", req.Operation, err)
	}

	// Check for nil response
	if scrapligoRes == nil {
		return Res{}, fmt.Errorf("operation %s: received nil response from driver", req.Operation)
	}

	// Log request XML content (Debug level only)
	// Pre-check size and level to avoid expensive processing when not needed
	if len(scrapligoRes.Input) > 0 {
		// Pre-check size limit before string conversion (avoid allocation)
		if len(scrapligoRes.Input) <= MaxXMLSizeForLogging {
			// Validate UTF-8 encoding
			if !utf8.Valid(scrapligoRes.Input) {
				c.logger.Warn(ctx, "Invalid UTF-8 in NETCONF request XML",
					"operation", req.Operation,
					"size", len(scrapligoRes.Input))
			} else {
				requestXML := c.prepareXMLForLogging(string(scrapligoRes.Input))
				c.logger.Debug(ctx, "NETCONF RPC request XML",
					"operation", req.Operation,
					"xml", requestXML)
			}
		} else {
			// Log truncation message only (cheap operation)
			c.logger.Debug(ctx, "NETCONF RPC request XML (truncated)",
				"operation", req.Operation,
				"size", len(scrapligoRes.Input),
				"limit", MaxXMLSizeForLogging,
				"xml", "[XML TOO LARGE FOR LOGGING]")
		}
	}

	// Log response XML content (Debug level only)
	if scrapligoRes.Result != "" {
		// Pre-check size limit before processing
		if len(scrapligoRes.Result) <= MaxXMLSizeForLogging {
			// Validate UTF-8 encoding
			if !utf8.ValidString(scrapligoRes.Result) {
				c.logger.Warn(ctx, "Invalid UTF-8 in NETCONF response XML",
					"operation", req.Operation,
					"size", len(scrapligoRes.Result))
			} else {
				responseXML := c.prepareXMLForLogging(scrapligoRes.Result)
				c.logger.Debug(ctx, "NETCONF RPC response XML",
					"operation", req.Operation,
					"xml", responseXML)
			}
		} else {
			// Log truncation message only (cheap operation)
			c.logger.Debug(ctx, "NETCONF RPC response XML (truncated)",
				"operation", req.Operation,
				"size", len(scrapligoRes.Result),
				"limit", MaxXMLSizeForLogging,
				"xml", "[XML TOO LARGE FOR LOGGING]")
		}
	}

	// Parse response XML
	return c.parseResponse(scrapligoRes)
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

	// Parse XML with xmldot
	result := xmldot.Get(scrapligoRes.Result, "rpc-reply")

	// Check for <ok/> response
	okResult := xmldot.Get(scrapligoRes.Result, "rpc-reply.ok")
	ok := okResult.Exists()

	// Check for <rpc-error> elements
	errors := c.parseRPCErrors(scrapligoRes.Result)

	// Extract message-id
	msgID := xmldot.Get(scrapligoRes.Result, "rpc-reply@message-id").String()

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

	// Config data (raw XML content)
	xml, _ = xmldot.SetRaw(xml, "edit-config.config", req.Config) //nolint:errcheck // XML building errors caught during validation

	return xml
}
