# Quick Start Guide

This guide will help you get started with go-netconf in minutes.

## Installation

Install go-netconf using `go get`:

```bash
go get github.com/netascode/go-netconf
```

## Requirements

- Go 1.21 or later
- Network device with NETCONF support (Cisco IOS-XE, Juniper Junos, or compatible)
- SSH access to the device (port 830 by default)
- Device must have NETCONF enabled and configured

## Your First NETCONF Connection

Let's create a simple program that connects to a NETCONF device and retrieves its capabilities:

```go
package main

import (
    "fmt"
    "log"

    "github.com/netascode/go-netconf"
)

func main() {
    // Create a NETCONF client (connection established lazily)
    client, err := netconf.NewClient(
        "192.168.1.1",  // Device hostname or IP
        netconf.Username("admin"),
        netconf.Password("secret"),
        netconf.Port(830),  // Default NETCONF over SSH port
    )
    if err != nil {
        log.Fatalf("Failed to create client: %v", err)
    }
    defer client.Close()

    // Open connection to retrieve capabilities
    if err := client.Open(); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }

    // Print session information (available after connection)
    fmt.Printf("Connected! Session ID: %s\n", client.SessionID())

    // List server capabilities
    fmt.Println("\nServer Capabilities:")
    for _, cap := range client.ServerCapabilities() {
        fmt.Printf("  - %s\n", cap)
    }
}
```

## Basic Get Operation

Retrieve configuration data from the running datastore:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/netascode/go-netconf"
)

func main() {
    // Create client (connection established lazily on first operation)
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

    // Get configuration with a filter (connection opens automatically)
    filter := netconf.SubtreeFilter("<interfaces/>")
    res, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        log.Fatalf("GetConfig failed: %v", err)
    }

    // Check if operation succeeded
    if !res.OK {
        log.Fatal("Operation did not return OK")
    }

    // Parse response using xmldot
    // Get all interface names using array syntax
    interfaces := res.Res.Get("data.interfaces.interface.#.name").Array()
    fmt.Printf("Found %d interfaces\n", len(interfaces))

    // IMPORTANT: Always check array bounds before accessing elements
    if len(interfaces) > 0 {
        // Safe to access first element
        firstInterface := interfaces[0].String()
        fmt.Printf("First interface: %s\n", firstInterface)
    } else {
        fmt.Println("No interfaces found")
    }

    // Safe iteration over all interfaces
    for i, iface := range interfaces {
        fmt.Printf("Interface %d: %s\n", i+1, iface.String())
    }
}
```

## Basic GetConfig Operation

Retrieve only configuration data (no state):

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/netascode/go-netconf"
)

func main() {
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

    // Get entire configuration (no filter)
    res, err := client.GetConfig(ctx, "running", netconf.NoFilter())
    if err != nil {
        log.Fatalf("GetConfig failed: %v", err)
    }

    // Get system hostname
    hostname := res.Res.Get("data.system.hostname").String()
    fmt.Printf("Device hostname: %s\n", hostname)
}
```

## Simple EditConfig Operation

Modify device configuration:

```go
package main

import (
    "context"
    "log"

    "github.com/netascode/go-netconf"
)

func main() {
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

    // Build configuration using Body builder
    body := netconf.Body{}.
        Set("config.interfaces.interface.name", "GigabitEthernet1").
        Set("config.interfaces.interface.description", "WAN Interface").
        Set("config.interfaces.interface.enabled", true)

    config, err := body.String()
    if err != nil {
        log.Fatalf("Failed to build config: %v", err)
    }

    // Apply configuration to running datastore
    res, err := client.EditConfig(ctx, "running", config,
        netconf.DefaultOperation("merge"),  // Merge with existing config
    )
    if err != nil {
        log.Fatalf("EditConfig failed: %v", err)
    }

    if res.OK {
        log.Println("Configuration applied successfully!")
    }
}
```

## Understanding Datastores

NETCONF devices typically support three datastores:

- **running**: Active configuration currently running on the device
- **candidate**: Staging area for configuration changes (requires `:candidate` capability)
- **startup**: Configuration loaded when device boots

You can check if a datastore is supported:

```go
if client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
    fmt.Println("Candidate datastore is supported")
}
```

## Error Handling

Always check for errors and handle them appropriately:

```go
res, err := client.GetConfig(ctx, "running", filter)
if err != nil {
    // Check if it's a NetconfError
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        fmt.Printf("Operation: %s\n", netconfErr.Operation)
        fmt.Printf("Retries: %d\n", netconfErr.Retries)
        fmt.Printf("Transient: %v\n", netconfErr.IsTransient)

        // Check individual errors
        for _, e := range netconfErr.Errors {
            fmt.Printf("Error: [%s] %s - %s\n",
                e.ErrorType, e.ErrorTag, e.ErrorMessage)
        }
    }
    log.Fatal(err)
}

// Check response status
if !res.OK {
    log.Fatal("Operation failed without explicit error")
}
```

## Next Steps

Now that you have the basics, explore these topics:

1. **[Operations Guide](operations.md)** - Complete reference for all NETCONF operations
   - Detailed documentation of Get, GetConfig, EditConfig, Commit, etc.
   - Operation modifiers (Timeout, DefaultOperation, etc.)
   - Error handling and retry logic
   - Response handling patterns

2. **[Filters Guide](filters.md)** - Subtree and XPath filter syntax
   - Filter construction best practices
   - Device compatibility considerations

3. **[Error Handling Guide](error-handling.md)** - Advanced error recovery patterns
   - NetconfError structure and usage
   - Transient error handling
   - Retry strategies

4. **[Concurrency Guide](concurrency.md)** - Thread-safe concurrent operations
   - Read/write operation patterns
   - Lock coordination across goroutines

5. **[Logging Guide](logging.md)** - Structured logging configuration
   - Logger configuration and log levels
   - Automatic sensitive data redaction
   - Pretty printing and performance considerations

## Common Gotchas

### 1. Always Close Connections

Always use `defer client.Close()` immediately after checking for errors to ensure connections are properly closed:

```go
client, err := netconf.NewClient(host, opts...)
if err != nil {
    return err
}
defer client.Close()  // Place defer AFTER error check to avoid resource leak
```

### 2. Use Context for Cancellation

Pass `context.Background()` or a custom context for timeout control:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

res, err := client.GetConfig(ctx, "running", filter)
```

### 3. Check Response Status

Even if no error is returned, check `res.OK`:

```go
res, err := client.Commit(ctx)
if err != nil {
    log.Fatal(err)
}
if !res.OK {
    log.Fatal("Commit returned without OK status")
}
```

### 4. Always Handle Body.String() Errors

The Body builder's `String()` method returns `(string, error)`:

```go
// ❌ Wrong - ignores error
config := body.Set("config.hostname", "router1").String()

// ✅ Correct - handles error
body := netconf.Body{}.Set("config.hostname", "router1")
config, err := body.String()
if err != nil {
    return fmt.Errorf("failed to build config: %w", err)
}
```

### 5. Use Defer for Lock/Unlock

Always use defer immediately after successful lock to ensure cleanup:

```go
// Lock candidate datastore
if _, err := client.Lock(ctx, "candidate"); err != nil {
    return err
}
// CRITICAL: Use defer to ensure unlock even on panic
defer func() {
    if _, err := client.Unlock(ctx, "candidate"); err != nil {
        log.Printf("Warning: Failed to unlock: %v", err)
    }
}()

// Perform operations...
```

Without defer, if your code panics or returns early, the lock will remain held until the session times out (typically 30 minutes), blocking all other sessions.

## Example with Complete Error Handling

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/netascode/go-netconf"
)

func main() {
    // Create client with custom timeouts
    client, err := netconf.NewClient(
        "192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"),
        netconf.ConnectTimeout(30*time.Second),
        netconf.OperationTimeout(60*time.Second),
        netconf.MaxRetries(5),
    )
    if err != nil {
        log.Fatalf("Connection failed: %v", err)
    }
    defer client.Close()

    // Create context with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
    defer cancel()

    // Retrieve configuration
    filter := netconf.SubtreeFilter("<system/>")
    res, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        if netconfErr, ok := err.(*netconf.NetconfError); ok {
            fmt.Printf("NETCONF error in %s operation:\n", netconfErr.Operation)
            for _, e := range netconfErr.Errors {
                fmt.Printf("  [%s] %s: %s\n", e.ErrorType, e.ErrorTag, e.ErrorMessage)
            }
        }
        log.Fatal(err)
    }

    if !res.OK {
        log.Fatal("GetConfig did not return OK")
    }

    // Parse and display result
    hostname := res.Res.Get("data.system.hostname").String()
    fmt.Printf("Hostname: %s\n", hostname)
}
```

## Production-Ready Workflow Example

For production use, always use the candidate datastore workflow with proper error handling:

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/netascode/go-netconf"
)

func main() {
    // Create client with production settings
    client, err := netconf.NewClient(
        "192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"),
        netconf.ConnectTimeout(30*time.Second),
        netconf.OperationTimeout(2*time.Minute),
        netconf.MaxRetries(5),
        netconf.LockReleaseTimeout(2*time.Minute),
    )
    if err != nil {
        log.Fatalf("Connection failed: %v", err)
    }
    defer client.Close()

    // Verify candidate datastore support
    if !client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
        log.Fatal("Device does not support candidate datastore")
    }

    ctx := context.Background()

    // Apply configuration safely
    if err := applyConfiguration(ctx, client); err != nil {
        log.Fatalf("Configuration failed: %v", err)
    }

    log.Println("Configuration applied successfully!")
}

func applyConfiguration(ctx context.Context, client *netconf.Client) error {
    // Lock candidate datastore
    if _, err := client.Lock(ctx, "candidate"); err != nil {
        return err
    }
    // CRITICAL: Always use defer to unlock
    defer func() {
        if _, err := client.Unlock(ctx, "candidate"); err != nil {
            log.Printf("Warning: Failed to unlock: %v", err)
        }
    }()

    // Build configuration
    body := netconf.Body{}.
        Set("config.system.hostname", "MyRouter").
        Set("config.system.domain-name", "example.com")

    config, err := body.String()
    if err != nil {
        return err
    }

    // Edit configuration
    if _, err := client.EditConfig(ctx, "candidate", config); err != nil {
        client.Discard(ctx)
        return err
    }

    // Commit changes
    if _, err := client.Commit(ctx); err != nil {
        return err
    }

    return nil
}
```

## Authentication Methods

### Password Authentication (Quick Start)

Password authentication is simple but less secure:

```go
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
)
```

### SSH Key Authentication (Recommended for Production)

SSH key authentication is more secure and recommended for production:

```go
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("automation"),
    netconf.SSHKey("/path/to/private_key"),
    netconf.Port(830),
)
```

**Generate SSH key pair:**
```bash
# Generate RSA key pair
ssh-keygen -t rsa -b 4096 -f ~/.ssh/netconf_automation

# Copy public key to device (device-specific method)
# For Cisco IOS-XE:
# Router(config)# ip ssh pubkey-chain
# Router(conf-ssh-pubkey)# username automation
# Router(conf-ssh-pubkey-user)# key-string
# Router(conf-ssh-pubkey-data)# [paste public key content]
```

## Key Takeaways

1. **Always check errors** - Never ignore error returns
2. **Use defer for cleanup** - Especially for `client.Close()` and `client.Unlock()`
3. **Use candidate datastore** - Provides atomic transactions and rollback capability
4. **Check device capabilities** - Verify features before using them
5. **Set appropriate timeouts** - Based on your network and operation complexity
6. **Handle Body.String() errors** - Body builder operations can fail
7. **Use SSH keys in production** - More secure than password authentication

## Common Patterns Quick Reference

### Connect and Close
```go
client, err := netconf.NewClient(host, netconf.Username(user), netconf.Password(pass))
if err != nil {
    return err
}
defer client.Close()
```

### Build Configuration
```go
body := netconf.Body{}.
    Set("config.path.to.element", value)

config, err := body.String()
if err != nil {
    return err
}
```

### Lock, Edit, Commit
```go
// Lock with proper error handling
if _, err := client.Lock(ctx, "candidate"); err != nil {
    return err
}
// CRITICAL: Always defer unlock
defer func() {
    if _, err := client.Unlock(ctx, "candidate"); err != nil {
        log.Printf("Warning: unlock failed: %v", err)
    }
}()

// Edit, validate, commit
if _, err := client.EditConfig(ctx, "candidate", config); err != nil {
    client.Discard(ctx)
    return err
}
if _, err := client.Commit(ctx); err != nil {
    return err
}
```

### Error Handling
```go
res, err := client.Operation(ctx, ...)
if err != nil {
    if netconfErr, ok := err.(*netconf.NetconfError); ok {
        // Handle NETCONF-specific error
    }
    return err
}
if !res.OK {
    return fmt.Errorf("operation failed")
}
```

## Additional Resources

- [Operations Guide](operations.md) - Complete NETCONF operations reference
- [Filters Guide](filters.md) - Subtree and XPath filter syntax
- [Error Handling Guide](error-handling.md) - Advanced error recovery patterns
- [Concurrency Guide](concurrency.md) - Thread-safe concurrent operations
- [Logging Guide](logging.md) - Structured logging configuration
- [RFC 6241: NETCONF Protocol](https://tools.ietf.org/html/rfc6241)
- [RFC 6242: NETCONF over SSH](https://tools.ietf.org/html/rfc6242)
- [GoDoc Documentation](https://pkg.go.dev/github.com/netascode/go-netconf)
- [xmldot Documentation](https://github.com/netascode/xmldot) - XML parsing library
