//nolint:errcheck,gosec // Example code prioritizes readability over error handling
// Package main demonstrates custom RPC operations with go-netconf.
//
// This example shows:
//   - Using RPC() method for custom operations
//   - Parsing RPC responses with xmldot
//   - Handling RPC errors
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
	host := getEnv("NETCONF_HOST", "192.168.1.1")
	username := getEnv("NETCONF_USERNAME", "admin")
	password := getEnv("NETCONF_PASSWORD", "secret")

	// Create client
	client, err := netconf.NewClient(
		host,
		netconf.Username(username),
		netconf.Password(password),
		netconf.Port(830),
	)
	if err != nil {
		log.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	fmt.Printf("Connected to %s\n", host)

	ctx := context.Background()

	// Example 1: Basic custom RPC
	fmt.Println("\n=== Basic Custom RPC ===")
	basicRPC(ctx, client)

	// Example 2: RPC with parameters
	fmt.Println("\n=== RPC with Parameters ===")
	rpcWithParameters(ctx, client)

	// Example 3: RPC error handling
	fmt.Println("\n=== RPC Error Handling ===")
	rpcErrorHandling(ctx, client)

	fmt.Println("\n=== Examples Complete ===")
}

// basicRPC demonstrates a simple custom RPC call
func basicRPC(ctx context.Context, client *netconf.Client) {
	// Custom RPC operation - replace with device-specific RPC
	rpc := `<get-system-info xmlns="http://example.com/ns"/>`

	res, err := client.RPC(ctx, rpc)
	if err != nil {
		fmt.Printf("RPC failed: %v\n", err)
		return
	}

	if res.OK {
		fmt.Println("RPC executed successfully")
		// Parse response if needed
		if res.Res.Exists() {
			fmt.Printf("Response available via res.Res (xmldot.Result)\n")
		}
	}
}

// rpcWithParameters demonstrates RPC with parameters
func rpcWithParameters(ctx context.Context, client *netconf.Client) {
	// Custom RPC with parameters
	rpc := `
		<get-interface-stats xmlns="http://example.com/ns">
			<interface-name>GigabitEthernet1</interface-name>
			<stats-type>detailed</stats-type>
		</get-interface-stats>
	`

	res, err := client.RPC(ctx, rpc)
	if err != nil {
		fmt.Printf("RPC failed: %v\n", err)
		return
	}

	if res.OK && res.Res.Exists() {
		// Parse response using xmldot
		stats := res.Res.Get("interface-stats")
		if stats.Exists() {
			rxBytes := stats.Get("rx-bytes").String()
			txBytes := stats.Get("tx-bytes").String()
			fmt.Printf("RX: %s bytes, TX: %s bytes\n", rxBytes, txBytes)
		}
	}
}

// rpcErrorHandling demonstrates RPC error handling
func rpcErrorHandling(ctx context.Context, client *netconf.Client) {
	// Invalid RPC that will likely fail
	rpc := `<invalid-operation xmlns="http://example.com/ns"/>`

	res, err := client.RPC(ctx, rpc)
	if err != nil {
		fmt.Printf("RPC failed: %v\n", err)
		return
	}

	// Check for RPC-level errors
	if !res.OK && len(res.Errors) > 0 {
		fmt.Println("RPC returned errors:")
		for i, rpcErr := range res.Errors {
			fmt.Printf("  Error %d:\n", i+1)
			fmt.Printf("    Type: %s\n", rpcErr.ErrorType)
			fmt.Printf("    Tag: %s\n", rpcErr.ErrorTag)
			fmt.Printf("    Message: %s\n", rpcErr.ErrorMessage)
			if rpcErr.ErrorInfo != "" {
				fmt.Printf("    Info: %s\n", rpcErr.ErrorInfo)
			}
		}
	} else {
		fmt.Println("RPC executed successfully")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
