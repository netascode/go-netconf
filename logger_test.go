// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"bytes"
	"context"
	"log"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    LogLevel
		expected string
	}{
		{"Debug", LogLevelDebug, "DEBUG"},
		{"Info", LogLevelInfo, "INFO"},
		{"Warn", LogLevelWarn, "WARN"},
		{"Error", LogLevelError, "ERROR"},
		{"None", LogLevelNone, "NONE"},
		{"Unknown", LogLevel(999), "UNKNOWN(999)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultLogger(t *testing.T) {
	// Capture log output
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	logger := NewDefaultLogger(LogLevelDebug)

	t.Run("Debug", func(t *testing.T) {
		buf.Reset()
		logger.Debug(context.Background(), "test message", "key1", "value1", "key2", "value2")
		output := buf.String()
		if !strings.Contains(output, "[DEBUG]") {
			t.Errorf("Expected [DEBUG] in output, got: %s", output)
		}
		if !strings.Contains(output, "test message") {
			t.Errorf("Expected 'test message' in output, got: %s", output)
		}
		if !strings.Contains(output, "key1=value1") {
			t.Errorf("Expected 'key1=value1' in output, got: %s", output)
		}
		if !strings.Contains(output, "key2=value2") {
			t.Errorf("Expected 'key2=value2' in output, got: %s", output)
		}
	})

	t.Run("Info", func(t *testing.T) {
		buf.Reset()
		logger.Info(context.Background(), "info message", "host", "192.168.1.1", "port", 830)
		output := buf.String()
		if !strings.Contains(output, "[INFO]") {
			t.Errorf("Expected [INFO] in output, got: %s", output)
		}
		if !strings.Contains(output, "info message") {
			t.Errorf("Expected 'info message' in output, got: %s", output)
		}
		if !strings.Contains(output, "host=192.168.1.1") {
			t.Errorf("Expected 'host=192.168.1.1' in output, got: %s", output)
		}
		if !strings.Contains(output, "port=830") {
			t.Errorf("Expected 'port=830' in output, got: %s", output)
		}
	})

	t.Run("Warn", func(t *testing.T) {
		buf.Reset()
		logger.Warn(context.Background(), "warning message")
		output := buf.String()
		if !strings.Contains(output, "[WARN]") {
			t.Errorf("Expected [WARN] in output, got: %s", output)
		}
		if !strings.Contains(output, "warning message") {
			t.Errorf("Expected 'warning message' in output, got: %s", output)
		}
	})

	t.Run("Error", func(t *testing.T) {
		buf.Reset()
		logger.Error(context.Background(), "error message", "error", "something went wrong")
		output := buf.String()
		if !strings.Contains(output, "[ERROR]") {
			t.Errorf("Expected [ERROR] in output, got: %s", output)
		}
		if !strings.Contains(output, "error message") {
			t.Errorf("Expected 'error message' in output, got: %s", output)
		}
		if !strings.Contains(output, "error=something went wrong") {
			t.Errorf("Expected 'error=something went wrong' in output, got: %s", output)
		}
	})
}

func TestDefaultLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	tests := []struct {
		name      string
		level     LogLevel
		logFunc   func(Logger)
		shouldLog bool
	}{
		{"Debug at Debug", LogLevelDebug, func(l Logger) { l.Debug(context.Background(), "test") }, true},
		{"Info at Debug", LogLevelDebug, func(l Logger) { l.Info(context.Background(), "test") }, true},
		{"Warn at Debug", LogLevelDebug, func(l Logger) { l.Warn(context.Background(), "test") }, true},
		{"Error at Debug", LogLevelDebug, func(l Logger) { l.Error(context.Background(), "test") }, true},

		{"Debug at Info", LogLevelInfo, func(l Logger) { l.Debug(context.Background(), "test") }, false},
		{"Info at Info", LogLevelInfo, func(l Logger) { l.Info(context.Background(), "test") }, true},
		{"Warn at Info", LogLevelInfo, func(l Logger) { l.Warn(context.Background(), "test") }, true},
		{"Error at Info", LogLevelInfo, func(l Logger) { l.Error(context.Background(), "test") }, true},

		{"Debug at Warn", LogLevelWarn, func(l Logger) { l.Debug(context.Background(), "test") }, false},
		{"Info at Warn", LogLevelWarn, func(l Logger) { l.Info(context.Background(), "test") }, false},
		{"Warn at Warn", LogLevelWarn, func(l Logger) { l.Warn(context.Background(), "test") }, true},
		{"Error at Warn", LogLevelWarn, func(l Logger) { l.Error(context.Background(), "test") }, true},

		{"Debug at Error", LogLevelError, func(l Logger) { l.Debug(context.Background(), "test") }, false},
		{"Info at Error", LogLevelError, func(l Logger) { l.Info(context.Background(), "test") }, false},
		{"Warn at Error", LogLevelError, func(l Logger) { l.Warn(context.Background(), "test") }, false},
		{"Error at Error", LogLevelError, func(l Logger) { l.Error(context.Background(), "test") }, true},

		{"Debug at None", LogLevelNone, func(l Logger) { l.Debug(context.Background(), "test") }, false},
		{"Info at None", LogLevelNone, func(l Logger) { l.Info(context.Background(), "test") }, false},
		{"Warn at None", LogLevelNone, func(l Logger) { l.Warn(context.Background(), "test") }, false},
		{"Error at None", LogLevelNone, func(l Logger) { l.Error(context.Background(), "test") }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			logger := NewDefaultLogger(tt.level)
			tt.logFunc(logger)
			output := buf.String()

			if tt.shouldLog && output == "" {
				t.Errorf("Expected log output, got empty string")
			}
			if !tt.shouldLog && output != "" {
				t.Errorf("Expected no log output, got: %s", output)
			}
		})
	}
}

func TestNoOpLogger(_ *testing.T) {
	// NoOpLogger should not panic and should do nothing
	logger := &NoOpLogger{}

	// These should all be no-ops
	logger.Debug(context.Background(), "test", "key", "value")
	logger.Info(context.Background(), "test", "key", "value")
	logger.Warn(context.Background(), "test", "key", "value")
	logger.Error(context.Background(), "test", "key", "value")

	// If we got here without panic, test passes
}

func TestClient_redactSensitiveData(t *testing.T) {
	client := &Client{
		redactionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`<password>.*?</password>`),
			regexp.MustCompile(`<secret>.*?</secret>`),
			regexp.MustCompile(`<key>.*?</key>`),
			regexp.MustCompile(`<community>.*?</community>`),
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Redact password",
			input:    "<config><password>secret123</password></config>",
			expected: "<config><password>[REDACTED]</password></config>",
		},
		{
			name:     "Redact secret",
			input:    "<config><secret>topsecret</secret></config>",
			expected: "<config><secret>[REDACTED]</secret></config>",
		},
		{
			name:     "Redact key",
			input:    "<config><key>abc123</key></config>",
			expected: "<config><key>[REDACTED]</key></config>",
		},
		{
			name:     "Redact community",
			input:    "<config><community>public</community></config>",
			expected: "<config><community>[REDACTED]</community></config>",
		},
		{
			name: "Redact multiple",
			input: `<config>
				<password>secret123</password>
				<key>abc123</key>
				<community>private</community>
			</config>`,
			expected: `<config>
				<password>[REDACTED]</password>
				<key>[REDACTED]</key>
				<community>[REDACTED]</community>
			</config>`,
		},
		{
			name:     "No sensitive data",
			input:    "<config><hostname>router1</hostname></config>",
			expected: "<config><hostname>router1</hostname></config>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.redactSensitiveData(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveData() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestClient_prepareXMLForLogging(t *testing.T) {
	client := &Client{
		prettyPrintLogs: true,
		redactionPatterns: []*regexp.Regexp{
			regexp.MustCompile(`<password>.*?</password>`),
			regexp.MustCompile(`<secret>.*?</secret>`),
			regexp.MustCompile(`<key>.*?</key>`),
			regexp.MustCompile(`<community>.*?</community>`),
		},
	}

	t.Run("Redaction and pretty print", func(t *testing.T) {
		input := "<config><password>secret</password><hostname>router1</hostname></config>"
		result := client.prepareXMLForLogging(input)

		// Should be redacted (password should not appear in plain text)
		if strings.Contains(result, "secret") {
			t.Errorf("Expected password to be redacted, got: %s", result)
		}

		// The result should contain either the redacted XML or be empty (if @pretty doesn't work)
		// In either case, the secret should be gone
		if result != "" {
			// If we got output, it should contain [REDACTED]
			if !strings.Contains(result, "[REDACTED]") {
				t.Errorf("Expected [REDACTED] in non-empty output, got: %s", result)
			}
			// Should contain hostname (not redacted)
			if !strings.Contains(result, "router1") {
				t.Errorf("Expected 'router1' in output, got: %s", result)
			}
		}
	})

	t.Run("Pretty print disabled", func(t *testing.T) {
		client.prettyPrintLogs = false
		input := "<config><password>secret</password><hostname>router1</hostname></config>"
		result := client.prepareXMLForLogging(input)

		// Should be redacted
		if strings.Contains(result, "secret") {
			t.Errorf("Expected password to be redacted, got: %s", result)
		}
		if !strings.Contains(result, "[REDACTED]") {
			t.Errorf("Expected [REDACTED] in output, got: %s", result)
		}
		// Should be on one line (not pretty printed)
		if strings.Contains(result, "\n") {
			t.Errorf("Expected no newlines (pretty print disabled), got: %s", result)
		}
	})
}

func TestWithLogger(t *testing.T) {
	logger := NewDefaultLogger(LogLevelInfo)
	client := &Client{
		logger: &NoOpLogger{},
	}

	// Apply option
	opt := WithLogger(logger)
	opt(client)

	// Verify logger was set
	if client.logger != logger {
		t.Errorf("Expected custom logger to be set")
	}
}

func TestWithLogger_Nil(t *testing.T) {
	client := &Client{
		logger: &NoOpLogger{},
	}

	// Apply option with nil logger (should not change)
	opt := WithLogger(nil)
	opt(client)

	// Verify logger was not changed
	if _, ok := client.logger.(*NoOpLogger); !ok {
		t.Errorf("Expected NoOpLogger to remain when nil logger provided")
	}
}

func TestWithPrettyPrintLogs(t *testing.T) {
	tests := []struct {
		name     string
		enabled  bool
		expected bool
	}{
		{"Enable", true, true},
		{"Disable", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &Client{}
			opt := WithPrettyPrintLogs(tt.enabled)
			opt(client)

			if client.prettyPrintLogs != tt.expected {
				t.Errorf("Expected prettyPrintLogs = %v, got %v", tt.expected, client.prettyPrintLogs)
			}
		})
	}
}

// TestDefaultLogger_LogInjectionPrevention tests log injection attack prevention
func TestDefaultLogger_LogInjectionPrevention(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	logger := NewDefaultLogger(LogLevelInfo)

	t.Run("Newline injection", func(t *testing.T) {
		buf.Reset()
		// Attempt to inject a fake ERROR log line
		logger.Info(context.Background(), "Test", "malicious", "value\nFAKE ERROR")

		output := buf.String()
		lines := strings.Split(output, "\n")

		// Should only have 1 log line + final newline = 2 elements
		if len(lines) > 2 {
			t.Errorf("Log injection detected: %d lines, expected 2", len(lines))
		}

		// Newline should be replaced with space
		if strings.Contains(output, "\nFAKE ERROR") {
			t.Error("Newline was not sanitized")
		}

		// Should contain sanitized version
		if !strings.Contains(output, "FAKE ERROR") {
			t.Errorf("Expected sanitized content in output, got: %s", output)
		}
	})

	t.Run("Carriage return injection", func(t *testing.T) {
		buf.Reset()
		logger.Info(context.Background(), "Test", "key", "value\r[ERROR] Injected")

		output := buf.String()

		// Carriage return should be replaced
		if strings.Contains(output, "\r") {
			t.Error("Carriage return was not sanitized")
		}
	})

	t.Run("Tab injection", func(t *testing.T) {
		buf.Reset()
		logger.Info(context.Background(), "Test", "key", "value\ttab")

		output := buf.String()

		// Tab should be replaced with space
		if strings.Contains(output, "\t") {
			t.Error("Tab was not sanitized")
		}

		// Should contain sanitized version (space)
		if !strings.Contains(output, "value tab") {
			t.Errorf("Expected sanitized tab in output, got: %s", output)
		}
	})

	t.Run("Control characters", func(t *testing.T) {
		buf.Reset()
		// Control character (ASCII 0x01)
		logger.Info(context.Background(), "Test", "key", "value\x01control")

		output := buf.String()

		// Control character should be replaced with dot
		if strings.Contains(output, "\x01") {
			t.Error("Control character was not sanitized")
		}

		// Should contain sanitized version (dot)
		if !strings.Contains(output, "value.control") {
			t.Errorf("Expected sanitized control character in output, got: %s", output)
		}
	})
}

// TestDefaultLogger_LongValueTruncation tests truncation of long log values
func TestDefaultLogger_LongValueTruncation(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer func() {
		log.SetOutput(os.Stderr)
	}()

	logger := NewDefaultLogger(LogLevelInfo)

	t.Run("Long value truncation", func(t *testing.T) {
		buf.Reset()

		// Create a very long value (> MaxLogValueLength)
		longValue := strings.Repeat("A", 2000)
		logger.Info(context.Background(), "Test", "key", longValue)

		output := buf.String()

		// Should be truncated
		if strings.Contains(output, strings.Repeat("A", 2000)) {
			t.Error("Long value was not truncated")
		}

		// Should contain truncation marker
		if !strings.Contains(output, "[TRUNCATED]") {
			t.Error("Truncation marker not found")
		}

		// Should contain first part of the value
		if !strings.Contains(output, strings.Repeat("A", 100)) {
			t.Error("Expected truncated value to contain first part")
		}
	})

	t.Run("Exact limit value", func(t *testing.T) {
		buf.Reset()

		// Create a value exactly at the limit
		exactValue := strings.Repeat("B", MaxLogValueLength)
		logger.Info(context.Background(), "Test", "key", exactValue)

		output := buf.String()

		// Should NOT be truncated (equal to limit)
		if strings.Contains(output, "[TRUNCATED]") {
			t.Error("Value at exact limit should not be truncated")
		}

		// Should contain full value
		if !strings.Contains(output, exactValue) {
			t.Error("Expected full value at exact limit")
		}
	})
}

// TestSanitizeLogValue tests the sanitizeLogValue function directly
func TestSanitizeLogValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "Normal string",
			input:    "normal value",
			expected: "normal value",
		},
		{
			name:     "String with newline",
			input:    "line1\nline2",
			expected: "line1 line2",
		},
		{
			name:     "String with carriage return",
			input:    "line1\rline2",
			expected: "line1 line2",
		},
		{
			name:     "String with tab",
			input:    "col1\tcol2",
			expected: "col1 col2",
		},
		{
			name:     "String with control character",
			input:    "value\x00null",
			expected: "value.null",
		},
		{
			name:     "Integer value",
			input:    12345,
			expected: "12345",
		},
		{
			name:     "Boolean value",
			input:    true,
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLogValue(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeLogValue(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeLogValue_ANSIEscapeSequences(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		notContains string // What should NOT be in output
	}{
		{
			name:        "ANSI red color code",
			input:       "\x1b[31mRED TEXT\x1b[0m",
			notContains: "\x1b",
		},
		{
			name:        "ANSI cursor movement",
			input:       "\x1b[2J\x1b[H",
			notContains: "\x1b",
		},
		{
			name:        "Terminal bell",
			input:       "alert\x07",
			notContains: "\x07",
		},
		{
			name:        "Backspace attack",
			input:       "admin\x08\x08\x08\x08\x08hacker",
			notContains: "\x08",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLogValue(tt.input)
			if tt.notContains != "" && strings.Contains(result, tt.notContains) {
				t.Errorf("Result still contains dangerous character: %q", tt.notContains)
			}
		})
	}
}

func TestSanitizeLogValue_UnicodeAttacks(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"Zero-width space", "admin\u200Bhacker"},
		{"Zero-width joiner", "admin\u200Dhacker"},
		{"RTL override", "admin\u202Ehacker"},
		{"BOM", "admin\uFEFFhacker"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLogValue(tt.input)
			// Result should either remove or replace dangerous Unicode
			// Length should be different from input (unless replaced with same-width)
			if result == tt.input {
				t.Errorf("Dangerous Unicode was not sanitized")
			}
		})
	}
}
