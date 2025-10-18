# Logging in go-netconf

This guide explains how to configure and use logging in go-netconf to monitor NETCONF operations, debug issues, and audit network device interactions.

## Table of Contents

- [Overview](#overview)
- [Default Behavior](#default-behavior)
- [Enabling Logging](#enabling-logging)
- [Log Levels](#log-levels)
- [Logger Interface](#logger-interface)
- [Custom Logger Integration](#custom-logger-integration)
- [Sensitive Data Redaction](#sensitive-data-redaction)
- [Performance Considerations](#performance-considerations)
- [Log Output Examples](#log-output-examples)
- [Best Practices](#best-practices)

## Overview

go-netconf provides pluggable logging support through the `Logger` interface. You can:

- Use the built-in `DefaultLogger` (wraps Go's standard `log` package)
- Integrate third-party loggers (slog, zap, logrus, zerolog)
- Implement custom logging behavior
- Control log levels and output formatting
- Automatically redact sensitive data (passwords, secrets, keys)

### Key Features

- **Zero overhead by default**: `NoOpLogger` is used when logging is disabled
- **Structured logging**: Key-value pairs for easy parsing and analysis
- **Automatic redaction**: Sensitive XML elements are replaced with `[REDACTED]`
- **Multiple log levels**: Debug, Info, Warn, Error, None
- **Pretty printing**: Optional XML formatting for readability
- **Integration points**: Connection lifecycle, operations, retries, errors, locks, reconnections

## Default Behavior

By default, go-netconf uses `NoOpLogger` which discards all log messages. This provides **zero overhead** when logging is not needed.

```go
// No logging enabled (default)
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
)
```

## Enabling Logging

Enable logging by providing a logger via the `WithLogger` option:

```go
// Enable logging at Info level
logger := netconf.NewDefaultLogger(netconf.LogLevelInfo)
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.Username("admin"),
    netconf.Password("secret"),
    netconf.WithLogger(logger),
)
```

### DefaultLogger

The `DefaultLogger` wraps Go's standard `log` package and outputs to `stderr`:

```go
// Create logger with specific level
logger := netconf.NewDefaultLogger(netconf.LogLevelDebug)

// Use with client
client, err := netconf.NewClient(
    "192.168.1.1",
    netconf.WithLogger(logger),
    // ... other options
)
```

### Log Output Format

DefaultLogger uses structured logging with the format:

```
[LEVEL] message key1=value1 key2=value2 ...
```

Example:
```
[INFO] NETCONF connection established host=192.168.1.1 port=830 sessionID=12345 version=1.0
[DEBUG] NETCONF RPC request operation=get-config target=running sessionID=12345
[WARN] NETCONF operation retry operation=edit-config attempt=1 maxRetries=10
[ERROR] NETCONF operation failed operation=lock retries=5 transient=true errorCount=1
```

## Log Levels

go-netconf supports five log levels:

| Level | Value | Description | Use Case |
|-------|-------|-------------|----------|
| `LogLevelDebug` | 0 | Most verbose | Development, troubleshooting |
| `LogLevelInfo` | 1 | Informational | Production monitoring |
| `LogLevelWarn` | 2 | Warnings | Important events |
| `LogLevelError` | 3 | Errors only | Critical failures |
| `LogLevelNone` | 4 | No logging | Disabled logging |

### Log Level Hierarchy

Each level includes all higher severity levels:

- **Debug**: Debug + Info + Warn + Error
- **Info**: Info + Warn + Error
- **Warn**: Warn + Error
- **Error**: Error only
- **None**: No logging

### Choosing a Log Level

**Development/Troubleshooting:**
```go
logger := netconf.NewDefaultLogger(netconf.LogLevelDebug)
```
- See all operations, RPC requests/responses, retries, backoff delays
- Most verbose output
- Performance impact due to frequent logging

**Production Monitoring:**
```go
logger := netconf.NewDefaultLogger(netconf.LogLevelInfo)
```
- Track connections, disconnections, major operations
- Reasonable verbosity
- Minimal performance impact

**Production (Warnings Only):**
```go
logger := netconf.NewDefaultLogger(netconf.LogLevelWarn)
```
- Only retries, reconnections, and errors
- Low verbosity
- Very low performance impact

**Production (Errors Only):**
```go
logger := netconf.NewDefaultLogger(netconf.LogLevelError)
```
- Only operation failures and errors
- Minimal verbosity
- Negligible performance impact

## Logger Interface

The `Logger` interface defines four methods:

```go
type Logger interface {
    Debug(msg string, keysAndValues ...interface{})
    Info(msg string, keysAndValues ...interface{})
    Warn(msg string, keysAndValues ...interface{})
    Error(msg string, keysAndValues ...interface{})
}
```

### Structured Logging

All logger methods accept key-value pairs:

```go
logger.Info("operation complete",
    "operation", "get-config",
    "duration", duration.String(),
    "host", "192.168.1.1",
    "success", true,
)
```

## Custom Logger Integration

### Using slog (Go 1.21+)

```go
package main

import (
    "log/slog"
    "github.com/netascode/go-netconf"
)

// SlogAdapter wraps slog.Logger
type SlogAdapter struct {
    logger *slog.Logger
}

func (s *SlogAdapter) Debug(msg string, keysAndValues ...interface{}) {
    s.logger.Debug(msg, keysAndValues...)
}

func (s *SlogAdapter) Info(msg string, keysAndValues ...interface{}) {
    s.logger.Info(msg, keysAndValues...)
}

func (s *SlogAdapter) Warn(msg string, keysAndValues ...interface{}) {
    s.logger.Warn(msg, keysAndValues...)
}

func (s *SlogAdapter) Error(msg string, keysAndValues ...interface{}) {
    s.logger.Error(msg, keysAndValues...)
}

func main() {
    // Create slog logger with JSON output
    slogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

    // Wrap in adapter
    logger := &SlogAdapter{logger: slogger}

    // Use with go-netconf
    client, _ := netconf.NewClient(
        "192.168.1.1",
        netconf.WithLogger(logger),
    )
}
```

### Using Zap

```go
package main

import (
    "go.uber.org/zap"
    "github.com/netascode/go-netconf"
)

// ZapAdapter wraps zap.SugaredLogger
type ZapAdapter struct {
    logger *zap.SugaredLogger
}

func (z *ZapAdapter) Debug(msg string, keysAndValues ...interface{}) {
    z.logger.Debugw(msg, keysAndValues...)
}

func (z *ZapAdapter) Info(msg string, keysAndValues ...interface{}) {
    z.logger.Infow(msg, keysAndValues...)
}

func (z *ZapAdapter) Warn(msg string, keysAndValues ...interface{}) {
    z.logger.Warnw(msg, keysAndValues...)
}

func (z *ZapAdapter) Error(msg string, keysAndValues ...interface{}) {
    z.logger.Errorw(msg, keysAndValues...)
}

func main() {
    zapLogger, _ := zap.NewProduction()
    defer zapLogger.Sync()

    logger := &ZapAdapter{logger: zapLogger.Sugar()}

    client, _ := netconf.NewClient(
        "192.168.1.1",
        netconf.WithLogger(logger),
    )
}
```

### Using Logrus

```go
package main

import (
    "github.com/sirupsen/logrus"
    "github.com/netascode/go-netconf"
)

// LogrusAdapter wraps logrus.Logger
type LogrusAdapter struct {
    logger *logrus.Logger
}

func (l *LogrusAdapter) Debug(msg string, keysAndValues ...interface{}) {
    l.logger.WithFields(l.toFields(keysAndValues)).Debug(msg)
}

func (l *LogrusAdapter) Info(msg string, keysAndValues ...interface{}) {
    l.logger.WithFields(l.toFields(keysAndValues)).Info(msg)
}

func (l *LogrusAdapter) Warn(msg string, keysAndValues ...interface{}) {
    l.logger.WithFields(l.toFields(keysAndValues)).Warn(msg)
}

func (l *LogrusAdapter) Error(msg string, keysAndValues ...interface{}) {
    l.logger.WithFields(l.toFields(keysAndValues)).Error(msg)
}

func (l *LogrusAdapter) toFields(keysAndValues []interface{}) logrus.Fields {
    fields := logrus.Fields{}
    for i := 0; i < len(keysAndValues); i += 2 {
        if i+1 < len(keysAndValues) {
            key := keysAndValues[i].(string)
            fields[key] = keysAndValues[i+1]
        }
    }
    return fields
}

func main() {
    logrusLogger := logrus.New()
    logrusLogger.SetFormatter(&logrus.JSONFormatter{})

    logger := &LogrusAdapter{logger: logrusLogger}

    client, _ := netconf.NewClient(
        "192.168.1.1",
        netconf.WithLogger(logger),
    )
}
```

## Sensitive Data Redaction

go-netconf automatically redacts sensitive data before logging. The following XML elements are replaced with `[REDACTED]`:

- `<password>...</password>` → `<password>[REDACTED]</password>`
- `<secret>...</secret>` → `<secret>[REDACTED]</secret>`
- `<key>...</key>` → `<key>[REDACTED]</key>`
- `<community>...</community>` → `<community>[REDACTED]</community>`

### Redaction Example

**Original XML:**
```xml
<config>
    <snmp>
        <community>public</community>
    </snmp>
    <system>
        <password>mysecret123</password>
        <hostname>router1</hostname>
    </system>
</config>
```

**Logged XML:**
```xml
<config>
    <snmp>
        <community>[REDACTED]</community>
    </snmp>
    <system>
        <password>[REDACTED]</password>
        <hostname>router1</hostname>
    </system>
</config>
```

### Security Considerations

- Redaction is **automatic** and cannot be disabled
- Redaction patterns are **pre-compiled** for performance
- Only XML element content is redacted (element names remain visible)
- Redaction applies to all log levels (including Debug)
- Custom elements are NOT redacted (only the four built-in patterns)

### Auditing and Compliance

The automatic redaction ensures:

- **PCI-DSS compliance**: No credit card data or passwords in logs
- **GDPR compliance**: No sensitive personal data exposed
- **Security audit trail**: Safe to store logs long-term
- **Debugging support**: Structure visible even with redacted content

## Performance Considerations

### NoOpLogger (Default)

When logging is disabled (default), go-netconf uses `NoOpLogger`:

- **Zero overhead**: All methods are no-ops
- **Compiler optimization**: Empty methods are inlined/eliminated
- **No allocations**: No memory allocations for log messages
- **Production-safe**: No performance impact

### DefaultLogger

When logging is enabled, consider these performance factors:

1. **Log Level Impact**:
   - Debug: Highest overhead (frequent logging)
   - Info: Moderate overhead (reasonable logging)
   - Warn/Error: Low overhead (infrequent logging)

2. **XML Pretty Printing**:
   ```go
   // Pretty printing enabled (default) - slight overhead
   client, _ := netconf.NewClient("192.168.1.1",
       netconf.WithLogger(logger),
       netconf.WithPrettyPrintLogs(true),
   )

   // Pretty printing disabled - better performance
   client, _ := netconf.NewClient("192.168.1.1",
       netconf.WithLogger(logger),
       netconf.WithPrettyPrintLogs(false),
   )
   ```

3. **Redaction Overhead**:
   - Regex patterns are **pre-compiled** once during `NewClient()`
   - Redaction is fast (simple regex replacement)
   - Only applied to Debug-level XML logging

### Performance Recommendations

**High-throughput production:**
```go
logger := netconf.NewDefaultLogger(netconf.LogLevelInfo)
client, _ := netconf.NewClient("192.168.1.1",
    netconf.WithLogger(logger),
    netconf.WithPrettyPrintLogs(false), // Disable formatting
)
```

**Development/debugging:**
```go
logger := netconf.NewDefaultLogger(netconf.LogLevelDebug)
client, _ := netconf.NewClient("192.168.1.1",
    netconf.WithLogger(logger),
    netconf.WithPrettyPrintLogs(true), // Enable formatting
)
```

**Production monitoring:**
```go
logger := netconf.NewDefaultLogger(netconf.LogLevelWarn)
client, _ := netconf.NewClient("192.168.1.1",
    netconf.WithLogger(logger),
)
```

## Log Output Examples

### Connection Lifecycle

```
[INFO] NETCONF connection established host=192.168.1.1 port=830 sessionID=12345 version=1.0
[DEBUG] NETCONF capabilities discovered count=15
[INFO] NETCONF connection closed host=192.168.1.1 sessionID=12345
```

### Operation Logging

```
[DEBUG] NETCONF RPC request operation=get-config target=running sessionID=12345
[DEBUG] NETCONF RPC response operation=get-config ok=true errorCount=0 messageID=101
```

### Retry and Backoff

```
[WARN] NETCONF operation retry operation=edit-config attempt=1 maxRetries=10
[DEBUG] NETCONF retry backoff operation=edit-config attempt=1 delay=1.2s
[WARN] NETCONF operation retry operation=edit-config attempt=2 maxRetries=10
[DEBUG] NETCONF retry backoff operation=edit-config attempt=2 delay=1.44s
```

### Lock Management

```
[INFO] NETCONF waiting for lock release target=candidate timeout=2m0s
[INFO] NETCONF lock acquired target=candidate
```

### Reconnection

```
[WARN] NETCONF reconnecting host=192.168.1.1 reason=transport error
[INFO] NETCONF reconnected host=192.168.1.1 sessionID=12346
```

### Error Logging

```
[ERROR] NETCONF operation failed operation=edit-config retries=10 transient=false errorCount=1
[ERROR] NETCONF RPC error index=0 errorType=application errorTag=invalid-value errorMessage=Invalid configuration data
```

## Best Practices

### 1. Use Appropriate Log Levels

```go
// Development
logger := netconf.NewDefaultLogger(netconf.LogLevelDebug)

// Production
logger := netconf.NewDefaultLogger(netconf.LogLevelInfo)

// Production (high-traffic)
logger := netconf.NewDefaultLogger(netconf.LogLevelWarn)
```

### 2. Structure Your Logs

```go
// Good: Use key-value pairs
logger.Info("operation complete",
    "operation", "get-config",
    "duration", duration,
    "success", true,
)

// Bad: Concatenate strings
logger.Info(fmt.Sprintf("operation %s complete in %s", op, duration))
```

### 3. Integrate with Existing Logging Infrastructure

```go
// Use your existing logger (slog, zap, logrus)
existingLogger := slog.Default()
adapter := &SlogAdapter{logger: existingLogger}

client, _ := netconf.NewClient("192.168.1.1",
    netconf.WithLogger(adapter),
)
```

### 4. Disable Pretty Printing in Production

```go
// Production: Disable for performance
client, _ := netconf.NewClient("192.168.1.1",
    netconf.WithLogger(logger),
    netconf.WithPrettyPrintLogs(false),
)
```

### 5. Use Structured Logging for Analysis

```go
// Use JSON logger for log aggregation systems (ELK, Splunk, etc.)
jsonLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
adapter := &SlogAdapter{logger: jsonLogger}

client, _ := netconf.NewClient("192.168.1.1",
    netconf.WithLogger(adapter),
)
```

### 6. Log Rotation

```go
// Use lumberjack or similar for log rotation
import "gopkg.in/natefinch/lumberjack.v2"

logFile := &lumberjack.Logger{
    Filename:   "/var/log/netconf.log",
    MaxSize:    100, // megabytes
    MaxBackups: 3,
    MaxAge:     28, // days
    Compress:   true,
}

log.SetOutput(logFile)
logger := netconf.NewDefaultLogger(netconf.LogLevelInfo)
```

### 7. Contextual Logging

```go
// Add context to your logger adapter
type ContextLogger struct {
    logger *slog.Logger
    device string
}

func (c *ContextLogger) Info(msg string, keysAndValues ...interface{}) {
    // Add device context to all logs
    kvs := append(keysAndValues, "device", c.device)
    c.logger.Info(msg, kvs...)
}

// Use with multiple devices
logger1 := &ContextLogger{logger: slog.Default(), device: "router1"}
client1, _ := netconf.NewClient("192.168.1.1", netconf.WithLogger(logger1))

logger2 := &ContextLogger{logger: slog.Default(), device: "router2"}
client2, _ := netconf.NewClient("192.168.1.2", netconf.WithLogger(logger2))
```

## Complete Example

See [examples/logging/main.go](../examples/logging/main.go) for a complete working example demonstrating:

- Default behavior (no logging)
- Enabling logging with DefaultLogger
- Different log levels
- Custom logger integration
- Pretty printing options
- Sensitive data redaction

## Related Documentation

- [Quick Start Guide](quickstart.md)
- [Operations Guide](operations.md)
- [Filters Guide](filters.md)
- [Error Handling Guide](error-handling.md)
- [Concurrency Guide](concurrency.md)
