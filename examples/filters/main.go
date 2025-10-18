//nolint:errcheck,gosec // Example code prioritizes readability over error handling
// Package main demonstrates filter usage with go-netconf.
//
// This example shows:
//   - SubtreeFilter() API
//   - XPathFilter() API
//   - NoFilter() API
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

	// Example 1: Basic subtree filter
	fmt.Println("\n=== SubtreeFilter: Basic ===")
	basicSubtreeFilter(ctx, client)

	// Example 2: Specific subtree filter
	fmt.Println("\n=== SubtreeFilter: Specific Selection ===")
	specificSubtreeFilter(ctx, client)

	// Example 3: Nested subtree filter
	fmt.Println("\n=== SubtreeFilter: Nested ===")
	nestedSubtreeFilter(ctx, client)

	// Example 4: XPath filter - basic
	fmt.Println("\n=== XPathFilter: Basic ===")
	basicXPathFilter(ctx, client)

	// Example 5: XPath filter - with predicate
	fmt.Println("\n=== XPathFilter: With Predicate ===")
	xpathWithPredicate(ctx, client)

	// Example 6: No filter
	fmt.Println("\n=== NoFilter ===")
	noFilterExample(ctx, client)

	fmt.Println("\n=== Examples Complete ===")
}

// basicSubtreeFilter demonstrates basic subtree filter
func basicSubtreeFilter(ctx context.Context, client *netconf.Client) {
	filter := netconf.SubtreeFilter("<interfaces/>")
	res, err := client.GetConfig(ctx, "running", filter)
	if err != nil {
		fmt.Printf("GetConfig failed: %v\n", err)
		return
	}

	if res.Res.Exists() {
		interfaces := res.Res.Get("data.interfaces.interface").Array()
		fmt.Printf("Found %d interfaces\n", len(interfaces))
	}
}

// specificSubtreeFilter demonstrates specific element selection
func specificSubtreeFilter(ctx context.Context, client *netconf.Client) {
	filter := netconf.SubtreeFilter(`
		<interfaces>
			<interface>
				<name>GigabitEthernet1</name>
			</interface>
		</interfaces>
	`)

	res, err := client.GetConfig(ctx, "running", filter)
	if err != nil {
		fmt.Printf("GetConfig failed: %v\n", err)
		return
	}

	if res.Res.Exists() {
		iface := res.Res.Get("data.interfaces.interface")
		if iface.Exists() {
			name := iface.Get("name").String()
			fmt.Printf("Interface: %s\n", name)
		}
	}
}

// nestedSubtreeFilter demonstrates nested element selection
func nestedSubtreeFilter(ctx context.Context, client *netconf.Client) {
	filter := netconf.SubtreeFilter(`
		<interfaces>
			<interface>
				<name/>
				<ipv4>
					<address/>
				</ipv4>
			</interface>
		</interfaces>
	`)

	res, err := client.GetConfig(ctx, "running", filter)
	if err != nil {
		fmt.Printf("GetConfig failed: %v\n", err)
		return
	}

	if res.Res.Exists() {
		interfaces := res.Res.Get("data.interfaces.interface").Array()
		fmt.Printf("Found %d interfaces with IP config\n", len(interfaces))
	}
}

// basicXPathFilter demonstrates XPath filter
func basicXPathFilter(ctx context.Context, client *netconf.Client) {
	// Check XPath capability
	if !client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
		fmt.Println("Server does not support XPath filters")
		return
	}

	filter := netconf.XPathFilter("/interfaces/interface")
	res, err := client.GetConfig(ctx, "running", filter)
	if err != nil {
		fmt.Printf("GetConfig failed: %v\n", err)
		return
	}

	if res.Res.Exists() {
		interfaces := res.Res.Get("data.interfaces.interface").Array()
		fmt.Printf("Found %d interfaces\n", len(interfaces))
	}
}

// xpathWithPredicate demonstrates XPath with predicates
func xpathWithPredicate(ctx context.Context, client *netconf.Client) {
	if !client.ServerHasCapability("urn:ietf:params:netconf:capability:xpath:1.0") {
		fmt.Println("Server does not support XPath filters")
		return
	}

	filter := netconf.XPathFilter("/interfaces/interface[name='GigabitEthernet1']")
	res, err := client.GetConfig(ctx, "running", filter)
	if err != nil {
		fmt.Printf("GetConfig failed: %v\n", err)
		return
	}

	if res.Res.Exists() {
		iface := res.Res.Get("data.interfaces.interface")
		if iface.Exists() {
			name := iface.Get("name").String()
			fmt.Printf("Found interface: %s\n", name)
		}
	}
}

// noFilterExample demonstrates retrieving without filters
func noFilterExample(ctx context.Context, client *netconf.Client) {
	filter := netconf.NoFilter()
	res, err := client.GetConfig(ctx, "running", filter)
	if err != nil {
		fmt.Printf("GetConfig failed: %v\n", err)
		return
	}

	if res.Res.Exists() {
		fmt.Println("Retrieved full configuration")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
