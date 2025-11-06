// Package main demonstrates candidate datastore workflow with go-netconf.
//
// This example shows:
//   - Lock/Unlock API
//   - EditConfig with candidate
//   - Validate API
//   - Commit/Discard API
//   - Confirmed commit workflow
//
// Usage:
//
//	export NETCONF_HOST=192.168.1.1
//	export NETCONF_USERNAME=admin
//	export NETCONF_PASSWORD=secret
//	go run main.go
//
//nolint:errcheck,gosec // Example code prioritizes readability over error handling
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/netascode/go-netconf"
)

func main() {
	host := getEnv("NETCONF_HOST", "192.168.1.1")
	username := getEnv("NETCONF_USERNAME", "admin")
	password := getEnv("NETCONF_PASSWORD", "secret")

	// Create client (connection established lazily on first operation)
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

	// Open connection explicitly to check capabilities
	if err := client.Open(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Check capability
	if !client.ServerHasCapability("urn:ietf:params:netconf:capability:candidate:1.0") {
		log.Fatal("Server does not support candidate datastore")
	}

	fmt.Printf("Connected to %s\n", host)

	ctx := context.Background()

	// Example 1: Basic workflow
	fmt.Println("\n=== Basic Candidate Workflow ===")
	basicWorkflow(ctx, client)

	// Example 2: Error recovery
	fmt.Println("\n=== Error Recovery ===")
	errorRecovery(ctx, client)

	// Example 3: Confirmed commit
	fmt.Println("\n=== Confirmed Commit ===")
	confirmedCommit(ctx, client)

	fmt.Println("\n=== Examples Complete ===")
}

// basicWorkflow demonstrates Lock → Edit → Validate → Commit → Unlock
func basicWorkflow(ctx context.Context, client *netconf.Client) {
	// Lock
	_, err := client.Lock(ctx, "candidate")
	if err != nil {
		fmt.Printf("Lock failed: %v\n", err)
		return
	}
	defer client.Unlock(ctx, "candidate")

	// Edit
	config := netconf.Body{}.
		Set("config.system.hostname", "Router1").
		Set("config.system.domain-name", "example.com")
	configXML, _ := config.String()

	_, err = client.EditConfig(ctx, "candidate", configXML)
	if err != nil {
		fmt.Printf("EditConfig failed: %v\n", err)
		client.Discard(ctx)
		return
	}

	// Validate
	_, err = client.Validate(ctx, "candidate")
	if err != nil {
		fmt.Printf("Validate failed: %v\n", err)
		client.Discard(ctx)
		return
	}

	// Commit
	_, err = client.Commit(ctx)
	if err != nil {
		fmt.Printf("Commit failed: %v\n", err)
		return
	}

	fmt.Println("Workflow completed successfully")
}

// errorRecovery demonstrates Discard on error
func errorRecovery(ctx context.Context, client *netconf.Client) {
	_, err := client.Lock(ctx, "candidate")
	if err != nil {
		fmt.Printf("Lock failed: %v\n", err)
		return
	}
	defer client.Unlock(ctx, "candidate")

	// Apply multiple configs
	configs := []string{
		`<config><system><hostname>Router2</hostname></system></config>`,
		`<config><system><location>DC1</location></system></config>`,
	}

	for i, cfg := range configs {
		_, err := client.EditConfig(ctx, "candidate", cfg)
		if err != nil {
			fmt.Printf("Config %d failed, discarding...\n", i+1)
			client.Discard(ctx)
			return
		}
	}

	// Validate all
	_, err = client.Validate(ctx, "candidate")
	if err != nil {
		fmt.Println("Validation failed, discarding...")
		client.Discard(ctx)
		return
	}

	// Commit all
	_, err = client.Commit(ctx)
	if err != nil {
		fmt.Printf("Commit failed: %v\n", err)
		return
	}

	fmt.Println("All configs applied successfully")
}

// confirmedCommit demonstrates confirmed commit with timeout
func confirmedCommit(ctx context.Context, client *netconf.Client) {
	if !client.ServerHasCapability("urn:ietf:params:netconf:capability:confirmed-commit:1.0") {
		fmt.Println("Server does not support confirmed commit")
		return
	}

	_, err := client.Lock(ctx, "candidate")
	if err != nil {
		fmt.Printf("Lock failed: %v\n", err)
		return
	}
	defer client.Unlock(ctx, "candidate")

	// Edit
	config := netconf.Body{}.Set("config.system.hostname", "Router3")
	configXML, _ := config.String()
	_, err = client.EditConfig(ctx, "candidate", configXML)
	if err != nil {
		fmt.Printf("EditConfig failed: %v\n", err)
		client.Discard(ctx)
		return
	}

	// Commit with 60s timeout
	fmt.Println("Issuing confirmed commit (60s timeout)...")
	_, err = client.Commit(ctx, netconf.Confirmed(60))
	if err != nil {
		fmt.Printf("Confirmed commit failed: %v\n", err)
		return
	}

	fmt.Println("Configuration applied, verifying...")
	time.Sleep(5 * time.Second)

	// Confirm
	fmt.Println("Confirming commit...")
	_, err = client.Commit(ctx)
	if err != nil {
		fmt.Printf("Commit confirmation failed: %v\n", err)
		return
	}

	fmt.Println("Commit confirmed")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
