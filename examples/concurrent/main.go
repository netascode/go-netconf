//nolint:errcheck,gosec // Example code prioritizes readability over error handling
// Package main demonstrates concurrent operations with go-netconf.
//
// This example shows:
//   - Thread-safe concurrent read operations
//   - Proper goroutine synchronization with WaitGroup
//   - Error handling in concurrent operations
//   - Rate-limiting with semaphores
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
	"sync"
	"time"

	"github.com/netascode/go-netconf"
)

func main() {
	host := getEnv("NETCONF_HOST", "192.168.1.1")
	username := getEnv("NETCONF_USERNAME", "admin")
	password := getEnv("NETCONF_PASSWORD", "secret")

	// Create client (thread-safe for concurrent reads)
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

	// Example 1: Basic concurrent reads
	fmt.Println("\n=== Concurrent Read Operations ===")
	concurrentReads(ctx, client)

	// Example 2: Concurrent reads with results collection
	fmt.Println("\n=== Concurrent Reads with Results ===")
	concurrentReadsWithResults(ctx, client)

	// Example 3: Rate-limited concurrent operations
	fmt.Println("\n=== Rate-Limited Operations ===")
	rateLimitedOperations(ctx, client)

	fmt.Println("\n=== Examples Complete ===")
}

// concurrentReads demonstrates thread-safe concurrent read operations
func concurrentReads(ctx context.Context, client *netconf.Client) {
	filters := []netconf.Filter{
		netconf.SubtreeFilter("<interfaces/>"),
		netconf.SubtreeFilter("<system/>"),
		netconf.SubtreeFilter("<routing/>"),
		netconf.SubtreeFilter("<protocols/>"),
	}

	start := time.Now()
	var wg sync.WaitGroup

	// Launch concurrent operations
	for i, filter := range filters {
		wg.Add(1)
		go func(index int, f netconf.Filter) {
			defer wg.Done()
			_, err := client.GetConfig(ctx, "running", f)
			if err != nil {
				fmt.Printf("  Operation %d failed: %v\n", index+1, err)
			} else {
				fmt.Printf("  Operation %d completed\n", index+1)
			}
		}(i, filter)
	}

	wg.Wait()
	fmt.Printf("All operations completed in %v\n", time.Since(start))
}

// concurrentReadsWithResults demonstrates collecting results from concurrent operations
func concurrentReadsWithResults(ctx context.Context, client *netconf.Client) {
	type result struct {
		name       string
		interfaces []string
		err        error
	}

	results := make(chan result, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		filter := netconf.SubtreeFilter("<interfaces/>")
		res, err := client.GetConfig(ctx, "running", filter)

		r := result{name: "Interfaces", err: err}

		if err == nil && res.Res.Exists() {
			ifaceArray := res.Res.Get("data.interfaces.interface").Array()
			for _, iface := range ifaceArray {
				name := iface.Get("name").String()
				if name != "" {
					r.interfaces = append(r.interfaces, name)
				}
			}
		}

		results <- r
	}()

	wg.Wait()
	close(results)

	// Process results
	for res := range results {
		if res.err != nil {
			fmt.Printf("%s failed: %v\n", res.name, res.err)
		} else {
			fmt.Printf("%s: found %d interfaces\n", res.name, len(res.interfaces))
		}
	}
}

// rateLimitedOperations demonstrates rate-limiting with semaphore pattern
func rateLimitedOperations(ctx context.Context, client *netconf.Client) {
	maxConcurrent := 3
	semaphore := make(chan struct{}, maxConcurrent)

	queries := []struct {
		name   string
		filter netconf.Filter
	}{
		{"Query 1", netconf.SubtreeFilter("<interfaces/>")},
		{"Query 2", netconf.SubtreeFilter("<system/>")},
		{"Query 3", netconf.SubtreeFilter("<routing/>")},
		{"Query 4", netconf.SubtreeFilter("<protocols/>")},
		{"Query 5", netconf.SubtreeFilter("<interfaces/>")},
		{"Query 6", netconf.SubtreeFilter("<system/>")},
	}

	fmt.Printf("Running %d queries (max %d concurrent)\n", len(queries), maxConcurrent)

	var wg sync.WaitGroup
	start := time.Now()

	for _, query := range queries {
		wg.Add(1)

		go func(q struct {
			name   string
			filter netconf.Filter
		}) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			_, err := client.GetConfig(ctx, "running", q.filter)
			if err != nil {
				fmt.Printf("  %s failed: %v\n", q.name, err)
			} else {
				fmt.Printf("  %s completed\n", q.name)
			}
		}(query)
	}

	wg.Wait()
	fmt.Printf("All operations completed in %v\n", time.Since(start))
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
