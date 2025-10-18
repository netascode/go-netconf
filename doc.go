// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

// Package netconf provides a simple, fluent API for interacting with network devices
// using the NETCONF protocol (RFC 6241).
//
// The library provides a high-level client interface that handles session management,
// XML manipulation, error handling with automatic retry logic, and thread-safe operations.
//
// # Quick Start
//
// Create a client and perform basic operations:
//
//	client, err := netconf.NewClient(
//	    "192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.Port(830),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Get configuration with filter
//	ctx := context.Background()
//	filter := netconf.SubtreeFilter("<interfaces/>")
//	res, err := client.GetConfig(ctx, "running", filter)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Parse response using xmldot
//	ifName := res.Res.Get("data.interfaces.interface.name").String()
//	fmt.Println("Interface:", ifName)
//
// # XML Manipulation
//
// Use the Body builder for constructing XML configurations:
//
//	config := netconf.Body{}.
//	    Set("interfaces.interface.name", "GigabitEthernet1").
//	    Set("interfaces.interface.description", "WAN Interface").
//	    Set("interfaces.interface.enabled", true).String()
//
//	ctx := context.Background()
//	res, err := client.EditConfig(ctx, "candidate", config)
//
// # Transaction Management
//
// Use the candidate datastore workflow for safe configuration changes:
//
//	ctx := context.Background()
//
//	// Lock candidate datastore
//	if _, err := client.Lock(ctx, "candidate"); err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Unlock(ctx, "candidate")
//
//	// Edit configuration
//	_, err = client.EditConfig(ctx, "candidate", config)
//	if err != nil {
//	    client.Discard(ctx)
//	    log.Fatal(err)
//	}
//
//	// Validate and commit
//	if _, err := client.Validate(ctx, "candidate"); err != nil {
//	    client.Discard(ctx)
//	    log.Fatal(err)
//	}
//	if _, err := client.Commit(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
// # Error Handling
//
// The library automatically retries transient errors with exponential backoff:
//
//	client, err := netconf.NewClient(
//	    "192.168.1.1",
//	    netconf.Username("admin"),
//	    netconf.Password("secret"),
//	    netconf.MaxRetries(5),
//	    netconf.BackoffMinDelay(1*time.Second),
//	    netconf.BackoffMaxDelay(60*time.Second),
//	)
//
// # Thread Safety
//
// Read operations (Get, GetConfig) are thread-safe and can be called concurrently.
// Write operations (EditConfig, CopyConfig, DeleteConfig) are synchronized with a mutex.
//
// # Supported Operations
//
//   - Get: Retrieve configuration and state data
//   - GetConfig: Retrieve configuration from datastore
//   - EditConfig: Modify configuration
//   - CopyConfig: Copy configuration between datastores
//   - DeleteConfig: Delete configuration datastore
//   - Lock/Unlock: Lock and unlock datastores
//   - Commit: Commit candidate to running
//   - Discard: Discard candidate changes
//   - Validate: Validate configuration
//   - RPC: Send custom RPC operations
//
// # References
//
//   - RFC 6241: Network Configuration Protocol (NETCONF)
//   - RFC 6242: Using the NETCONF Protocol over SSH
//   - xmldot: https://github.com/netascode/xmldot
//   - scrapligo: https://github.com/scrapli/scrapligo
package netconf
