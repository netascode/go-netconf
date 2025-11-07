# Error Handling Guide

This guide explains error types, retry strategies, and recovery patterns in go-netconf.

## Table of Contents

- [Error Types](#error-types)
- [Standard Errors](#standard-errors)
- [NETCONF Errors](#netconf-errors)
- [Transient Errors](#transient-errors)
- [Automatic Retry Logic](#automatic-retry-logic)
- [Error Handling Patterns](#error-handling-patterns)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Error Types

go-netconf provides three categories of errors:

1. **Standard Go Errors** - Simple error values for common failures
2. **NetconfError** - Structured errors with operation context and retry information
3. **NETCONF rpc-error Elements** - Device-reported errors from NETCONF protocol

## Standard Errors

The library defines standard error values for common failure conditions:

```go
var (
    ErrLockReleaseTimeout error = errors.New("netconf: lock release timeout")
)
```

Most errors are returned as `NetconfError` structs with detailed context. The standard `ErrLockReleaseTimeout` is returned when waiting for a lock release exceeds the configured timeout.

**Example - Checking Standard Errors:**
```go
import "errors"

client, err := netconf.NewClient("192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.LockReleaseTimeout(2*time.Minute))
if err != nil {
    log.Fatalf("Connection failed: %v", err)
}

// Try to acquire lock
_, err = client.Lock(ctx, "candidate")
if err != nil {
    if errors.Is(err, netconf.ErrLockReleaseTimeout) {
        log.Fatal("Timed out waiting for lock release")
    }
    log.Fatalf("Lock failed: %v", err)
}
```

## NETCONF Errors

### NetconfError Structure

Structured error type with operation context:

```go
type NetconfError struct {
    Operation   string        // Operation that failed ("get", "edit-config", etc.)
    Errors      []ErrorModel  // NETCONF rpc-error elements
    Message     string        // Human-readable error message
    InternalMsg string        // Detailed internal error (for secure logging)
    Retries     int           // Number of retry attempts made
    IsTransient bool          // True if error was identified as transient
}
```

**Example - Handling NetconfError:**
```go
ctx := context.Background()
res, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    // Type assert to NetconfError
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        log.Printf("Operation: %s", netconfErr.Operation)
        log.Printf("Retries: %d", netconfErr.Retries)
        log.Printf("Transient: %v", netconfErr.IsTransient)

        // Check NETCONF errors
        for _, rpcErr := range netconfErr.Errors {
            log.Printf("Error: [%s] %s - %s",
                rpcErr.ErrorType,
                rpcErr.ErrorTag,
                rpcErr.ErrorMessage)
        }
    }
    return err
}
```

### ErrorModel Structure

NETCONF rpc-error elements per RFC 6241:

```go
type ErrorModel struct {
    ErrorType     string  // transport, rpc, protocol, application
    ErrorTag      string  // in-use, invalid-value, operation-failed, etc.
    ErrorSeverity string  // error, warning
    ErrorAppTag   string  // Application-specific error tag
    ErrorPath     string  // XPath to element in error
    ErrorMessage  string  // Human-readable message
    ErrorInfo     string  // Additional error information
}
```

**Common Error Tags:**

| Error Tag | Meaning | Typical Cause |
|-----------|---------|---------------|
| in-use | Resource is in use | Datastore locked by another session |
| invalid-value | Invalid value for element | Configuration validation failed |
| too-big | Request or response too large | Data size exceeds limits |
| missing-attribute | Required attribute missing | Incomplete configuration |
| bad-attribute | Invalid attribute value | Attribute validation failed |
| unknown-element | Element not recognized | Unknown configuration element |
| missing-element | Required element missing | Incomplete configuration |
| bad-element | Invalid element value | Element validation failed |
| unknown-namespace | Namespace not recognized | Invalid xmlns declaration |
| access-denied | Access denied | Insufficient permissions |
| lock-denied | Lock request denied | Lock held by another session |
| resource-denied | Resource unavailable | System resource exhaustion |
| rollback-failed | Rollback failed | Cannot revert changes |
| data-exists | Data already exists | Create operation on existing data |
| data-missing | Data not found | Delete/replace operation on missing data |
| operation-not-supported | Operation not supported | Unsupported NETCONF operation |
| operation-failed | General operation failure | Unspecified operation error |

**Example - Checking Error Tags:**
```go
res, err := client.Lock(ctx, "candidate")
if err != nil {
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        for _, rpcErr := range netconfErr.Errors {
            if rpcErr.ErrorTag == "lock-denied" {
                log.Println("Datastore is locked by another session")
                log.Printf("Error info: %s", rpcErr.ErrorInfo)
                // Implement lock wait or retry logic
            }
        }
    }
}
```

## Transient Errors

Transient errors are temporary conditions that may resolve automatically. The library identifies these patterns and retries automatically:

### Transient Error Patterns

The library automatically retries only confirmed transient error patterns from RFC 6241:

```go
var TransientErrors = []TransientError{
    // Lock conflicts (RFC 6241 Section 4.3)
    {ErrorTag: "lock-denied"},
    {ErrorTag: "in-use"},

    // Transport errors (RFC 6241 Section 4.3)
    {ErrorType: "transport"},
}
```

**Note:** Only includes patterns confirmed by RFC 6241 and observed device behavior. Additional patterns may be added as they are confirmed with real devices.

### Lock Conflicts

When a datastore is locked by another session:

**Error Pattern:**
```go
ErrorType: "application"
ErrorTag: "lock-denied"
ErrorMessage: "Configuration database locked by session 12345"
```

**Library Behavior:**
- Automatically waits for lock release (configurable timeout)
- Polls lock status at 1-second intervals
- Retries lock operation when released

**Configuration:**
```go
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.LockReleaseTimeout(120*time.Second),  // Wait up to 2 minutes
)
```

### Transport Errors

Connection or transport-level failures:

**Error Pattern:**
```go
ErrorType: "transport"
ErrorMessage: "Connection reset by peer"
```

**Library Behavior:**
- Automatic session reconnection
- Capability re-negotiation
- Operation retry with new session

## Automatic Retry Logic

### Exponential Backoff

The library uses exponential backoff with jitter for retries:

**Formula:**
```
delay = min(minDelay * (factor ^ attempt) + jitter, maxDelay)
jitter = random(0, delay * 0.1)
```

**Default Configuration:**
```go
MaxRetries:         10
BackoffMinDelay:    1 * time.Second
BackoffMaxDelay:    60 * time.Second
BackoffDelayFactor: 1.2
```

**Custom Configuration:**
```go
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.MaxRetries(5),                       // Retry up to 5 times
    netconf.BackoffMinDelay(2*time.Second),      // Start with 2 seconds
    netconf.BackoffMaxDelay(30*time.Second),     // Cap at 30 seconds
    netconf.BackoffDelayFactor(1.5),             // 1.5x increase per retry
)
```

**Backoff Sequence Example:**
```
Attempt 0: 0s (immediate)
Attempt 1: 2s + jitter
Attempt 2: 3s + jitter  (2 * 1.5)
Attempt 3: 4.5s + jitter  (3 * 1.5)
Attempt 4: 6.75s + jitter  (4.5 * 1.5)
Attempt 5: 10.1s + jitter  (6.75 * 1.5)
```

### Retry Behavior

**Transient Errors:**
- Automatically retried with exponential backoff
- Retry count tracked in NetconfError
- Max retries configurable

**Non-Transient Errors:**
- Fail immediately without retry
- Return error to caller

**Example - Checking Retry Information:**
```go
res, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        if netconfErr.Retries > 0 {
            log.Printf("Operation failed after %d retries", netconfErr.Retries)
            log.Printf("Was transient: %v", netconfErr.IsTransient)
        } else {
            log.Println("Operation failed immediately (non-transient error)")
        }
    }
}
```

## Error Handling Patterns

### Pattern 1: Basic Error Handling

Simple error checking:

```go
ctx := context.Background()
res, err := client.GetConfig(ctx, "running", filter)
if err != nil {
    log.Fatalf("GetConfig failed: %v", err)
}

// Process result
data := res.Res.Get("data").String()
```

### Pattern 2: Structured Error Handling

Handle different error types:

```go
res, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    // Check standard errors
    if errors.Is(err, netconf.ErrLockDenied) {
        log.Println("Datastore is locked, waiting...")
        time.Sleep(5 * time.Second)
        // Retry
        return client.EditConfig(ctx, "candidate", config)
    }

    // Check NetconfError
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        log.Printf("Operation %s failed", netconfErr.Operation)

        // Check specific error tags
        for _, rpcErr := range netconfErr.Errors {
            switch rpcErr.ErrorTag {
            case "invalid-value":
                log.Printf("Invalid configuration value: %s", rpcErr.ErrorMessage)
                return fmt.Errorf("config validation failed: %w", err)
            case "access-denied":
                log.Printf("Access denied: %s", rpcErr.ErrorMessage)
                return fmt.Errorf("insufficient permissions: %w", err)
            default:
                log.Printf("Error: %s - %s", rpcErr.ErrorTag, rpcErr.ErrorMessage)
            }
        }
    }

    return err
}
```

### Pattern 3: Retry with Custom Logic

Implement custom retry for specific operations:

```go
func editConfigWithRetry(ctx context.Context, client *netconf.Client,
                         target, config string, maxRetries int) error {
    var lastErr error

    for attempt := 0; attempt <= maxRetries; attempt++ {
        res, err := client.EditConfig(ctx, target, config)
        if err == nil && res.OK {
            return nil  // Success
        }

        lastErr = err

        // Check if we should retry
        if !shouldRetry(err) {
            return err  // Non-retryable error
        }

        if attempt < maxRetries {
            // Exponential backoff
            delay := time.Duration(math.Pow(2, float64(attempt))) * time.Second
            log.Printf("Attempt %d failed, retrying in %v...", attempt+1, delay)
            time.Sleep(delay)
        }
    }

    return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

func shouldRetry(err error) bool {
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        // Retry on transient errors
        if netconfErr.IsTransient {
            return true
        }

        // Check for lock-denied errors (automatically retried)
        for _, rpcErr := range netconfErr.Errors {
            if rpcErr.ErrorTag == "lock-denied" || rpcErr.ErrorTag == "in-use" {
                return true
            }
        }
    }
    return false
}
```

### Pattern 4: Error Recovery with Discard

Recover from validation errors with proper cleanup:

```go
func applyConfigSafe(ctx context.Context, client *netconf.Client, config string) error {
    // Lock candidate
    if _, err := client.Lock(ctx, "candidate"); err != nil {
        return fmt.Errorf("lock failed: %w", err)
    }
    // CRITICAL: Always use defer with error handling
    defer func() {
        if _, err := client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("Warning: Failed to unlock: %v", err)
        }
    }()

    // Edit configuration
    _, err := client.EditConfig(ctx, "candidate", config)
    if err != nil {
        // Discard changes on error
        if _, discardErr := client.Discard(ctx); discardErr != nil {
            log.Printf("Discard failed: %v", discardErr)
        }
        return fmt.Errorf("edit-config failed: %w", err)
    }

    // Validate
    _, err = client.Validate(ctx, "candidate")
    if err != nil {
        // Discard invalid configuration
        if _, discardErr := client.Discard(ctx); discardErr != nil {
            log.Printf("Discard failed: %v", discardErr)
        }
        return fmt.Errorf("validation failed: %w", err)
    }

    // Commit
    _, err = client.Commit(ctx)
    if err != nil {
        return fmt.Errorf("commit failed: %w", err)
    }

    return nil
}
```

### Pattern 5: Timeout Handling

Handle context timeouts separately:

```go
func getConfigWithTimeout(ctx context.Context, client *netconf.Client) error {
    // Create context with timeout
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()

    res, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        // Check if timeout occurred
        if errors.Is(err, context.DeadlineExceeded) {
            log.Println("Operation timed out after 30 seconds")
            return fmt.Errorf("timeout: %w", err)
        }

        // Other errors
        return err
    }

    // Process result
    _ = res
    return nil
}
```

### Pattern 6: Graceful Degradation

Fallback to simpler operations on error:

```go
func getInterfaces(ctx context.Context, client *netconf.Client) (netconf.Res, error) {
    // Try with XPath filter first
    if client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
        filter := netconf.XPathFilter("/interfaces/interface[enabled='true']")
        res, err := client.Get(ctx, filter)
        if err == nil {
            return res, nil
        }

        log.Printf("XPath filter failed: %v, trying subtree filter", err)
    }

    // Fallback to subtree filter
    filter := netconf.SubtreeFilter("<interfaces/>")
    res, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        return netconf.Res{}, fmt.Errorf("all filter attempts failed: %w", err)
    }

    // Filter enabled interfaces in application code
    return res, nil
}
```

## Best Practices

### 1. Always Check Errors

Never ignore errors:

```go
// Bad
client.EditConfig(ctx, "candidate", config)

// Good
_, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    return err
}
```

### 2. Use Defer for Cleanup

**CRITICAL**: Always use defer to ensure resources are released, even on panic or early return:

```go
// Lock datastore
if _, err := client.Lock(ctx, "candidate"); err != nil {
    return err
}
// CRITICAL: Use defer with error handling
defer func() {
    if _, err := client.Unlock(ctx, "candidate"); err != nil {
        log.Printf("Warning: Failed to unlock: %v", err)
    }
}()

// Perform operations safely - lock will be released even if panic occurs
```

**Why This Matters:**

Without defer, if your code panics or returns early due to an error, the lock remains held until the session times out (typically 30 minutes). This blocks all other NETCONF sessions from acquiring the lock, creating a denial-of-service condition.

### 3. Log Errors Appropriately

Use structured logging:

```go
if err != nil {
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        // Use DetailedError() for internal logs only
        log.Debug(netconfErr.DetailedError())

        // Use Error() for user-facing logs
        log.Error(netconfErr.Error())
    }
}
```

### 4. Handle Transient vs Permanent Errors

Differentiate error handling:

```go
if netconfErr, ok := err.(*netconf.NetconfError); ok {
    if netconfErr.IsTransient {
        // Transient error - library already retried
        log.Println("Operation failed after automatic retries")
        // Consider manual retry or alerting
    } else {
        // Permanent error - fix required
        log.Println("Permanent error - configuration or permissions issue")
        // Fix configuration or permissions
    }
}
```

### 5. Set Appropriate Timeouts

Configure timeouts based on operation:

```go
// Quick operations
client, _ := netconf.NewClient("192.168.1.1",
    netconf.TotalTimeout(1*time.Minute))

// Long-running operations
res, err := client.Commit(ctx, netconf.Timeout(5*time.Minute))
```

### 6. Monitor Retry Counts

Track retry patterns:

```go
if netconfErr, ok := err.(*netconf.NetconfError); ok {
    if netconfErr.Retries > 3 {
        // High retry count indicates recurring issue
        metrics.RecordHighRetryCount(netconfErr.Operation, netconfErr.Retries)
        alert.Send("High retry count for NETCONF operation")
    }
}
```

### 7. Validate Before Operations

Prevent errors proactively:

```go
// Check capabilities before operations
if !client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
    return fmt.Errorf("device doesn't support candidate datastore")
}

// Validate configuration before applying
body := netconf.Body{}.Set("config.system.hostname", hostname)
config, err := body.String()
if err != nil {
    return fmt.Errorf("invalid configuration: %w", err)
}

// Use the validated config
_, err = client.EditConfig(ctx, "candidate", config)
```

## Troubleshooting

### Problem: ErrAuthenticationFailed

**Symptoms:** Client creation fails with authentication error

**Causes:**
- Incorrect username or password
- SSH key not found or invalid
- Device requires specific authentication method

**Solutions:**
```go
// Verify credentials
client, err := netconf.NewClient("192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("correct-password"))

// Use SSH key authentication
client, err := netconf.NewClient("192.168.1.1",
    netconf.Username("admin"),
    netconf.SSHKey("/path/to/private/key"))
```

### Problem: ErrOperationTimeout

**Symptoms:** Operations time out before completing

**Causes:**
- Operation takes longer than timeout
- Network latency
- Device is slow to respond
- Large data transfer

**Solutions:**
```go
// Increase timeouts
client, _ := netconf.NewClient("192.168.1.1",
    netconf.AttemptTimeout(60*time.Second),   // Per-attempt timeout
    netconf.TotalTimeout(5*time.Minute))      // Total timeout across retries

// Increase per-operation timeout
res, err := client.Get(ctx, filter, netconf.Timeout(5*time.Minute))

// Use more specific filters to reduce data
filter := netconf.SubtreeFilter("<system><hostname/></system>")
```

### Problem: lock-denied Errors

**Symptoms:** Lock operations fail with "lock-denied"

**Causes:**
- Another session holds the lock
- Previous session didn't unlock
- Concurrent lock attempts

**Solutions:**
```go
// Increase lock wait timeout
client, _ := netconf.NewClient("192.168.1.1",
    netconf.LockReleaseTimeout(300*time.Second))  // 5 minutes

// Check lock status before operations
res, err := client.Lock(ctx, "candidate")
if err != nil {
    log.Println("Waiting for lock...")
    // Library automatically waits and retries
}

// Always unlock with defer and error handling
defer func() {
    if _, err := client.Unlock(ctx, "candidate"); err != nil {
        log.Printf("Warning: unlock failed: %v", err)
    }
}()
```

### Problem: High Retry Counts

**Symptoms:** Operations succeed but with many retries

**Causes:**
- Frequent transient errors
- Resource contention
- Network instability

**Solutions:**
```go
// Monitor retry counts
if netconfErr, ok := err.(*netconf.NetconfError); ok {
    if netconfErr.Retries > 5 {
        log.Printf("High retry count: %d for %s",
            netconfErr.Retries, netconfErr.Operation)
        // Investigate root cause
    }
}

// Adjust backoff parameters
client, _ := netconf.NewClient("192.168.1.1",
    netconf.BackoffMinDelay(5*time.Second),     // Start with longer delay
    netconf.BackoffMaxDelay(120*time.Second))   // Allow longer max delay
```

### Problem: Validation Errors

**Symptoms:** EditConfig or Commit fails with validation errors

**Causes:**
- Invalid configuration values
- Missing required elements
- Constraint violations

**Solutions:**
```go
// Use Validate before Commit
_, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    return err
}

_, err = client.Validate(ctx, "candidate")
if err != nil {
    // Extract validation errors
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        for _, rpcErr := range netconfErr.Errors {
            log.Printf("Validation error: %s at %s",
                rpcErr.ErrorMessage, rpcErr.ErrorPath)
        }
    }

    // Discard invalid config
    client.Discard(ctx)
    return err
}

// Safe to commit
client.Commit(ctx)
```

## Complete Production Example

This example demonstrates comprehensive error handling in a production environment:

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "time"

    "github.com/netascode/go-netconf"
)

func main() {
    // Create client with production error handling configuration
    client, err := netconf.NewClient(
        "192.168.1.1",
        netconf.Username("automation"),
        netconf.SSHKey("/path/to/key"),
        netconf.MaxRetries(5),
        netconf.AttemptTimeout(60*time.Second),
        netconf.TotalTimeout(5*time.Minute),
        netconf.LockReleaseTimeout(3*time.Minute),
        netconf.BackoffMinDelay(2*time.Second),
        netconf.BackoffMaxDelay(60*time.Second),
    )
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    ctx := context.Background()

    // Apply configuration with comprehensive error handling
    if err := applyConfigurationWithErrorHandling(ctx, client); err != nil {
        handleFatalError(err)
    }

    log.Println("Configuration applied successfully")
}

func applyConfigurationWithErrorHandling(ctx context.Context, client *netconf.Client) error {
    // Verify capabilities
    if !client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
        return fmt.Errorf("device missing candidate datastore capability")
    }

    // Build configuration
    body := netconf.Body{}.
        Set("config.system.hostname", "router-01").
        Set("config.system.domain-name", "example.com")

    config, err := body.String()
    if err != nil {
        return fmt.Errorf("failed to build configuration: %w", err)
    }

    // Lock with automatic retry
    log.Println("Acquiring lock...")
    if _, err := client.Lock(ctx, "candidate"); err != nil {
        if errors.Is(err, netconf.ErrLockReleaseTimeout) {
            return fmt.Errorf("lock timeout - another session may be stuck: %w", err)
        }
        return fmt.Errorf("failed to acquire lock: %w", err)
    }

    // CRITICAL: Ensure unlock with defer
    defer func() {
        log.Println("Releasing lock...")
        if _, err := client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("ERROR: Failed to unlock candidate: %v", err)
        }
    }()

    // Edit configuration
    log.Println("Editing configuration...")
    if _, err := client.EditConfig(ctx, "candidate", config); err != nil {
        handleEditConfigError(ctx, client, err)
        return err
    }

    // Validate configuration
    log.Println("Validating configuration...")
    if _, err := client.Validate(ctx, "candidate"); err != nil {
        handleValidationError(ctx, client, err)
        return err
    }

    // Commit changes
    log.Println("Committing changes...")
    if _, err := client.Commit(ctx); err != nil {
        handleCommitError(ctx, client, err)
        return err
    }

    return nil
}

func handleEditConfigError(ctx context.Context, client *netconf.Client, err error) {
    log.Printf("EditConfig failed: %v", err)

    // Discard changes
    log.Println("Discarding invalid configuration...")
    if _, discardErr := client.Discard(ctx); discardErr != nil {
        log.Printf("ERROR: Failed to discard changes: %v", discardErr)
    }

    // Extract detailed error information
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        log.Printf("Operation: %s, Retries: %d, Transient: %v",
            netconfErr.Operation, netconfErr.Retries, netconfErr.IsTransient)

        for _, rpcErr := range netconfErr.Errors {
            log.Printf("NETCONF Error: [%s] %s - %s",
                rpcErr.ErrorType, rpcErr.ErrorTag, rpcErr.ErrorMessage)
            if rpcErr.ErrorPath != "" {
                log.Printf("Error Path: %s", rpcErr.ErrorPath)
            }
        }

        // Check for high retry count
        if netconfErr.Retries > 3 {
            log.Printf("WARNING: High retry count (%d) indicates recurring issue",
                netconfErr.Retries)
        }
    }
}

func handleValidationError(ctx context.Context, client *netconf.Client, err error) {
    log.Printf("Validation failed: %v", err)

    // Discard invalid configuration
    log.Println("Discarding invalid configuration...")
    if _, discardErr := client.Discard(ctx); discardErr != nil {
        log.Printf("ERROR: Failed to discard changes: %v", discardErr)
    }

    // Extract validation error details
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        for _, rpcErr := range netconfErr.Errors {
            log.Printf("Validation Error: %s at %s",
                rpcErr.ErrorMessage, rpcErr.ErrorPath)

            // Specific error tag handling
            switch rpcErr.ErrorTag {
            case "invalid-value":
                log.Printf("Configuration contains invalid value: %s", rpcErr.ErrorMessage)
            case "missing-element":
                log.Printf("Configuration missing required element: %s", rpcErr.ErrorMessage)
            case "unknown-namespace":
                log.Printf("Invalid namespace in configuration: %s", rpcErr.ErrorMessage)
            }
        }
    }
}

func handleCommitError(ctx context.Context, client *netconf.Client, err error) {
    log.Printf("Commit failed: %v", err)

    // For commit errors, configuration is automatically rolled back
    // Log detailed information for troubleshooting
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        log.Printf("Commit failed after %d retries", netconfErr.Retries)

        for _, rpcErr := range netconfErr.Errors {
            log.Printf("Commit Error: [%s] %s - %s",
                rpcErr.ErrorType, rpcErr.ErrorTag, rpcErr.ErrorMessage)

            // Log additional error info if available
            if rpcErr.ErrorInfo != "" {
                log.Printf("Additional Info: %s", rpcErr.ErrorInfo)
            }
        }
    }

    // Note: Candidate datastore is automatically reverted on commit failure
    log.Println("Configuration automatically rolled back due to commit failure")
}

func handleFatalError(err error) {
    log.Printf("FATAL ERROR: Configuration failed: %v", err)

    // Send alerts to monitoring system
    // sendAlert("NETCONF Configuration Failed", err.Error())

    // Log for post-mortem analysis
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        // Use DetailedError for internal logs
        log.Printf("Detailed Error: %s", netconfErr.DetailedError())
    }

    // Exit with error code
    log.Fatal("Configuration operation failed")
}
```

## Error Handling Summary

### Quick Reference

| Error Type | Automatic Retry | Discard Required | Lock Release Required |
|-----------|----------------|------------------|----------------------|
| `lock-denied` | ✅ Yes (with polling) | ❌ No | ❌ No (never acquired) |
| `in-use` | ✅ Yes (with polling) | ❌ No | ❌ No (never acquired) |
| `transport` | ✅ Yes (with reconnect) | ❌ No | ✅ Yes (if lock held) |
| `invalid-value` | ❌ No | ✅ Yes | ✅ Yes (if locked) |
| `missing-element` | ❌ No | ✅ Yes | ✅ Yes (if locked) |
| `access-denied` | ❌ No | ✅ Yes | ✅ Yes (if locked) |
| `resource-denied` | ❌ No | ✅ Yes | ✅ Yes (if locked) |
| `operation-failed` | ❌ No | ✅ Yes (if edit-config) | ✅ Yes (if locked) |

### Decision Tree

```
Error Occurred
    ↓
Is it NetconfError?
    ↓ Yes
    ├─ IsTransient == true?
    │   ↓ Yes
    │   └─ Already retried by library → Check retry count → Log if high
    │
    └─ Check ErrorTag
        ├─ lock-denied → Check LockReleaseTimeout configuration
        ├─ invalid-value → Discard + Fix configuration
        ├─ access-denied → Check permissions
        ├─ transport → Check network/session
        └─ Other → Log details + Handle based on operation
```

### Best Practices Checklist

- ✅ Always check errors (never ignore return values)
- ✅ Use defer for lock cleanup with error handling
- ✅ Call Discard() on validation/edit failures
- ✅ Check Body.String() errors before using config
- ✅ Verify capabilities before operations
- ✅ Set appropriate timeouts for operation complexity
- ✅ Log NetconfError.DetailedError() for debugging (internal logs only)
- ✅ Monitor retry counts for recurring issues
- ✅ Use context.WithTimeout for operation-level timeouts
- ✅ Handle ErrLockReleaseTimeout separately

## See Also

- [Operations Guide](operations.md) - Detailed operation documentation
- [Quick Start Guide](quickstart.md) - Basic error handling patterns
- [Concurrency Guide](concurrency.md) - Thread-safe error handling
- [Logging Guide](logging.md) - Structured logging configuration
- [RFC 6241 Appendix A](https://tools.ietf.org/html/rfc6241#appendix-A) - NETCONF error types
- [RFC 6241 Section 4.3](https://tools.ietf.org/html/rfc6241#section-4.3) - RPC error handling
