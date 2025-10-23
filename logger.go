// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"context"
	"fmt"
	"log"
	"strings"
	"unicode/utf8"
)

// MaxLogValueLength limits the length of log values to prevent log injection
// and excessive log file growth. Values longer than this are truncated.
const MaxLogValueLength = 1024

// Logger interface for pluggable logging support
//
// All methods receive a context.Context as the first parameter, enabling
// integration with context-based logging frameworks and distributed tracing.
//
// Implementations should use structured logging with key-value pairs.
// The go-netconf library provides two implementations:
//   - DefaultLogger: Wraps Go's standard log package with configurable log level
//   - NoOpLogger: Zero-overhead logging when disabled (default)
//
// Context Usage Guidelines:
//
// The context parameter enables trace correlation and debugging:
//   - Use context.Background() for utility methods and internal operations
//   - Propagate the user's context from API calls (Get, Set, EditConfig, etc.)
//   - Extract trace IDs, request IDs, or tenant IDs for correlation
//   - Check context deadline to log timeout information
//
// When to use context.Background():
//   - Internal utility methods (validation, formatting, sanitization)
//   - Constructor methods (NewClient)
//   - Configuration processing
//   - Static operations that don't involve user requests
//
// When to propagate user context:
//   - NETCONF operations (Get, EditConfig, Lock, Unlock, etc.)
//   - Network operations (Connect, Close)
//   - Request/response processing
//   - Any operation initiated by a user API call
//
// Example custom logger integration:
//
//	type SlogAdapter struct {
//	    logger *slog.Logger
//	}
//
//	func (s *SlogAdapter) Debug(ctx context.Context, msg string, keysAndValues ...any) {
//	    s.logger.DebugContext(ctx, msg, keysAndValues...)
//	}
//	// ... implement other methods
//
//	client, _ := netconf.NewClient("192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.WithLogger(&SlogAdapter{logger: slog.Default()}))
type Logger interface {
	Debug(ctx context.Context, msg string, keysAndValues ...any)
	Info(ctx context.Context, msg string, keysAndValues ...any)
	Warn(ctx context.Context, msg string, keysAndValues ...any)
	Error(ctx context.Context, msg string, keysAndValues ...any)
}

// LogLevel represents the severity threshold for logging
type LogLevel int

const (
	// LogLevelDebug enables all log levels (most verbose)
	LogLevelDebug LogLevel = iota

	// LogLevelInfo enables Info, Warn, and Error logs
	LogLevelInfo

	// LogLevelWarn enables Warn and Error logs
	LogLevelWarn

	// LogLevelError enables only Error logs
	LogLevelError

	// LogLevelNone disables all logging
	LogLevelNone
)

// String returns the string representation of a LogLevel
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	case LogLevelNone:
		return "NONE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", l)
	}
}

// DefaultLogger wraps Go's standard log package with configurable log level
//
// Log output format: [LEVEL] message key1=value1 key2=value2
//
// Context Parameter Usage:
//
// DefaultLogger does NOT use the context parameter. It is provided to satisfy
// the Logger interface and enable integration with context-aware logging frameworks.
//
// Custom logger implementations SHOULD use the context to extract trace correlation
// data such as:
//   - Request ID / Trace ID for distributed tracing
//   - User ID or tenant ID for multi-tenant applications
//   - Deadline information for timeout debugging
//
// Example of context-aware logging in a custom logger:
//
//	func (s *SlogAdapter) Debug(ctx context.Context, msg string, keysAndValues ...any) {
//	    // Extract trace ID from context
//	    if traceID := ctx.Value("trace_id"); traceID != nil {
//	        keysAndValues = append(keysAndValues, "trace_id", traceID)
//	    }
//	    s.logger.DebugContext(ctx, msg, keysAndValues...)
//	}
//
// Example:
//
//	logger := netconf.NewDefaultLogger(netconf.LogLevelDebug)
//	client, _ := netconf.NewClient("192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.WithLogger(logger))
type DefaultLogger struct {
	level LogLevel
}

// NewDefaultLogger creates a DefaultLogger with the specified log level
func NewDefaultLogger(level LogLevel) *DefaultLogger {
	return &DefaultLogger{level: level}
}

// Debug logs a debug message with structured key-value pairs
func (l *DefaultLogger) Debug(ctx context.Context, msg string, keysAndValues ...any) {
	if l.level <= LogLevelDebug {
		l.log("DEBUG", msg, keysAndValues...)
	}
}

// Info logs an informational message with structured key-value pairs
func (l *DefaultLogger) Info(ctx context.Context, msg string, keysAndValues ...any) {
	if l.level <= LogLevelInfo {
		l.log("INFO", msg, keysAndValues...)
	}
}

// Warn logs a warning message with structured key-value pairs
func (l *DefaultLogger) Warn(ctx context.Context, msg string, keysAndValues ...any) {
	if l.level <= LogLevelWarn {
		l.log("WARN", msg, keysAndValues...)
	}
}

// Error logs an error message with structured key-value pairs
func (l *DefaultLogger) Error(ctx context.Context, msg string, keysAndValues ...any) {
	if l.level <= LogLevelError {
		l.log("ERROR", msg, keysAndValues...)
	}
}

// sanitizeLogValue sanitizes a log value to prevent log injection attacks
// and limit log size. Handles control characters, ANSI escape sequences,
// Unicode attacks (RTL override, zero-width), and excessive length.
//
// Security Note: Log injection attacks exploit control characters (especially
// newlines) to inject fake log entries or hide malicious activity. This function
// neutralizes such attempts by replacing control characters with safe alternatives.
//
// Example attack prevented:
//
//	Input: "user\n[ERROR] Fake attack message"
//	Output: "user .[ERROR].Fake.attack.message"
//
// Returns the sanitized string value.
func sanitizeLogValue(val any) string {
	str := fmt.Sprintf("%v", val)

	// Truncate long values to prevent log file DoS
	if len(str) > MaxLogValueLength {
		str = str[:MaxLogValueLength] + "...[TRUNCATED]"
	}

	// Sanitize potentially malicious characters
	var builder strings.Builder
	builder.Grow(len(str))

	for i := 0; i < len(str); i++ {
		r := rune(str[i])

		// Handle multi-byte UTF-8 sequences
		if r >= 0x80 {
			// Decode full rune
			decoded, size := utf8.DecodeRuneInString(str[i:])
			if decoded == utf8.RuneError {
				builder.WriteRune('.')
				// CRITICAL: Must advance index even on error to prevent infinite loop
				if size == 0 {
					size = 1 // Ensure forward progress on malformed UTF-8
				}
				i += size - 1
				continue
			}

			// Block dangerous Unicode characters
			switch decoded {
			case 0x200B, 0x200C, 0x200D, 0xFEFF: // Zero-width characters
				// Skip entirely (don't even write space)
			case 0x202E: // Right-to-left override
				builder.WriteRune(' ')
			default:
				// Allow normal Unicode
				builder.WriteString(str[i : i+size])
				i += size - 1 // Advance past multi-byte sequence
			}
			continue
		}

		// ASCII control characters and ANSI escape sequences
		switch r {
		case '\n', '\r': // Newline injection
			builder.WriteRune(' ')
		case '\t': // Tab injection
			builder.WriteRune(' ')
		case 0x1B: // ESC - start of ANSI sequence
			builder.WriteRune('.') // Visible indicator
		case 0x07: // Bell
			builder.WriteRune('.')
		case 0x08: // Backspace (log manipulation)
			builder.WriteRune('.')
		case 0x0C: // Form feed
			builder.WriteRune(' ')
		default:
			if r < 32 || r == 127 {
				// Other control characters
				builder.WriteRune('.')
			} else {
				// Normal printable ASCII
				builder.WriteRune(r)
			}
		}
	}

	return builder.String()
}

// log formats and outputs a log message with structured key-value pairs
//
// All key-value pairs are sanitized to prevent log injection attacks and
// enforce size limits. The message string is NOT sanitized as it comes from
// trusted sources (the library code itself).
func (l *DefaultLogger) log(level, msg string, keysAndValues ...any) {
	if l.level > logLevelFromString(level) {
		return
	}

	// Pre-allocate builder capacity to reduce allocations
	estimatedSize := len(level) + len(msg) + 10 + (len(keysAndValues) * 25)
	var builder strings.Builder
	builder.Grow(estimatedSize)

	builder.WriteString("[")
	builder.WriteString(level)
	builder.WriteString("] ")
	builder.WriteString(msg)

	// Format key-value pairs
	for i := 0; i < len(keysAndValues); i += 2 {
		builder.WriteString(" ")

		// Write key (sanitized)
		if i < len(keysAndValues) {
			builder.WriteString(sanitizeLogValue(keysAndValues[i]))
		}

		// Write value (sanitized)
		if i+1 < len(keysAndValues) {
			builder.WriteString("=")
			builder.WriteString(sanitizeLogValue(keysAndValues[i+1]))
		} else {
			// Odd-length array - mark missing value explicitly
			builder.WriteString("=<MISSING>")
		}
	}

	log.Println(builder.String())
}

// logLevelFromString converts a level string to LogLevel for comparison
func logLevelFromString(level string) LogLevel {
	switch level {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARN":
		return LogLevelWarn
	case "ERROR":
		return LogLevelError
	default:
		return LogLevelNone
	}
}

// NoOpLogger is a no-operation logger that discards all log messages
//
// This logger provides zero overhead when logging is disabled. All methods
// are no-ops and will be optimized away by the compiler.
//
// This is the default logger used by go-netconf when no custom logger
// is configured.
//
// Example:
//
//	// Logging is disabled by default (uses NoOpLogger)
//	client, _ := netconf.NewClient("192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"))
type NoOpLogger struct{}

// Debug discards the log message
func (n *NoOpLogger) Debug(_ context.Context, _ string, _ ...any) {}

// Info discards the log message
func (n *NoOpLogger) Info(_ context.Context, _ string, _ ...any) {}

// Warn discards the log message
func (n *NoOpLogger) Warn(_ context.Context, _ string, _ ...any) {}

// Error discards the log message
func (n *NoOpLogger) Error(_ context.Context, _ string, _ ...any) {}
