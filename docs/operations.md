# NETCONF Operations Reference

This guide provides detailed documentation for all NETCONF operations supported by go-netconf, with examples and best practices.

## Table of Contents

- [Get](#get)
- [GetConfig](#getconfig)
- [EditConfig](#editconfig)
- [CopyConfig](#copyconfig)
- [DeleteConfig](#deleteconfig)
- [Lock](#lock)
- [Unlock](#unlock)
- [Commit](#commit)
- [Discard](#discard)
- [Validate](#validate)
- [RPC](#rpc)
- [Operation Modifiers](#operation-modifiers)
- [Response Handling](#response-handling)
- [Error Handling & Retry Logic](#error-handling--retry-logic)
- [Common NETCONF Capabilities](#common-netconf-capabilities)
- [Complete Workflow Examples](#complete-workflow-examples)

## Get

Retrieves configuration and state data from the device.

**Signature:**
```go
func (c *Client) Get(ctx context.Context, filter Filter, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `filter`: Subtree filter, XPath filter, or NoFilter() for all data
- `mods`: Optional request modifiers (Timeout, MessageID, etc.)

**Returns:**
- `Res`: Response containing the retrieved data
- `error`: Error if operation fails

**Example - Get with Subtree Filter:**
```go
ctx := context.Background()
filter := netconf.SubtreeFilter("<interfaces/>")
res, err := client.Get(ctx, filter)
if err != nil {
    log.Fatal(err)
}

// Extract interface name
ifName := res.Res.Get("data.interfaces.interface.name").String()
fmt.Println("Interface:", ifName)
```

**Example - Get with XPath Filter:**
```go
filter := netconf.XPathFilter("/interfaces/interface[name='GigabitEthernet1']")
res, err := client.Get(ctx, filter)
```

**Example - Get All Data:**
```go
res, err := client.Get(ctx, netconf.NoFilter())
```

**Example - Get with Custom Timeout:**
```go
res, err := client.Get(ctx, filter, netconf.Timeout(30*time.Second))
```

**Best Practices:**
- Use specific filters to reduce data transfer and processing time
- XPath filters are more powerful but require device support
- For large configurations, use specific subtree filters
- Consider pagination for very large datasets

**Common Errors:**
- `invalid XPath filter`: XPath syntax error or unsupported function
- `invalid subtree filter`: Malformed XML in subtree
- `operation timed out`: Increase timeout or reduce filter scope

## GetConfig

Retrieves configuration data from a specific datastore.

**Signature:**
```go
func (c *Client) GetConfig(ctx context.Context, source string, filter Filter, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `source`: Datastore name ("running", "candidate", "startup")
- `filter`: Subtree filter, XPath filter, or NoFilter()
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response containing configuration data
- `error`: Error if operation fails

**Example - Get Running Configuration:**
```go
ctx := context.Background()
filter := netconf.SubtreeFilter("<system><hostname/></system>")
res, err := client.GetConfig(ctx, "running", filter)
if err != nil {
    log.Fatal(err)
}

hostname := res.Res.Get("data.system.hostname").String()
fmt.Println("Hostname:", hostname)
```

**Example - Get Candidate Configuration:**
```go
res, err := client.GetConfig(ctx, "candidate", netconf.NoFilter())
```

**Example - Get Startup Configuration:**
```go
res, err := client.GetConfig(ctx, "startup", filter)
```

**Best Practices:**
- Use GetConfig instead of Get when you only need configuration (not state)
- Check device capabilities to verify candidate/startup datastore support
- Filter to specific sections to improve performance

**Common Errors:**
- `invalid source datastore`: Invalid datastore name
- `required capability not supported`: Device doesn't support specified datastore

## EditConfig

Modifies configuration in the specified datastore.

**Signature:**
```go
func (c *Client) EditConfig(ctx context.Context, target, config string, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `target`: Target datastore ("candidate" or "running")
- `config`: XML configuration to apply
- `mods`: Optional modifiers (DefaultOperation, TestOption, ErrorOption)

**Returns:**
- `Res`: Response indicating success or failure
- `error`: Error if operation fails

**Example - Basic Edit:**
```go
ctx := context.Background()

// Build configuration using Body builder
body := netconf.Body{}.
    Set("config.system.hostname", "router1").
    Set("config.system.domain-name", "example.com")

config, err := body.String()
if err != nil {
    log.Fatal(err)
}

res, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    log.Fatal(err)
}
```

**Example - Edit with Merge Operation:**
```go
body := netconf.Body{}.
    Set("config.interfaces.interface.name", "GigabitEthernet1").
    Set("config.interfaces.interface.description", "WAN Interface")

config, err := body.String()
if err != nil {
    log.Fatal(err)
}

res, err := client.EditConfig(ctx, "candidate", config,
    netconf.DefaultOperation("merge"))
if err != nil {
    log.Fatal(err)
}
```

**Example - Edit with Replace Operation:**
```go
body := netconf.Body{}.
    Set("config.interfaces.interface.name", "GigabitEthernet1").
    Set("config.interfaces.interface.mtu", 9000)

config, err := body.String()
if err != nil {
    log.Fatal(err)
}

res, err := client.EditConfig(ctx, "candidate", config,
    netconf.DefaultOperation("replace"))
if err != nil {
    log.Fatal(err)
}
```

**Example - Edit with Test-Then-Set:**
```go
body := netconf.Body{}.
    Set("config.system.hostname", "router1")

config, err := body.String()
if err != nil {
    log.Fatal(err)
}

res, err := client.EditConfig(ctx, "candidate", config,
    netconf.DefaultOperation("merge"),
    netconf.TestOption("test-then-set"))
if err != nil {
    log.Fatal(err)
}
```

**Example - Edit with Rollback-On-Error:**
```go
body := netconf.Body{}.
    Set("config.system.hostname", "router1").
    Set("config.system.location", "DataCenter1")

config, err := body.String()
if err != nil {
    log.Fatal(err)
}

res, err := client.EditConfig(ctx, "candidate", config,
    netconf.ErrorOption("rollback-on-error"))
if err != nil {
    log.Fatal(err)
}
```

**Default Operations:**
- `merge`: Merge new configuration with existing (default)
- `replace`: Replace existing configuration
- `none`: No default operation (must specify per element)

**Test Options:**
- `test-then-set`: Validate before applying (default)
- `set`: Apply without testing
- `test-only`: Validate only, don't apply

**Error Options:**
- `stop-on-error`: Stop at first error (default)
- `continue-on-error`: Continue processing despite errors
- `rollback-on-error`: Roll back all changes on error

**Best Practices:**
- Always use candidate datastore for complex changes
- Use test-then-set to validate before applying
- Use rollback-on-error for atomic transactions
- Build configuration with Body builder for safety

**Common Errors:**
- `invalid configuration`: Malformed XML or validation error
- `invalid target datastore`: Target not supported
- `lock-denied`: Datastore is locked by another session

## CopyConfig

Copies configuration between datastores or from/to URLs.

**Signature:**
```go
func (c *Client) CopyConfig(ctx context.Context, source, target string, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `source`: Source datastore ("running", "candidate", "startup") or URL
- `target`: Target datastore ("running", "candidate", "startup") or URL
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response indicating success
- `error`: Error if operation fails

**Example - Copy Running to Startup:**
```go
ctx := context.Background()
res, err := client.CopyConfig(ctx, "running", "startup")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Configuration saved to startup")
```

**Example - Copy Candidate to Running:**
```go
res, err := client.CopyConfig(ctx, "candidate", "running")
```

**Example - Copy from URL:**
```go
res, err := client.CopyConfig(ctx, "https://config-server.example.com/device1.xml", "candidate")
```

**Best Practices:**
- Lock target datastore before copying to prevent concurrent modifications
- Verify source datastore exists and is valid before copying
- Always backup before overwriting running config
- Use startup datastore for persistent configuration across reboots

**Common Errors:**
- `invalid source/target`: Invalid datastore name or URL
- `lock-denied`: Target datastore is locked

## DeleteConfig

Deletes a configuration datastore (startup only).

**Signature:**
```go
func (c *Client) DeleteConfig(ctx context.Context, target string, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `target`: Target datastore (must be "startup")
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response indicating success
- `error`: Error if operation fails

**Example - Delete Startup Configuration:**
```go
ctx := context.Background()
res, err := client.DeleteConfig(ctx, "startup")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Startup configuration deleted")
```

**Constraints:**
- Only "startup" datastore can be deleted per RFC 6241
- Cannot delete "running" or "candidate" datastores

**Best Practices:**
- Backup before deleting
- Confirm device supports startup datastore deletion
- Understand device behavior after startup deletion

**Common Errors:**
- `only 'startup' datastore can be deleted`: Attempted to delete running/candidate

## Lock

Locks a datastore to prevent other sessions from modifying it.

**Signature:**
```go
func (c *Client) Lock(ctx context.Context, target string, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `target`: Datastore to lock ("running", "candidate", "startup")
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response indicating lock success
- `error`: Error if lock fails

**Example - Lock Candidate:**
```go
ctx := context.Background()
res, err := client.Lock(ctx, "candidate")
if err != nil {
    log.Fatal(err)
}
defer client.Unlock(ctx, "candidate")  // CRITICAL: Always unlock

// Perform configuration changes
client.EditConfig(ctx, "candidate", config)
client.Commit(ctx)
```

**Example - Lock Running:**
```go
res, err := client.Lock(ctx, "running")
if err != nil {
    log.Fatal(err)
}
defer client.Unlock(ctx, "running")
```

**Best Practices:**
- **CRITICAL: Always use defer to ensure unlock** - Failure to unlock causes deadlocks that prevent all other sessions from operating
- Lock for the shortest time necessary to minimize contention
- The library automatically handles lock-denied errors with intelligent polling (configurable via `LockReleaseTimeout`)
- Use candidate datastore to minimize running datastore lock duration
- Never unlock datastores you didn't lock (causes operation-failed errors)

**Lock Failure Scenarios:**

If your program crashes or exits without unlocking, other sessions (including your own) will be unable to acquire the lock until:
- The device's session timeout expires (device-specific, typically 10-30 minutes)
- An administrator manually clears the lock
- The device is rebooted

**This is why defer is CRITICAL** - it ensures unlocking even if panics occur.

**Common Errors:**
- `lock-denied`: Another session holds the lock
- `in-use`: Target resource is currently in use

**Lock Conflict Handling:**

The library automatically waits for lock release with configurable timeout:

```go
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.LockReleaseTimeout(120*time.Second),  // Wait up to 2 minutes
)
```

## Unlock

Unlocks a previously locked datastore.

**Signature:**
```go
func (c *Client) Unlock(ctx context.Context, target string, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `target`: Datastore to unlock ("running", "candidate", "startup")
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response indicating unlock success
- `error`: Error if unlock fails

**Example - Standard Unlock Pattern:**
```go
res, err := client.Lock(ctx, "candidate")
if err != nil {
    log.Fatal(err)
}
defer client.Unlock(ctx, "candidate")  // Proper defer pattern
```

**Best Practices:**
- Always pair Lock with Unlock via defer
- Unlock even if operations fail
- Don't unlock datastores you didn't lock

**Common Errors:**
- `operation-failed`: Datastore is not locked by this session

## Commit

Commits the candidate datastore to the running datastore.

**Signature:**
```go
func (c *Client) Commit(ctx context.Context, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `mods`: Optional modifiers (Confirmed, Persist)

**Returns:**
- `Res`: Response indicating commit success
- `error`: Error if commit fails

**Requires:** `:candidate` capability

**Example - Standard Commit:**
```go
ctx := context.Background()
res, err := client.Commit(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Println("Configuration committed")
```

**Example - Confirmed Commit:**
```go
// Step 1: Issue confirmed commit (auto-rollback after 60 seconds)
res, err := client.Commit(ctx, netconf.Confirmed(60))
if err != nil {
    log.Fatal(err)
}

// Step 2: Verify configuration works
if err := verifyConfig(); err != nil {
    log.Fatal("Config verification failed, will auto-rollback")
}

// Step 3: CRITICAL - Confirm commit within timeout
res, err = client.Commit(ctx)  // Confirms the previous commit
if err != nil {
    log.Fatal(err)
}
```

**Example - Confirmed Commit with Persist ID:**
```go
res, err := client.Commit(ctx,
    netconf.Confirmed(120),
    netconf.Persist("commit-12345"))
```

**Confirmed Commit:**

Confirmed commits automatically roll back if not confirmed within the timeout period. This prevents configuration errors from persisting if the session is lost.

**Best Practices:**
- Use confirmed commit for risky changes
- Set timeout long enough for verification
- Always confirm within timeout period
- Test rollback behavior in lab environment

**Common Errors:**
- `required capability not supported`: Device lacks :candidate capability
- `operation-failed`: Validation failed during commit

## Discard

Discards uncommitted changes in the candidate datastore.

**Signature:**
```go
func (c *Client) Discard(ctx context.Context, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response indicating discard success
- `error`: Error if operation fails

**Requires:** `:candidate` capability

**Example - Discard After Error:**
```go
ctx := context.Background()

// Lock and edit
client.Lock(ctx, "candidate")
defer client.Unlock(ctx, "candidate")

_, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    // Discard changes on error
    client.Discard(ctx)
    log.Fatal(err)
}

// Validate before commit
_, err = client.Validate(ctx, "candidate")
if err != nil {
    client.Discard(ctx)
    log.Fatal(err)
}

client.Commit(ctx)
```

**Best Practices:**
- Discard on validation failure
- Discard on error during multi-step configuration
- Discard unwanted changes before starting new configuration

**Common Errors:**
- `required capability not supported`: Device lacks :candidate capability

## Validate

Validates configuration in the specified source without applying it. This is a standard NETCONF operation defined in RFC 6241.

**Signature:**
```go
func (c *Client) Validate(ctx context.Context, source string, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `source`: Source to validate ("candidate", "running", or config URL)
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response indicating validation result
- `error`: Error if validation fails

**Requires:** `:validate` capability (RFC 6241)

**Example - Validate Candidate:**
```go
ctx := context.Background()
res, err := client.Validate(ctx, "candidate")
if err != nil {
    log.Printf("Validation failed: %v", err)
    client.Discard(ctx)
    return
}
fmt.Println("Configuration is valid")
```

**Example - Validate Before Commit:**
```go
// Edit configuration
client.EditConfig(ctx, "candidate", config)

// Validate before committing
if _, err := client.Validate(ctx, "candidate"); err != nil {
    client.Discard(ctx)
    log.Fatal(err)
}

// Safe to commit
client.Commit(ctx)
```

**Best Practices:**
- Check for `:validate` capability before using
- Use in candidate datastore workflow
- Validate complex configurations during development

**Common Errors:**
- `required capability not supported`: Device lacks :validate capability
- `invalid-value`: Configuration contains invalid values
- `missing-element`: Required elements are missing

## RPC

Sends custom vendor-specific or YANG-modeled RPC operations.

**Signature:**
```go
func (c *Client) RPC(ctx context.Context, rpcXML string, mods ...func(*Req)) (Res, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `rpcXML`: Custom RPC XML content
- `mods`: Optional request modifiers

**Returns:**
- `Res`: Response from custom RPC
- `error`: Error if RPC fails

**Example - Cisco IOS-XR:**
```go
ctx := context.Background()
rpc := `<get-system-info xmlns="http://cisco.com/ns/yang/Cisco-IOS-XR-shellutil-oper"/>`
res, err := client.RPC(ctx, rpc)
if err != nil {
    log.Fatal(err)
}

info := res.Res.Get("system-info").String()
fmt.Println(info)
```

**Example - Juniper Junos:**
```go
rpc := `<get-interface-information>
    <interface-name>ge-0/0/0</interface-name>
</get-interface-information>`
res, err := client.RPC(ctx, rpc)
```

**Example - Nokia SR OS:**
```go
rpc := `<get-config xmlns="urn:nokia.com:sros:ns:yang:sr">
    <configure>
        <system>
            <name/>
        </system>
    </configure>
</get-config>`
res, err := client.RPC(ctx, rpc)
```

**Best Practices:**
- Use standard operations (Get, EditConfig, etc.) instead of RPC when possible
- Document vendor-specific RPCs in your code

**Common Errors:**
- `invalid RPC XML`: Malformed XML
- `operation-not-supported`: Device doesn't support the RPC

**When to Use RPC:**
- Vendor-specific operations not in standard NETCONF
- Device-specific diagnostics or operational commands
- Custom YANG-modeled operations
- Firmware updates or system maintenance operations

**When NOT to Use RPC:**
- Standard NETCONF operations (use dedicated methods)
- Operations that can be done with EditConfig

## Operation Modifiers

All operations support optional request modifiers to customize their behavior.

### Timeout

Sets a custom timeout for an individual operation, overriding the client's default operation timeout.

**Signature:**
```go
func Timeout(duration time.Duration) func(*Req)
```

**Example:**
```go
// Set 30-second timeout for this specific Get operation
res, err := client.Get(ctx, filter, netconf.Timeout(30*time.Second))
```

**Notes:**
- Request-specific timeout takes precedence over context deadline
- If no timeout is specified, uses the client's `OperationTimeout` (default: 60s)
- Useful for operations that may take longer than the default timeout

### DefaultOperation

Sets the default merge behavior for edit-config operations.

**Valid Values:**
- `"merge"`: Merge new configuration with existing configuration (default)
- `"replace"`: Replace existing configuration at the target node
- `"none"`: No default operation (operation must be specified per element)

**Example:**
```go
res, err := client.EditConfig(ctx, "candidate", config,
    netconf.DefaultOperation("replace"))
```

### TestOption

Controls validation and application behavior for edit-config operations.

**Valid Values:**
- `"test-then-set"`: Validate configuration before applying (default)
- `"set"`: Apply configuration without validation
- `"test-only"`: Validate configuration only, do not apply

**Example:**
```go
res, err := client.EditConfig(ctx, "candidate", config,
    netconf.TestOption("test-only"))
```

**Notes:**
- `test-then-set` is recommended for production use
- `test-only` is useful for pre-validation before batch operations

### ErrorOption

Controls error handling behavior during edit-config operations.

**Valid Values:**
- `"stop-on-error"`: Stop processing at the first error (default)
- `"continue-on-error"`: Continue processing despite errors
- `"rollback-on-error"`: Rollback all changes if any error occurs

**Example:**
```go
res, err := client.EditConfig(ctx, "candidate", config,
    netconf.ErrorOption("rollback-on-error"))
```

**Notes:**
- `rollback-on-error` provides atomic transaction semantics
- `continue-on-error` is useful for batch operations where partial success is acceptable

### Confirmed

Enables confirmed commit with automatic rollback if not confirmed within the timeout period.

**Signature:**
```go
func Confirmed(timeoutSeconds int) func(*Req)
```

**Example:**
```go
// Issue confirmed commit with 60-second timeout
res, err := client.Commit(ctx, netconf.Confirmed(60))

// Verify configuration works...

// Confirm within timeout to prevent rollback
res, err = client.Commit(ctx)
```

**Notes:**
- Requires `:confirmed-commit` capability
- Must be followed by a confirming commit within the timeout period
- See [Commit](#commit) section for complete workflow examples

### Persist

Sets a persist ID for confirmed commit operations, allowing commit identification across sessions.

**Signature:**
```go
func Persist(persistID string) func(*Req)
```

**Example:**
```go
res, err := client.Commit(ctx,
    netconf.Confirmed(120),
    netconf.Persist("commit-12345"))
```

**Notes:**
- Used with confirmed commits for later confirmation or cancellation
- Persist ID must be unique across active confirmed commits

## Response Handling

All operations return a `Res` struct:

```go
type Res struct {
    Res       xmldot.Result  // Parsed XML response
    OK        bool           // True if <ok/> received
    Errors    []ErrorModel   // NETCONF errors
    MessageID string         // Response message ID
}
```

**Example - Check Success:**
```go
res, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    log.Fatal(err)
}

if res.OK {
    fmt.Println("Operation succeeded")
}

if len(res.Errors) > 0 {
    for _, rpcErr := range res.Errors {
        fmt.Printf("Error: %s - %s\n", rpcErr.ErrorTag, rpcErr.ErrorMessage)
    }
}
```

**Example - Extract Data:**
```go
res, err := client.Get(ctx, filter)
if err != nil {
    log.Fatal(err)
}

// Use xmldot to query response
hostname := res.Res.Get("data.system.hostname").String()
interfaces := res.Res.Get("data.interfaces.interface").Array()
```

## Error Handling & Retry Logic

The go-netconf library provides automatic retry logic for transient errors with exponential backoff.

### Transient Errors

The following error conditions automatically trigger retry with exponential backoff:

- **Lock conflicts**: `lock-denied`, `in-use`
- **Transport errors**: Connection failures, session timeouts

**Note:** Only confirmed transient patterns from RFC 6241 are automatically retried.

### Retry Configuration

Configure retry behavior when creating the client:

```go
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.MaxRetries(10),                        // Max retry attempts (default: 10)
    netconf.BackoffMinDelay(1*time.Second),        // Min backoff delay (default: 1s)
    netconf.BackoffMaxDelay(60*time.Second),       // Max backoff delay (default: 60s)
    netconf.BackoffDelayFactor(1.2),               // Backoff multiplier (default: 1.2)
    netconf.LockReleaseTimeout(120*time.Second),   // Lock wait timeout (default: 120s)
)
```

### Retry Behavior

**Lock-Denied Errors**:
- Uses intelligent lock polling instead of exponential backoff
- Polls every 1 second until lock is released
- Waits up to `LockReleaseTimeout` for lock release
- Retries immediately when lock becomes available

**Transport Errors**:
- Automatically attempts to reconnect the session
- Applies exponential backoff between reconnection attempts
- Re-negotiates capabilities after successful reconnection

**Other Transient Errors**:
- Uses exponential backoff with jitter: `delay = min(minDelay * (factor ^ attempt) + jitter, maxDelay)`
- Jitter is 0-10% of delay using cryptographically secure random numbers
- Prevents thundering herd problem in multi-client scenarios

### Error Information

Errors return detailed context via `NetconfError`:

```go
res, err := client.EditConfig(ctx, "candidate", config)
if err != nil {
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        fmt.Printf("Operation: %s\n", netconfErr.Operation)
        fmt.Printf("Retries: %d\n", netconfErr.Retries)
        fmt.Printf("Transient: %v\n", netconfErr.IsTransient)

        // Access NETCONF rpc-error details
        for _, rpcErr := range netconfErr.Errors {
            fmt.Printf("Error Tag: %s\n", rpcErr.ErrorTag)
            fmt.Printf("Error Message: %s\n", rpcErr.ErrorMessage)
            fmt.Printf("Error Type: %s\n", rpcErr.ErrorType)
        }
    }
}
```

### Best Practices

- **Configure appropriate timeouts**: Set `OperationTimeout` based on expected operation duration
- **Use context deadlines**: Combine with context timeouts for request-level control
- **Monitor retry counts**: High retry counts may indicate systemic issues
- **Handle non-transient errors**: Not all errors are retryable - implement proper error handling
- **Test timeout scenarios**: Verify application behavior when operations timeout

## Common NETCONF Capabilities

Check device capabilities before using advanced features:

```go
// Check for candidate datastore support
if client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
    // Use candidate datastore operations
}
```

### Standard NETCONF 1.0 Capabilities (RFC 6241)

| Capability URN | Description | Required Methods |
|---------------|-------------|------------------|
| `urn:ietf:params:netconf:base:1.0` | Base NETCONF 1.0 protocol | Get, GetConfig, EditConfig, CopyConfig, DeleteConfig, Lock, Unlock |
| `urn:ietf:params:netconf:capability:writable-running:1.0` | Writable running datastore | EditConfig with target="running" |
| `urn:ietf:params:netconf:capability:candidate:1.0` | Candidate datastore | Commit, Discard |
| `urn:ietf:params:netconf:capability:confirmed-commit:1.0` | Confirmed commit | Commit with Confirmed() modifier |
| `urn:ietf:params:netconf:capability:rollback-on-error:1.0` | Rollback on error | EditConfig with ErrorOption("rollback-on-error") |
| `urn:ietf:params:netconf:capability:validate:1.0` | Configuration validation | Validate |
| `urn:ietf:params:netconf:capability:startup:1.0` | Startup datastore | GetConfig/CopyConfig with source/target="startup" |
| `urn:ietf:params:netconf:capability:xpath:1.0` | XPath filtering | Get/GetConfig with XPathFilter() |

### NETCONF 1.1 Capabilities (RFC 6241)

| Capability URN | Description | Notes |
|---------------|-------------|-------|
| `urn:ietf:params:netconf:base:1.1` | Base NETCONF 1.1 protocol | Chunked message framing |

### Capability Check Examples

```go
// Check for candidate datastore
if !client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
    log.Fatal("Device does not support candidate datastore")
}

// Check for confirmed commit
if !client.ServerHasCapability("urn:ietf:params:netconf:capability:confirmed-commit:1.0") {
    fmt.Println("Device does not support confirmed commit - using standard commit")
    _, err := client.Commit(ctx)
} else {
    // Use confirmed commit
    _, err := client.Commit(ctx, netconf.Confirmed(60))
}

// Check for validation capability
if client.ServerHasCapability("urn:ietf:params:netconf:capability:validate:1.0") {
    if _, err := client.Validate(ctx, "candidate"); err != nil {
        client.Discard(ctx)
        return err
    }
}

// List all server capabilities
caps := client.ServerCapabilities()
fmt.Println("Server Capabilities:")
for _, cap := range caps {
    fmt.Printf("  - %s\n", cap)
}
```

## Complete Workflow Examples

This example demonstrates the complete candidate datastore workflow with validation and confirmed commit:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/netascode/go-netconf"
)

func main() {
    // Create client with retry configuration
    client, err := netconf.NewClient(
        "192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"),
        netconf.MaxRetries(5),
        netconf.LockReleaseTimeout(120*time.Second),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Check required capabilities
    if !client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
        log.Fatal("Device doesn't support candidate datastore")
    }

    // Step 1: Lock candidate datastore
    log.Println("Locking candidate datastore...")
    if _, err := client.Lock(ctx, "candidate"); err != nil {
        log.Fatal(err)
    }
    // CRITICAL: Always use defer to unlock
    defer func() {
        log.Println("Unlocking candidate datastore...")
        if _, err := client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("Warning: Failed to unlock: %v", err)
        }
    }()

    // Step 2: Build and apply configuration
    log.Println("Editing configuration...")
    body := netconf.Body{}.
        Set("config.system.hostname", "router1").
        Set("config.system.domain-name", "example.com")

    config, err := body.String()
    if err != nil {
        client.Discard(ctx)
        log.Fatal(err)
    }

    if _, err := client.EditConfig(ctx, "candidate", config); err != nil {
        // Discard changes on error
        client.Discard(ctx)
        log.Fatal(err)
    }

    // Step 3: Validate configuration (if supported)
    if client.ServerHasCapability("urn:ietf:params:netconf:capability:validate:1.0") {
        log.Println("Validating configuration...")
        if _, err := client.Validate(ctx, "candidate"); err != nil {
            // Discard invalid changes
            client.Discard(ctx)
            log.Fatal(err)
        }
    }

    // Step 4: Confirmed commit with verification
    log.Println("Issuing confirmed commit (60 second timeout)...")
    if _, err := client.Commit(ctx, netconf.Confirmed(60)); err != nil {
        log.Fatal(err)
    }

    // Step 5: Test configuration works
    log.Println("Verifying configuration...")
    time.Sleep(5 * time.Second)
    // In production: test connectivity, verify services, etc.

    // Step 6: Confirm commit to prevent rollback
    log.Println("Confirming commit...")
    if _, err := client.Commit(ctx); err != nil {
        log.Fatal(err)
    }

    log.Println("Configuration successfully applied!")
}
```

### Example 2: Error Recovery with Multiple Changes

This example shows how to handle errors during multi-step configuration:

```go
import (
    "context"
    "fmt"
    "log"

    "github.com/netascode/go-netconf"
)

func applyMultipleConfigs(ctx context.Context, client *netconf.Client) error {
    // Lock candidate
    if _, err := client.Lock(ctx, "candidate"); err != nil {
        return fmt.Errorf("lock failed: %w", err)
    }
    defer client.Unlock(ctx, "candidate")

    // Multiple configuration changes
    configs := []struct {
        name   string
        config string
    }{
        {"hostname", `<config><system><hostname>NewRouter</hostname></system></config>`},
        {"location", `<config><system><location>DataCenter1</location></system></config>`},
        {"domain", `<config><system><domain-name>example.com</domain-name></system></config>`},
    }

    // Apply each configuration
    for _, cfg := range configs {
        log.Printf("Applying %s configuration...", cfg.name)
        _, err := client.EditConfig(ctx, "candidate", cfg.config)
        if err != nil {
            // Discard all changes on any error
            log.Printf("Failed to apply %s, discarding all changes", cfg.name)
            client.Discard(ctx)
            return fmt.Errorf("edit config %s failed: %w", cfg.name, err)
        }
    }

    // Validate all changes together (if supported)
    if client.ServerHasCapability("urn:ietf:params:netconf:capability:validate:1.0") {
        log.Println("Validating all changes...")
        if _, err := client.Validate(ctx, "candidate"); err != nil {
            log.Println("Validation failed, discarding changes")
            client.Discard(ctx)
            return fmt.Errorf("validation failed: %w", err)
        }
    }

    // Commit all changes atomically
    log.Println("Committing all changes...")
    if _, err := client.Commit(ctx); err != nil {
        return fmt.Errorf("commit failed: %w", err)
    }

    log.Println("All configurations applied successfully!")
    return nil
}
```

### Example 3: Concurrent Operations

This example demonstrates thread-safe operations across multiple goroutines:

```go
import (
    "context"
    "log"
    "sync"

    "github.com/netascode/go-netconf"
)

func concurrentOperations() {
    client, err := netconf.NewClient(
        "192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    ctx := context.Background()

    // Launch multiple goroutines for concurrent reads
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()

            // Concurrent Get operations are safe (use RLock internally)
            filter := netconf.SubtreeFilter("<interfaces/>")
            res, err := client.Get(ctx, filter)
            if err != nil {
                log.Printf("Goroutine %d: Get failed: %v", id, err)
                return
            }

            log.Printf("Goroutine %d: Retrieved %d bytes", id, len(res.Res.Raw))
        }(i)
    }

    wg.Wait()
    log.Println("All concurrent operations completed")
}
```

## See Also

- [Quick Start Guide](quickstart.md) - Getting started with go-netconf
- [Filters Guide](filters.md) - Subtree and XPath filter usage
- [Error Handling Guide](error-handling.md) - Error types and recovery strategies
- [Concurrency Guide](concurrency.md) - Thread-safe operations
- [Logging Guide](logging.md) - Structured logging configuration
