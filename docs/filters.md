# NETCONF Filters Guide

This guide explains how to use subtree and XPath filters effectively with go-netconf.

## Table of Contents

- [Overview](#overview)
- [Filter Types](#filter-types)
  - [Subtree Filters](#subtree-filters)
  - [XPath Filters](#xpath-filters)
  - [No Filter](#no-filter)
- [Quick Start](#quick-start)
- [Subtree Filter Examples](#subtree-filter-examples)
- [XPath Filter Examples](#xpath-filter-examples)
- [Best Practices](#best-practices)
- [Common Patterns](#common-patterns)
- [Performance Considerations](#performance-considerations)
- [Security Considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)

## Overview

Filters allow you to retrieve specific portions of configuration or state data from NETCONF-enabled devices, reducing network traffic and processing time.

**Two filter types:**
1. **Subtree Filters** - XML-based, universally supported, easier to construct
2. **XPath Filters** - Expression-based, more powerful, requires `:xpath:1.0` capability

**Key Benefits:**
- Reduced network bandwidth
- Faster response times
- Server-side filtering (offload from client)
- More maintainable code (specific queries vs full dumps)

## Filter Types

### Subtree Filters

Subtree filters use XML structure to match elements. The filter specifies the structure of the data you want to retrieve.

**Creating a Subtree Filter:**
```go
filter := netconf.SubtreeFilter("<interfaces/>")
```

**Key Characteristics:**
- ✅ Universally supported (NETCONF 1.0 base capability)
- ✅ XML-based matching - familiar to XML users
- ✅ Simple to construct for basic queries
- ✅ Can match on element structure and attributes
- ⚠️  More verbose for complex conditional queries
- ⚠️  Limited predicate support

**When to Use:**
- Maximum device compatibility required
- Simple structural queries
- Matching on element presence
- Working with well-known XML schemas

### XPath Filters

XPath filters use XPath 1.0 expressions to select data. They provide more powerful querying capabilities.

**Creating an XPath Filter:**
```go
filter := netconf.XPathFilter("/interfaces/interface[name='GigabitEthernet1']")
```

**Key Characteristics:**
- ⚠️  Requires `:xpath:1.0` capability (not universal)
- ✅ Expression-based queries
- ✅ Supports complex predicates and functions
- ✅ More concise for complex queries
- ✅ Powerful conditional matching
- ⚠️  Steeper learning curve

**When to Use:**
- Device supports `:xpath:1.0` capability
- Complex conditional queries needed
- Matching on computed values
- Compact filter expressions preferred
- Familiarity with XPath syntax

### No Filter

Retrieves all data (configuration and state).

**Creating No Filter:**
```go
filter := netconf.NoFilter()
```

**Use Cases:**
- Device discovery and capability detection
- Full configuration backup
- Development and debugging
- Small devices with limited configuration
- Initial exploration of data structure

**⚠️ Warnings:**
- Can return very large responses (megabytes)
- May timeout on complex devices
- Increased network and processing overhead
- Should not be used in production for routine operations

## Quick Start

### Basic Setup

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/netascode/go-netconf"
    "github.com/netascode/xmldot"
)

func main() {
    // Create client
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

    // Create filter
    filter := netconf.SubtreeFilter("<interfaces/>")

    // Execute query
    res, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        log.Fatal(err)
    }

    // Process results
    if res.Res.Exists() {
        interfaces := xmldot.Get(res.Res.Raw, "data.interfaces.interface").Array()
        fmt.Printf("Found %d interfaces\n", len(interfaces))
    }
}
```

### Filter Selection Flow

```go
// Check device capabilities first
func selectFilter(client *netconf.Client, ifName string) netconf.Filter {
    if client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
        // Use XPath for precise matching
        return netconf.XPathFilter(fmt.Sprintf(
            "/interfaces/interface[name='%s']", ifName))
    }

    // Fallback to subtree filter
    return netconf.SubtreeFilter(fmt.Sprintf(`
        <interfaces>
            <interface>
                <name>%s</name>
            </interface>
        </interfaces>
    `, ifName))
}
```

## Subtree Filter Examples

### Example 1: Simple Element Match

Retrieve all interfaces:

```go
ctx := context.Background()

filter := netconf.SubtreeFilter(`<interfaces/>`)
res, err := client.GetConfig(ctx, "running", filter)
if err != nil {
    log.Fatal(err)
}

// Process results
if res.Res.Exists() {
    interfaces := xmldot.Get(res.Res.Raw, "data.interfaces.interface").Array()
    fmt.Printf("Found %d interfaces\n", len(interfaces))
}
```

**How it works:**
- Empty element `<interfaces/>` selects that element and all children
- Returns complete interface hierarchy

### Example 2: Nested Element Match

Retrieve system configuration:

```go
filter := netconf.SubtreeFilter(`
<system>
    <hostname/>
    <domain-name/>
</system>
`)
res, err := client.GetConfig(ctx, "running", filter)
if err != nil {
    log.Fatal(err)
}

// Extract values
hostname := xmldot.Get(res.Res.Raw, "data.system.hostname").String()
domain := xmldot.Get(res.Res.Raw, "data.system.domain-name").String()
fmt.Printf("System: %s.%s\n", hostname, domain)
```

### Example 3: Match with Child Elements

Retrieve specific interface:

```go
filter := netconf.SubtreeFilter(`
<interfaces>
    <interface>
        <name>GigabitEthernet1</name>
    </interface>
</interfaces>
`)
res, err := client.GetConfig(ctx, "running", filter)
if err != nil {
    log.Fatal(err)
}

// Note: Behavior is device-specific
// Some devices return only matching interface
// Others may return all interfaces (filter as hint)
```

**⚠️ Device-Specific Behavior:**
- RFC 6241 allows implementation flexibility
- Some devices match exact structure
- Others use filters as selection hints
- Always test on target devices

### Example 4: Multiple Root Elements

Retrieve multiple sections:

```go
filter := netconf.SubtreeFilter(`
<system/>
<interfaces/>
<routing/>
`)
res, err := client.GetConfig(ctx, "running", filter)
```

**Note:** Multiple root elements at same level are allowed.

### Example 5: Partial Field Selection

Get specific fields from interfaces:

```go
filter := netconf.SubtreeFilter(`
<interfaces>
    <interface>
        <name/>
        <description/>
        <enabled/>
    </interface>
</interfaces>
`)
res, err := client.GetConfig(ctx, "running", filter)
```

**Benefit:** Reduces response size by requesting only needed fields.

**⚠️ Warning:** Not all devices support partial field selection. Some may ignore field specifications and return full objects.

### Example 6: Namespace-Aware Filters

With XML namespaces:

```go
filter := netconf.SubtreeFilter(`
<interfaces xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
    <interface>
        <name/>
        <type/>
    </interface>
</interfaces>
`)
res, err := client.GetConfig(ctx, "running", filter)
```

**When Required:**
- YANG-modeled configuration (most modern devices)
- Multi-vendor environments
- Strict schema validation

## XPath Filter Examples

### Example 1: Simple Path

Retrieve all interfaces:

```go
ctx := context.Background()

// Check capability first
if !client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
    log.Fatal("Device does not support XPath filters")
}

filter := netconf.XPathFilter("/interfaces/interface")
res, err := client.Get(ctx, filter)
if err != nil {
    log.Fatal(err)
}
```

### Example 2: Path with Predicate

Retrieve specific interface by name:

```go
filter := netconf.XPathFilter("/interfaces/interface[name='GigabitEthernet1']")
res, err := client.Get(ctx, filter)
if err != nil {
    log.Fatal(err)
}

// Extract interface data
iface := xmldot.Get(res.Res.Raw, "data.interfaces.interface")
if iface.Exists() {
    name := xmldot.Get(iface.Raw, "name").String()
    desc := xmldot.Get(iface.Raw, "description").String()
    fmt.Printf("Interface: %s - %s\n", name, desc)
}
```

### Example 3: Multiple Predicates

Match on multiple conditions:

```go
filter := netconf.XPathFilter(
    "/interfaces/interface[name='GigabitEthernet1'][type='ethernet']")
res, err := client.Get(ctx, filter)
```

**Equivalent to:** name='GigabitEthernet1' AND type='ethernet'

### Example 4: Logical Operators

Use `and`, `or` in predicates:

```go
// AND operator
filter := netconf.XPathFilter(
    "/interfaces/interface[enabled='true' and type='ethernet']")

// OR operator
filter := netconf.XPathFilter(
    "/interfaces/interface[type='ethernet' or type='fastethernet']")
```

### Example 5: Descendant Axis

Match at any level:

```go
filter := netconf.XPathFilter("//interface[name='GigabitEthernet1']")
res, err := client.Get(ctx, filter)
```

**Explanation:** `//` matches interface at any depth in the document.

**⚠️ Performance Warning:** Descendant axis can be slow on large configurations.

### Example 6: Attribute Selection

Select by attribute:

```go
filter := netconf.XPathFilter("/interfaces/interface[@type='ethernet']")
res, err := client.Get(ctx, filter)
```

**Note:** `@type` refers to XML attribute, not child element.

### Example 7: Position Predicates

Select first interface:

```go
filter := netconf.XPathFilter("/interfaces/interface[1]")
res, err := client.Get(ctx, filter)
```

**⚠️ Important:** XPath positions are 1-indexed (not 0-indexed).

### Example 8: Contains Function

Match partial string:

```go
filter := netconf.XPathFilter(
    "/interfaces/interface[contains(description, 'WAN')]")
res, err := client.Get(ctx, filter)
```

### Example 9: Starts-With Function

Match by prefix:

```go
filter := netconf.XPathFilter(
    "/interfaces/interface[starts-with(name, 'Gigabit')]")
res, err := client.Get(ctx, filter)
```

### Example 10: Namespace-Aware XPath

With namespace prefix:

```go
filter := netconf.XPathFilter(
    "/if:interfaces/if:interface[if:name='GigabitEthernet1']")
res, err := client.Get(ctx, filter)
```

**Note:** Namespace prefix (`if:`) must be defined by device or in context.

## Comparison: Subtree vs XPath

| Feature | Subtree | XPath |
|---------|---------|-------|
| **Support** | Universal (base capability) | Requires `:xpath:1.0` capability |
| **Syntax** | XML-based | Expression-based |
| **Learning Curve** | Easier (XML familiarity) | Steeper (XPath syntax) |
| **Query Power** | Basic structural matching | Advanced conditional queries |
| **Predicates** | Limited (element values only) | Full XPath 1.0 functions |
| **Verbosity** | More verbose for complex queries | More concise |
| **Performance** | Generally faster (simpler matching) | Varies by complexity |
| **Validation** | XML well-formedness + security | XPath syntax + dangerous functions |

**Decision Matrix:**

**Use Subtree When:**
- ✅ Maximum device compatibility needed
- ✅ Simple structural queries sufficient
- ✅ Team is more familiar with XML
- ✅ Performance is critical
- ✅ Exact structure matching acceptable

**Use XPath When:**
- ✅ Device supports `:xpath:1.0` capability
- ✅ Complex conditional queries needed
- ✅ Value-based filtering required
- ✅ Team is familiar with XPath
- ✅ Compact expressions preferred
- ✅ Computed predicates needed

## Best Practices

### 1. Always Check Capability Support

Before using XPath filters, verify device support:

```go
func getInterface(ctx context.Context, client *netconf.Client, ifName string) (netconf.Res, error) {
    var filter netconf.Filter

    if client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
        // Preferred: precise XPath matching
        filter = netconf.XPathFilter(fmt.Sprintf(
            "/interfaces/interface[name='%s']", ifName))
    } else {
        // Fallback: subtree filter
        filter = netconf.SubtreeFilter(fmt.Sprintf(`
            <interfaces>
                <interface>
                    <name>%s</name>
                </interface>
            </interfaces>
        `, ifName))
    }

    return client.GetConfig(ctx, "running", filter)
}
```

### 2. Be Specific - Avoid NoFilter()

Use specific filters to reduce data transfer:

```go
// ❌ Bad: retrieves everything (can be megabytes)
filter := netconf.NoFilter()

// ✅ Good: retrieves only what's needed
filter := netconf.SubtreeFilter("<system><hostname/></system>")
```

**Performance Impact:**
- NoFilter(): 2-10 seconds for large configs
- Specific filter: 100-500ms typical

### 3. Use Context with Timeouts

Always provide context with appropriate timeouts:

```go
// ✅ Good: explicit timeout
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

res, err := client.GetConfig(ctx, "running", filter)

// ❌ Avoid: background context without timeout
// res, err := client.GetConfig(context.Background(), "running", filter)
```

### 4. Handle Empty Results Gracefully

Filters may return no data:

```go
res, err := client.Get(ctx, filter)
if err != nil {
    return fmt.Errorf("query failed: %w", err)
}

// Check if response has data
if !res.Res.Exists() {
    log.Println("No data matched filter")
    return nil
}

// Check if specific path exists
interfaces := xmldot.Get(res.Res.Raw, "data.interfaces.interface").Array()
if len(interfaces) == 0 {
    log.Println("No interfaces found")
    return nil
}

// Process interfaces...
```

### 5. Use Namespaces When Required

Always include namespaces for YANG-modeled data:

```go
filter := netconf.SubtreeFilter(`
<interfaces xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
    <interface>
        <name/>
    </interface>
</interfaces>
`)
```

**How to find required namespaces:**
1. Check device YANG models
2. Use NoFilter() once to see full structure
3. Consult device documentation
4. Check RFC for standard models (ietf-interfaces, etc.)

### 6. Escape Special Characters in XPath

In XPath, properly escape quotes:

```go
// ❌ Wrong: causes syntax error
filter := netconf.XPathFilter("/interface[description='WAN's Interface']")

// ✅ Right: escape using different quotes
filter := netconf.XPathFilter(`/interface[description="WAN's Interface"]`)

// ✅ Alternative: escape in string
filter := netconf.XPathFilter("/interface[description=\"WAN's Interface\"]")
```

### 7. Cache Common Filters

Reuse filter objects to avoid repeated construction:

```go
var (
    // Package-level filter cache
    interfaceFilter = netconf.SubtreeFilter("<interfaces/>")
    systemFilter    = netconf.SubtreeFilter("<system/>")
    routingFilter   = netconf.SubtreeFilter("<routing/>")
)

func getInterfaces(ctx context.Context, client *netconf.Client) (netconf.Res, error) {
    // Reuse cached filter
    return client.GetConfig(ctx, "running", interfaceFilter)
}
```

### 8. Test Filters in Development

Validate filter syntax early:

```go
func testFilter(ctx context.Context, client *netconf.Client, filter netconf.Filter) error {
    // Test with short timeout
    ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()

    _, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        return fmt.Errorf("filter validation failed: %w", err)
    }
    return nil
}
```

### 9. Profile Filter Performance

Compare different filter approaches:

```go
func compareFilters(ctx context.Context, client *netconf.Client) {
    filters := map[string]netconf.Filter{
        "subtree": netconf.SubtreeFilter("<interfaces/>"),
        "xpath":   netconf.XPathFilter("/interfaces/interface"),
    }

    for name, filter := range filters {
        start := time.Now()
        _, err := client.GetConfig(ctx, "running", filter)
        elapsed := time.Since(start)

        if err != nil {
            log.Printf("%s filter failed: %v", name, err)
        } else {
            log.Printf("%s filter took %v", name, elapsed)
        }
    }
}
```

### 10. Validate User Input

When building dynamic filters from user input:

```go
func getUserInterface(ctx context.Context, client *netconf.Client, userInput string) (netconf.Res, error) {
    // ❌ Dangerous: XPath injection risk
    // filter := netconf.XPathFilter(fmt.Sprintf("/interfaces/interface[name='%s']", userInput))

    // ✅ Safe: validate input first
    if !isValidInterfaceName(userInput) {
        return netconf.Res{}, fmt.Errorf("invalid interface name: %s", userInput)
    }

    // Escape special characters
    escaped := escapeXPathString(userInput)
    filter := netconf.XPathFilter(fmt.Sprintf("/interfaces/interface[name='%s']", escaped))

    return client.GetConfig(ctx, "running", filter)
}

func isValidInterfaceName(name string) bool {
    // Only allow alphanumeric, dash, slash, dot
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9\-/\.]+$`, name)
    return matched
}

func escapeXPathString(s string) string {
    // Escape single quotes by using double quotes
    if strings.Contains(s, "'") && !strings.Contains(s, "\"") {
        return s
    }
    return strings.ReplaceAll(s, "'", "\\'")
}
```

## Common Patterns

### Pattern 1: Get All of One Type

```go
// Subtree
filter := netconf.SubtreeFilter("<interfaces/>")

// XPath
filter := netconf.XPathFilter("/interfaces/interface")
```

### Pattern 2: Get Specific Instance

```go
// Subtree (hint to device)
filter := netconf.SubtreeFilter(`
<interfaces>
    <interface>
        <name>GigabitEthernet1</name>
    </interface>
</interfaces>
`)

// XPath (precise matching)
filter := netconf.XPathFilter("/interfaces/interface[name='GigabitEthernet1']")
```

### Pattern 3: Get Multiple Sections

```go
// Subtree
filter := netconf.SubtreeFilter(`
<system/>
<interfaces/>
`)

// XPath (union operator)
filter := netconf.XPathFilter("/system | /interfaces")
```

### Pattern 4: Get Enabled Items Only

```go
// Subtree (gets all, filter in application)
filter := netconf.SubtreeFilter("<interfaces/>")
res, err := client.Get(ctx, filter)
// Client-side filtering
for _, iface := range xmldot.Get(res.Res.Raw, "data.interfaces.interface").Array() {
    if xmldot.Get(iface.Raw, "enabled").Bool() {
        // Process enabled interface
    }
}

// XPath (server-side filtering)
filter := netconf.XPathFilter("/interfaces/interface[enabled='true']")
res, err := client.Get(ctx, filter)
// All results are pre-filtered
```

### Pattern 5: Get Specific Fields

```go
// Subtree (partial field selection)
filter := netconf.SubtreeFilter(`
<interfaces>
    <interface>
        <name/>
        <description/>
    </interface>
</interfaces>
`)

// XPath (get full interface, extract fields in code)
filter := netconf.XPathFilter("/interfaces/interface")
name := xmldot.Get(res.Res.Raw, "data.interfaces.interface.0.name").String()
desc := xmldot.Get(res.Res.Raw, "data.interfaces.interface.0.description").String()
```

### Pattern 6: Dynamic Filter Construction

```go
type FilterBuilder struct {
    client *netconf.Client
}

func (fb *FilterBuilder) InterfaceByName(name string) netconf.Filter {
    if fb.client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
        return netconf.XPathFilter(fmt.Sprintf(
            "/interfaces/interface[name='%s']", name))
    }
    return netconf.SubtreeFilter(fmt.Sprintf(`
        <interfaces>
            <interface>
                <name>%s</name>
            </interface>
        </interfaces>
    `, name))
}
```

### Pattern 7: Filter Strategies

```go
type FilterStrategy interface {
    GetInterfaces(ctx context.Context, client *netconf.Client) (netconf.Res, error)
}

type SubtreeStrategy struct{}

func (s SubtreeStrategy) GetInterfaces(ctx context.Context, client *netconf.Client) (netconf.Res, error) {
    filter := netconf.SubtreeFilter("<interfaces/>")
    return client.GetConfig(ctx, "running", filter)
}

type XPathStrategy struct{}

func (x XPathStrategy) GetInterfaces(ctx context.Context, client *netconf.Client) (netconf.Res, error) {
    filter := netconf.XPathFilter("/interfaces/interface")
    return client.Get(ctx, filter)
}

func NewFilterStrategy(client *netconf.Client) FilterStrategy {
    if client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
        return XPathStrategy{}
    }
    return SubtreeStrategy{}
}

// Usage
strategy := NewFilterStrategy(client)
res, err := strategy.GetInterfaces(ctx, client)
```

## Performance Considerations

### Filter Complexity Impact

| Filter Type | Complexity | Typical Time |
|-------------|-----------|--------------|
| NoFilter() | Highest | 2-10 seconds |
| Subtree (broad) | High | 500-2000ms |
| Subtree (specific) | Medium | 100-500ms |
| XPath (simple path) | Medium | 100-500ms |
| XPath (complex predicates) | Variable | 200-1000ms |
| XPath (descendant axis) | High | 500-2000ms |

### Optimization Tips

1. **Use specific filters:**
   ```go
   // ❌ Slow: broad filter
   filter := netconf.SubtreeFilter("<config/>")

   // ✅ Fast: specific filter
   filter := netconf.SubtreeFilter("<interfaces><interface><name>eth0</name></interface></interfaces>")
   ```

2. **Avoid descendant axis in XPath:**
   ```go
   // ❌ Slow: descendant axis at every level
   filter := netconf.XPathFilter("//interface//enabled")

   // ✅ Fast: specific path
   filter := netconf.XPathFilter("/interfaces/interface/enabled")
   ```

3. **Cache filter results:**
   ```go
   type CachedFilter struct {
       filter    netconf.Filter
       result    netconf.Res
       timestamp time.Time
       ttl       time.Duration
   }

   func (cf *CachedFilter) Get(ctx context.Context, client *netconf.Client) (netconf.Res, error) {
       if time.Since(cf.timestamp) < cf.ttl {
           return cf.result, nil
       }

       res, err := client.GetConfig(ctx, "running", cf.filter)
       if err != nil {
           return res, err
       }

       cf.result = res
       cf.timestamp = time.Now()
       return res, nil
   }
   ```

4. **Use request-specific timeouts:**
   ```go
   // Adjust timeout based on filter complexity
   res, err := client.GetConfig(ctx, "running", filter,
       netconf.Timeout(10*time.Second))
   ```

## Security Considerations

### XPath Injection

When building XPath filters from user input:

```go
// ❌ Vulnerable to XPath injection
func getInterfaceUnsafe(ctx context.Context, client *netconf.Client, userInput string) (netconf.Res, error) {
    filter := netconf.XPathFilter(fmt.Sprintf("/interfaces/interface[name='%s']", userInput))
    return client.GetConfig(ctx, "running", filter)
}

// Attack: userInput = "' or '1'='1"
// Result: /interfaces/interface[name='' or '1'='1']
// Returns all interfaces instead of specific one

// ✅ Safe: validate and sanitize input
func getInterfaceSafe(ctx context.Context, client *netconf.Client, userInput string) (netconf.Res, error) {
    // Validate input format
    if !regexp.MustCompile(`^[a-zA-Z0-9\-/\.]+$`).MatchString(userInput) {
        return netconf.Res{}, fmt.Errorf("invalid interface name")
    }

    filter := netconf.XPathFilter(fmt.Sprintf("/interfaces/interface[name='%s']", userInput))
    return client.GetConfig(ctx, "running", filter)
}
```

## Troubleshooting

### Problem: "invalid XPath filter" Error

**Cause:** XPath syntax error or blocked dangerous function

**Solutions:**
1. Verify XPath 1.0 syntax
2. Check for dangerous functions (`document()`, `system-property()`)
3. Validate balanced brackets and parentheses
4. Reduce expression complexity
5. Check predicate depth

```go
// ❌ Bad: dangerous function
filter := netconf.XPathFilter("document('/etc/passwd')")

// ❌ Bad: unbalanced brackets
filter := netconf.XPathFilter("/interface[name='eth0'")

// ❌ Bad: excessive complexity
filter := netconf.XPathFilter("//a[//b[//c[//d]]]")

// ✅ Good
filter := netconf.XPathFilter("/interfaces/interface[name='eth0']")
```

**Debugging:**
```go
// Test filter syntax
_, err := client.GetConfig(ctx, "running", filter)
if err != nil {
    if strings.Contains(err.Error(), "invalid XPath") {
        log.Printf("XPath syntax error: %v", err)
        // Simplify expression or check syntax
    }
}
```

### Problem: "invalid subtree filter" Error

**Cause:** Malformed XML or security violation

**Solutions:**
1. Validate XML well-formedness
2. Remove DOCTYPE or ENTITY declarations
3. Verify proper namespace declarations
4. Check for control characters

```go
// ❌ Bad: malformed XML
filter := netconf.SubtreeFilter("<interfaces")

// ❌ Bad: XXE security risk
filter := netconf.SubtreeFilter(`
<!DOCTYPE foo [<!ENTITY xxe SYSTEM 'file:///etc/passwd'>]>
<test>&xxe;</test>
`)

// ❌ Bad: control characters
filter := netconf.SubtreeFilter("<interfaces>\x00</interfaces>")

// ✅ Good
filter := netconf.SubtreeFilter("<interfaces/>")
```

### Problem: Empty Result Set

**Cause:** Filter doesn't match any data

**Solutions:**
1. Verify element names match device schema
2. Check namespace requirements
3. Test with NoFilter() first to see full structure
4. Validate filter against device capabilities

```go
// Debug: Get full structure first
res, err := client.GetConfig(ctx, "running", netconf.NoFilter())
if err != nil {
    log.Fatal(err)
}

// Examine structure
fmt.Println("Full configuration structure:")
fmt.Println(res.Res.Get("data").String())

// Then build specific filter based on actual structure
```

### Problem: Filter Too Broad

**Cause:** Filter matches too much data, causing timeouts or large responses

**Solutions:**
1. Add more specific predicates
2. Filter for specific instances
3. Use XPath predicates for value matching
4. Implement pagination if supported

```go
// ❌ Too broad
filter := netconf.SubtreeFilter("<config/>")

// ✅ More specific
filter := netconf.SubtreeFilter("<interfaces><interface><name>eth0</name></interface></interfaces>")

// ✅ Even better with XPath
filter := netconf.XPathFilter("/interfaces/interface[name='eth0']")
```

### Problem: Namespace Errors

**Cause:** Missing or incorrect namespace declarations

**Solutions:**
1. Include proper xmlns attributes
2. Use device-specific namespaces
3. Check YANG module documentation
4. Inspect full config to see namespaces

```go
// Debug: Check what namespaces are used
res, err := client.GetConfig(ctx, "running", netconf.NoFilter())
// Look for xmlns attributes in response

// ❌ May fail without namespace
filter := netconf.SubtreeFilter("<interfaces><interface><name/></interface></interfaces>")

// ✅ Better with namespace
filter := netconf.SubtreeFilter(`
<interfaces xmlns="urn:ietf:params:xml:ns:yang:ietf-interfaces">
    <interface>
        <name/>
    </interface>
</interfaces>
`)
```

### Problem: Performance Issues

**Cause:** Overly complex filters or large result sets

**Solutions:**
1. Simplify XPath expressions
2. Reduce predicate nesting
3. Use specific paths instead of descendant axis
4. Implement caching
5. Use request-specific timeouts

```go
// ❌ Slow: descendant axis
filter := netconf.XPathFilter("//interface//enabled")

// ✅ Faster: specific path
filter := netconf.XPathFilter("/interfaces/interface/enabled")

// ✅ Add timeout for slow queries
res, err := client.GetConfig(ctx, "running", filter,
    netconf.Timeout(30*time.Second))
```

### Problem: Device Does Not Support XPath

**Cause:** Device lacks `:xpath:1.0` capability

**Solution:** Implement capability detection with fallback

```go
func getFilterForDevice(client *netconf.Client, query string) netconf.Filter {
    // Check capability
    caps := client.ServerCapabilities()
    hasXPath := false
    for _, cap := range caps {
        if strings.Contains(cap, "xpath") {
            hasXPath = true
            break
        }
    }

    if hasXPath {
        return netconf.XPathFilter(query)
    }

    // Fallback to subtree
    return convertToSubtree(query)
}
```

### Problem: Context Deadline Exceeded

**Cause:** Operation timeout before filter returns

**Solutions:**
1. Increase context timeout
2. Use more specific filters
3. Use request-specific timeout
4. Check network latency

```go
// ❌ May timeout with default
ctx := context.Background()
res, err := client.GetConfig(ctx, "running", broadFilter)

// ✅ Explicit timeout
ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
defer cancel()
res, err := client.GetConfig(ctx, "running", broadFilter)

// ✅ Or use request-specific timeout
res, err := client.GetConfig(context.Background(), "running", broadFilter,
    netconf.Timeout(60*time.Second))
```

## See Also

- [Quick Start Guide](quickstart.md) - Getting started with go-netconf
- [Operations Guide](operations.md) - Complete NETCONF operations documentation
- [Error Handling Guide](error-handling.md) - Error recovery and retry strategies
- [Concurrency Guide](concurrency.md) - Concurrent filter operations
- [Logging Guide](logging.md) - Structured logging configuration
- [RFC 6241 Section 6](https://tools.ietf.org/html/rfc6241#section-6) - NETCONF filter specification
- [XPath 1.0 Specification](https://www.w3.org/TR/xpath-10/) - XPath reference
- [YANG RFC 7950](https://tools.ietf.org/html/rfc7950) - YANG data modeling language

## Complete Example

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/netascode/go-netconf"
    "github.com/netascode/xmldot"
)

func main() {
    // Create client
    client, err := netconf.NewClient(
        "192.168.1.1",
        netconf.Username("admin"),
        netconf.Password("secret"),
        netconf.Port(830),
    )
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer client.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Select filter based on capabilities
    var filter netconf.Filter
    if client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
        fmt.Println("Using XPath filter")
        filter = netconf.XPathFilter("/interfaces/interface[enabled='true']")
    } else {
        fmt.Println("Using subtree filter")
        filter = netconf.SubtreeFilter("<interfaces/>")
    }

    // Execute query
    res, err := client.GetConfig(ctx, "running", filter)
    if err != nil {
        log.Fatalf("Query failed: %v", err)
    }

    // Process results
    if !res.Res.Exists() {
        log.Println("No data returned")
        return
    }

    interfaces := xmldot.Get(res.Res.Raw, "data.interfaces.interface").Array()
    fmt.Printf("Found %d interfaces\n", len(interfaces))

    for i, iface := range interfaces {
        name := xmldot.Get(iface.Raw, "name").String()
        desc := xmldot.Get(iface.Raw, "description").String()
        enabled := xmldot.Get(iface.Raw, "enabled").Bool()

        fmt.Printf("%d. %s\n", i+1, name)
        fmt.Printf("   Description: %s\n", desc)
        fmt.Printf("   Enabled: %v\n", enabled)
    }
}
```
