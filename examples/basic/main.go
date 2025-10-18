//nolint:errcheck,gosec // Example code prioritizes readability over error handling
// Package main demonstrates basic go-netconf API usage.
//
// This example shows:
//   - Client creation with options
//   - GetConfig with filters
//   - EditConfig with Body builder
//   - Response parsing with xmldot
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

	// Create client with options
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

	fmt.Printf("Connected to %s (session: %s)\n", host, client.SessionID())

	ctx := context.Background()

	// GetConfig with filter
	fmt.Println("\n=== GetConfig ===")
	filter := netconf.SubtreeFilter("<interfaces/>")
	res, err := client.GetConfig(ctx, "running", filter)
	if err != nil {
		log.Fatalf("GetConfig failed: %v", err)
	}

	// Parse response with xmldot
	if res.Res.Exists() {
		interfaces := res.Res.Get("data.interfaces.interface").Array()
		fmt.Printf("Found %d interfaces\n", len(interfaces))
		for _, iface := range interfaces {
			name := iface.Get("name").String()
			desc := iface.Get("description").String()
			fmt.Printf("  %s: %s\n", name, desc)
		}
	}

	// EditConfig with Body builder
	fmt.Println("\n=== EditConfig ===")
	body := netconf.Body{}.
		Set("config.system.hostname", "NewRouter").
		Set("config.system.domain-name", "example.com")

	config, err := body.String()
	if err != nil {
		log.Fatalf("Body.String failed: %v", err)
	}

	res, err = client.EditConfig(ctx, "candidate", config)
	if err != nil {
		log.Fatalf("EditConfig failed: %v", err)
	}

	if res.OK {
		fmt.Println("Configuration applied to candidate")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
