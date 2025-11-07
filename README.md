<h1 align="center">go-netconf</h1>

<p align="center">
<a href="https://godoc.org/github.com/netascode/go-netconf"><img src="https://img.shields.io/badge/api-reference-blue.svg?style=flat-square" alt="GoDoc"></a>
<a href="https://goreportcard.com/report/github.com/netascode/go-netconf"><img src="https://goreportcard.com/badge/github.com/netascode/go-netconf?style=flat-square" alt="Go Report Card"></a>
<a href="https://github.com/netascode/go-netconf/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/netascode/go-netconf/ci.yml?branch=main&style=flat-square&label=build" alt="CI"></a>
<a href="https://codecov.io/gh/netascode/go-netconf"><img src="https://codecov.io/gh/netascode/go-netconf/branch/main/graph/badge.svg?style=flat-square" alt="codecov"></a>
<a href="https://github.com/netascode/go-netconf/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MPL--2.0-blue.svg?style=flat-square" alt="License"></a>
</p>

<h3 align="center">A simple, fluent Go client library for interacting with network devices using the NETCONF protocol (RFC 6241).</h3>

## Features

- **Simple API**: Fluent, chainable API design
- **Lazy Connection**: Automatic connection establishment on first operation
- **XML Manipulation**: Path-based XML operations using [xmldot](https://github.com/netascode/xmldot) (like gjson/sjson for JSON)
- **Complete NETCONF Support**: All standard operations (Get, GetConfig, EditConfig, Lock, Commit, etc.)
- **Robust Transport**: Built on [scrapligo](https://github.com/scrapli/scrapligo) for reliable SSH connectivity and NETCONF protocol handling
- **Automatic Retry**: Built-in retry logic with exponential backoff for transient errors
- **Thread-Safe**: Concurrent read operations with synchronized write operations
- **Capability Discovery**: Automatic capability negotiation and checking
- **Transaction Support**: Full candidate datastore workflow support
- **Structured Logging**: Configurable logging with automatic sensitive data redaction

## Installation

```bash
go get github.com/netascode/go-netconf
```

## Requirements

- Go 1.24 or later
- Network device with NETCONF support

## Quick Start

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
        netconf.Port(830),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Connection opens automatically on first operation
    ctx := context.Background()
    filter := netconf.SubtreeFilter("<interfaces/>")
    res, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        log.Fatal(err)
    }

    // Parse response using xmldot
    ifName := res.Res.Get("data.interfaces.interface.name").String()
    fmt.Println("Interface:", ifName)
}
```

## Usage

### Client Creation

The client uses lazy connection - the connection is established automatically on the first operation:

```go
// Create client without connecting
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.Port(830),
    netconf.MaxRetries(5),
    netconf.AttemptTimeout(60*time.Second),
    netconf.TotalTimeout(5*time.Minute),
)

// Connection established automatically on first operation
res, err := client.GetConfig(ctx, "running", filter)

// Or open connection explicitly if needed
if err := client.Open(); err != nil {
    log.Fatal(err)
}
```

### Get Configuration

```go
ctx := context.Background()

// Get with filter
filter := netconf.SubtreeFilter("<interfaces/>")
res, err := client.GetConfig(ctx, "running", filter)

// Get with XPath filter
filter := netconf.XPathFilter("/interfaces/interface[name='GigabitEthernet1']")
res, err := client.Get(ctx, filter)

// Get without filter (all data)
res, err := client.GetConfig(ctx, "running", netconf.NoFilter())
```

### Edit Configuration

```go
ctx := context.Background()

// Build configuration using Body builder
config := netconf.Body{}.
    Set("interfaces.interface.name", "GigabitEthernet1").
    Set("interfaces.interface.description", "WAN Interface").
    Set("interfaces.interface.enabled", true).String()

// Apply configuration
res, err := client.EditConfig(ctx, "candidate", config,
    netconf.DefaultOperation("merge"),
    netconf.TestOption("test-then-set"),
)
```

### Transaction Workflow

```go
ctx := context.Background()

// Lock candidate datastore
if _, err := client.Lock(ctx, "candidate"); err != nil {
    log.Fatal(err)
}
defer client.Unlock(ctx, "candidate")

// Edit configuration
config := netconf.Body{}.
    Set("system.hostname", "router1").String()
_, err = client.EditConfig(ctx, "candidate", config)
if err != nil {
    client.Discard(ctx)
    log.Fatal(err)
}

// Commit changes
if _, err := client.Commit(ctx); err != nil {
    log.Fatal(err)
}
```

### Error Handling

The library automatically retries transient errors (lock conflicts, resource exhaustion, transport failures) with exponential backoff:

```go
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.MaxRetries(10),                      // Maximum retry attempts
    netconf.BackoffMinDelay(1*time.Second),      // Minimum 1 second delay
    netconf.BackoffMaxDelay(60*time.Second),     // Maximum 60 second delay
    netconf.BackoffDelayFactor(1.2),             // Exponential factor
)
```

### Connection Lifecycle Management

The client uses lazy connection - the connection is established automatically on the first operation. For explicit connection control, use `Open()` to establish the connection immediately, and `IsClosed()` to check connection state:

```go
// Create client without connecting
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
)
if err != nil {
    log.Fatal(err)
}

// Connection opens automatically on first operation
res, err := client.GetConfig(ctx, "running", filter)

// Close connection
client.Close()

// Reopen connection (Open is idempotent - safe to call even if already open)
if err := client.Open(); err != nil {
    log.Fatal(err)
}
res, err = client.EditConfig(ctx, "candidate", config)
```

### Capability Checking

```go
// Check if server supports candidate datastore
if client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
    ctx := context.Background()
    // Use candidate datastore
    client.Lock(ctx, "candidate")
    // ...
}

// Get all server capabilities
caps := client.ServerCapabilities()
for _, cap := range caps {
    fmt.Println(cap)
}
```

### Custom RPC Operations

```go
ctx := context.Background()
rpc := `
<get-system-info xmlns="http://example.com/system">
    <detail>full</detail>
</get-system-info>
`
res, err := client.RPC(ctx, rpc)
```

## Supported Operations

| Operation | Description |
|-----------|-------------|
| Get | Retrieve configuration and state data |
| GetConfig | Retrieve configuration from datastore |
| EditConfig | Modify configuration |
| CopyConfig | Copy configuration between datastores |
| DeleteConfig | Delete configuration datastore |
| Lock | Lock datastore |
| Unlock | Unlock datastore |
| Commit | Commit candidate to running |
| Discard | Discard candidate changes |
| Validate | Validate configuration (requires `:validate` capability) |
| RPC | Send custom RPC operations |

## Documentation

- [GoDoc](https://pkg.go.dev/github.com/netascode/go-netconf)
- [RFC 6241 - NETCONF Protocol](https://tools.ietf.org/html/rfc6241)
- [RFC 6242 - NETCONF over SSH](https://tools.ietf.org/html/rfc6242)

## Examples

See the [examples](examples/) directory for library usage examples:

- **basic** - Client creation, GetConfig, EditConfig, response parsing
- **candidate** - Lock/Unlock, Validate, Commit/Discard workflows
- **concurrent** - Thread-safe concurrent operations
- **custom-rpc** - Custom RPC operations with RPC() method
- **filters** - SubtreeFilter, XPathFilter, NoFilter APIs
- **logging** - Logger configuration and log levels

## Testing

```bash
# Run tests
make test

# Run tests with coverage
make coverage

# Run linters
make lint

# Run all checks
make verify
```

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.

## Acknowledgments

This library is built on top of [scrapligo](https://github.com/scrapli/scrapligo) for the NETCONF transport layer. Scrapligo provides robust SSH connectivity and NETCONF protocol handling, allowing go-netconf to focus on providing a simple, idiomatic Go API.

## License

Mozilla Public License Version 2.0 - see [LICENSE](LICENSE) for details.
