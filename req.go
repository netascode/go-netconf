// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import "time"

// Req represents a NETCONF request with operation details and modifiers
type Req struct {
	// Operation name (e.g., "get", "edit-config", "commit")
	Operation string

	// Request parameters
	Target           string // datastore target (candidate, running, startup)
	Filter           Filter // XML filter for get operations
	Config           string // XML config for edit operations
	DefaultOperation string // default operation for edit-config (merge, replace, none)
	TestOption       string // test-option for edit-config (test-then-set, set, test-only)
	ErrorOption      string // error-option for edit-config (stop-on-error, continue-on-error, rollback-on-error)

	// Confirmed commit parameters (C4)
	ConfirmTimeout int    // timeout in seconds for confirmed commit
	PersistID      string // persist ID for commit operations

	// Request modifiers
	Timeout time.Duration // Custom timeout for this operation
}

// Filter represents a NETCONF filter for get/get-config operations
type Filter struct {
	// Type of filter: "subtree" or "xpath"
	Type string

	// Content is the XML subtree or XPath expression
	Content string
}

// SubtreeFilter creates a subtree filter from XML
//
// Example:
//
//	filter := netconf.SubtreeFilter("<interfaces/>")
//	res, err := client.GetConfig("running", filter)
func SubtreeFilter(xml string) Filter {
	return Filter{
		Type:    "subtree",
		Content: xml,
	}
}

// XPathFilter creates an XPath filter
//
// Example:
//
//	filter := netconf.XPathFilter("/interfaces/interface[name='GigabitEthernet1']")
//	res, err := client.Get(filter)
func XPathFilter(xpath string) Filter {
	return Filter{
		Type:    "xpath",
		Content: xpath,
	}
}

// NoFilter creates an empty filter (retrieves all data)
//
// Example:
//
//	res, err := client.GetConfig("running", netconf.NoFilter())
func NoFilter() Filter {
	return Filter{
		Type:    "",
		Content: "",
	}
}
