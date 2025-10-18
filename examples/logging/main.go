//nolint:errcheck,gosec // Example code prioritizes readability over error handling
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

// Package main demonstrates logging configuration in go-netconf.
//
// This example shows:
//   - Default behavior (no logging)
//   - Configuring log levels (Debug, Info, Warn, Error, None)
//   - Using WithLogger() and WithPrettyPrintLogs() options
//   - Automatic sensitive data redaction
//
// Usage:
//
//	export NETCONF_HOST=192.168.1.1
//	export NETCONF_USERNAME=admin
//	export NETCONF_PASSWORD=secret
//	go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/netascode/go-netconf"
)

func main() {
	// Example 1: Default behavior (No logging)
	fmt.Println("=== Example 1: Default Behavior (No Logging) ===")
	client1, err := netconf.NewClient(
		os.Getenv("NETCONF_HOST"),
		netconf.Username(os.Getenv("NETCONF_USERNAME")),
		netconf.Password(os.Getenv("NETCONF_PASSWORD")),
		netconf.InsecureSkipHostKeyVerification(),
	)
	if err != nil {
		log.Printf("Failed to connect (no logging): %v", err)
	} else {
		fmt.Println("Connected successfully (logs are disabled by default)")
		client1.Close() //nolint:errcheck,gosec // Example code
	}

	// Example 2: Enable logging at Info level
	fmt.Println("\n=== Example 2: Info Level Logging ===")
	logger := netconf.NewDefaultLogger(netconf.LogLevelInfo)
	client2, err := netconf.NewClient(
		os.Getenv("NETCONF_HOST"),
		netconf.Username(os.Getenv("NETCONF_USERNAME")),
		netconf.Password(os.Getenv("NETCONF_PASSWORD")),
		netconf.InsecureSkipHostKeyVerification(),
		netconf.WithLogger(logger),
	)
	if err != nil {
		log.Printf("Failed to connect (info logging): %v", err)
	} else {
		fmt.Println("Connected - check logs above for connection info")
		defer client2.Close() //nolint:errcheck // Example code

		// Perform a simple operation
		ctx := context.Background()
		_, err := client2.GetConfig(ctx, "running", netconf.NoFilter())
		if err != nil {
			log.Printf("GetConfig failed: %v", err)
		}
	}

	// Example 3: Enable debug logging with pretty printing disabled
	fmt.Println("\n=== Example 3: Debug Level Logging (No Pretty Print) ===")
	debugLogger := netconf.NewDefaultLogger(netconf.LogLevelDebug)
	client3, err := netconf.NewClient(
		os.Getenv("NETCONF_HOST"),
		netconf.Username(os.Getenv("NETCONF_USERNAME")),
		netconf.Password(os.Getenv("NETCONF_PASSWORD")),
		netconf.InsecureSkipHostKeyVerification(),
		netconf.WithLogger(debugLogger),
		netconf.WithPrettyPrintLogs(false), // Disable XML formatting for performance
	)
	if err != nil {
		log.Printf("Failed to connect (debug logging): %v", err)
	} else {
		fmt.Println("Connected - check logs above for detailed debug info")
		defer client3.Close() //nolint:errcheck // Example code

		// Perform operations to see detailed logging
		ctx := context.Background()
		filter := netconf.SubtreeFilter("<interfaces/>")
		_, err := client3.GetConfig(ctx, "running", filter)
		if err != nil {
			log.Printf("GetConfig with filter failed: %v", err)
		}
	}

	// Example 4: Different log levels
	fmt.Println("\n=== Example 4: Log Level Comparison ===")
	logLevels := []struct {
		name  string
		level netconf.LogLevel
	}{
		{"Debug (most verbose)", netconf.LogLevelDebug},
		{"Info", netconf.LogLevelInfo},
		{"Warn", netconf.LogLevelWarn},
		{"Error (least verbose)", netconf.LogLevelError},
		{"None (logging disabled)", netconf.LogLevelNone},
	}

	for _, ll := range logLevels {
		fmt.Printf("\nLog Level: %s\n", ll.name)
		logger := netconf.NewDefaultLogger(ll.level)

		// Demonstrate different log levels
		logger.Debug("This is a debug message", "key", "value")
		logger.Info("This is an info message", "host", "192.168.1.1")
		logger.Warn("This is a warning message", "attempt", 1)
		logger.Error("This is an error message", "error", "something went wrong")
	}

	// Example 5: Sensitive data redaction
	fmt.Println("\n=== Example 5: Sensitive Data Redaction ===")
	fmt.Println("Demonstrating automatic redaction of sensitive data in logs...")

	// Create client with debug logging to show redaction in action
	redactionLogger := netconf.NewDefaultLogger(netconf.LogLevelDebug)
	client5, err := netconf.NewClient(
		os.Getenv("NETCONF_HOST"),
		netconf.Username(os.Getenv("NETCONF_USERNAME")),
		netconf.Password(os.Getenv("NETCONF_PASSWORD")),
		netconf.InsecureSkipHostKeyVerification(),
		netconf.WithLogger(redactionLogger),
	)
	if err != nil {
		log.Printf("Failed to connect (redaction example): %v", err)
	} else {
		defer client5.Close() //nolint:errcheck // Example code

		// Build configuration with sensitive data in various formats
		// Note: This will likely fail on a real device, but demonstrates redaction
		config := netconf.Body{}.
			Set("config.system.hostname", "SecureRouter").
			Set("config.snmp.community", "secret-community-string").
			Set("config.users.user.name", "admin").
			Set("config.users.user.password", "super-secret-password-123").
			Set("config.api.key", "sk-1234567890abcdef").
			Set("config.credentials.secret", "my-secret-token")

		configXML, err := config.String()
		if err != nil {
			log.Printf("Failed to build config: %v", err)
		} else {
			fmt.Println("\nOriginal config contains sensitive data:")
			fmt.Println("  - SNMP community: secret-community-string")
			fmt.Println("  - User password: super-secret-password-123")
			fmt.Println("  - API key: sk-1234567890abcdef")
			fmt.Println("  - Secret token: my-secret-token")

			fmt.Println("\nAttempting EditConfig with sensitive data...")
			fmt.Println("Look at the DEBUG logs above - sensitive values are replaced with [REDACTED]")

			// This operation will likely fail (invalid config for most devices),
			// but the logging demonstrates redaction
			ctx := context.Background()
			_, err = client5.EditConfig(ctx, "candidate", configXML)
			if err != nil {
				fmt.Printf("\nEditConfig failed as expected: %v\n", err)
				fmt.Println("But notice in the DEBUG logs - all sensitive data was redacted!")
			}

			fmt.Println("\nRedaction patterns automatically protect:")
			fmt.Println("  ✓ Element content: <password>value</password> → <password>[REDACTED]</password>")
			fmt.Println("  ✓ Element content: <secret>value</secret> → <secret>[REDACTED]</secret>")
			fmt.Println("  ✓ Element content: <key>value</key> → <key>[REDACTED]</key>")
			fmt.Println("  ✓ Element content: <community>value</community> → <community>[REDACTED]</community>")
			fmt.Println("  ✓ Attributes: password=\"value\" → password=\"[REDACTED]\"")
			fmt.Println("  ✓ XPath filters: [password=\"value\"] → [password=\"[REDACTED]\"]")
		}
	}
}
