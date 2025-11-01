// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"fmt"
	"testing"
)

// Test constants for filter types and operations (GOCONST)
const (
	testFilterSubtree = "subtree"
	testFilterXPath   = "xpath"
	testOpGet         = "get"
	testOpEditConfig  = "edit-config"
	testOpCommit      = "commit"
)

// TestSubtreeFilter tests the SubtreeFilter constructor
func TestSubtreeFilter(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		expected Filter
	}{
		{
			name: "simple subtree filter",
			xml:  "<interfaces/>",
			expected: Filter{
				Type:    testFilterSubtree,
				Content: "<interfaces/>",
			},
		},
		{
			name: "complex subtree filter",
			xml:  "<interfaces><interface><name>eth0</name></interface></interfaces>",
			expected: Filter{
				Type:    testFilterSubtree,
				Content: "<interfaces><interface><name>eth0</name></interface></interfaces>",
			},
		},
		{
			name: "empty subtree filter",
			xml:  "",
			expected: Filter{
				Type:    testFilterSubtree,
				Content: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := SubtreeFilter(tt.xml)
			if filter.Type != tt.expected.Type {
				t.Errorf("expected Type %q, got %q", tt.expected.Type, filter.Type)
			}
			if filter.Content != tt.expected.Content {
				t.Errorf("expected Content %q, got %q", tt.expected.Content, filter.Content)
			}
		})
	}
}

// TestXPathFilter tests the XPathFilter constructor
func TestXPathFilter(t *testing.T) {
	tests := []struct {
		name     string
		xpath    string
		expected Filter
	}{
		{
			name:  "simple xpath filter",
			xpath: "/interfaces/interface",
			expected: Filter{
				Type:    testFilterXPath,
				Content: "/interfaces/interface",
			},
		},
		{
			name:  "xpath with predicate",
			xpath: "/interfaces/interface[name='eth0']",
			expected: Filter{
				Type:    testFilterXPath,
				Content: "/interfaces/interface[name='eth0']",
			},
		},
		{
			name:  "complex xpath",
			xpath: "/config/system/*[contains(name(), 'interface')]",
			expected: Filter{
				Type:    testFilterXPath,
				Content: "/config/system/*[contains(name(), 'interface')]",
			},
		},
		{
			name:  "empty xpath filter",
			xpath: "",
			expected: Filter{
				Type:    testFilterXPath,
				Content: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := XPathFilter(tt.xpath)
			if filter.Type != tt.expected.Type {
				t.Errorf("expected Type %q, got %q", tt.expected.Type, filter.Type)
			}
			if filter.Content != tt.expected.Content {
				t.Errorf("expected Content %q, got %q", tt.expected.Content, filter.Content)
			}
		})
	}
}

// TestNoFilter tests the NoFilter constructor
func TestNoFilter(t *testing.T) {
	filter := NoFilter()

	if filter.Type != "" {
		t.Errorf("expected empty Type, got %q", filter.Type)
	}
	if filter.Content != "" {
		t.Errorf("expected empty Content, got %q", filter.Content)
	}
}

// TestFilterStructure tests the Filter struct directly
func TestFilterStructure(t *testing.T) {
	t.Run("subtree filter fields", func(t *testing.T) {
		filter := Filter{
			Type:    testFilterSubtree,
			Content: "<test/>",
		}

		if filter.Type != testFilterSubtree {
			t.Errorf("expected Type 'subtree', got %q", filter.Type)
		}
		if filter.Content != "<test/>" {
			t.Errorf("expected Content '<test/>', got %q", filter.Content)
		}
	})

	t.Run("xpath filter fields", func(t *testing.T) {
		filter := Filter{
			Type:    testFilterXPath,
			Content: "/test/path",
		}

		if filter.Type != testFilterXPath {
			t.Errorf("expected Type 'xpath', got %q", filter.Type)
		}
		if filter.Content != "/test/path" {
			t.Errorf("expected Content '/test/path', got %q", filter.Content)
		}
	})

	t.Run("empty filter fields", func(t *testing.T) {
		filter := Filter{}

		if filter.Type != "" {
			t.Errorf("expected empty Type, got %q", filter.Type)
		}
		if filter.Content != "" {
			t.Errorf("expected empty Content, got %q", filter.Content)
		}
	})
}

// TestReqStructure tests the Req struct directly
func TestReqStructure(t *testing.T) {
	t.Run("get request", func(t *testing.T) {
		req := Req{
			Operation: testOpGet,
			Filter: Filter{
				Type:    testFilterSubtree,
				Content: "<interfaces/>",
			},
		}

		if req.Operation != testOpGet {
			t.Errorf("expected Operation 'get', got %q", req.Operation)
		}
		if req.Filter.Type != testFilterSubtree {
			t.Errorf("expected Filter.Type 'subtree', got %q", req.Filter.Type)
		}
	})

	t.Run("edit-config request", func(t *testing.T) {
		req := Req{
			Operation:        testOpEditConfig,
			Target:           "candidate",
			Config:           "<config><hostname>router1</hostname></config>",
			DefaultOperation: "merge",
		}

		if req.Operation != testOpEditConfig {
			t.Errorf("expected Operation 'edit-config', got %q", req.Operation)
		}
		if req.Target != "candidate" {
			t.Errorf("expected Target 'candidate', got %q", req.Target)
		}
		if req.DefaultOperation != "merge" {
			t.Errorf("expected DefaultOp 'merge', got %q", req.DefaultOperation)
		}
	})

	t.Run("commit request with confirmed", func(t *testing.T) {
		req := Req{
			Operation:      testOpCommit,
			ConfirmTimeout: 60,
			PersistID:      "test-123",
		}

		if req.Operation != testOpCommit {
			t.Errorf("expected Operation 'commit', got %q", req.Operation)
		}
		if req.ConfirmTimeout != 60 {
			t.Errorf("expected ConfirmTimeout 60, got %d", req.ConfirmTimeout)
		}
		if req.PersistID != "test-123" {
			t.Errorf("expected PersistID 'test-123', got %q", req.PersistID)
		}
	})
}

// TestFilterComparisonEquality tests filter equality
func TestFilterComparisonEquality(t *testing.T) {
	f1 := SubtreeFilter("<test/>")
	f2 := SubtreeFilter("<test/>")

	if f1.Type != f2.Type || f1.Content != f2.Content {
		t.Error("expected identical subtree filters to be equal")
	}

	f3 := XPathFilter("/test")
	f4 := XPathFilter("/test")

	if f3.Type != f4.Type || f3.Content != f4.Content {
		t.Error("expected identical xpath filters to be equal")
	}

	f5 := NoFilter()
	f6 := NoFilter()

	if f5.Type != f6.Type || f5.Content != f6.Content {
		t.Error("expected identical empty filters to be equal")
	}
}

// TestFilterComparisonInequality tests filter inequality
func TestFilterComparisonInequality(t *testing.T) {
	f1 := SubtreeFilter("<test/>")
	f2 := SubtreeFilter("<other/>")

	if f1.Content == f2.Content {
		t.Error("expected different subtree filters to be unequal")
	}

	f3 := SubtreeFilter("<test/>")
	f4 := XPathFilter("<test/>")

	if f3.Type == f4.Type {
		t.Error("expected subtree and xpath filters to have different types")
	}

	f5 := XPathFilter("/test")
	f6 := NoFilter()

	if f5.Type == f6.Type {
		t.Error("expected xpath and empty filters to be different")
	}
}

// TestFilterWithSpecialCharacters tests filters with special characters
func TestFilterWithSpecialCharacters(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"with quotes", `<test attr="value"/>`},
		{"with apostrophes", `<test attr='value'/>`},
		{"with ampersand", `<test>&amp;</test>`},
		{"with less than", `<test>&lt;</test>`},
		{"with greater than", `<test>&gt;</test>`},
		{"with newlines", "<test>\n  <inner/>\n</test>"},
		{"with unicode", `<test>测试</test>`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := SubtreeFilter(tt.content)
			if filter.Content != tt.content {
				t.Errorf("expected content to be preserved, got %q", filter.Content)
			}
		})
	}
}

// TestXPathWithSpecialSyntax tests XPath filters with various XPath syntax
func TestXPathWithSpecialSyntax(t *testing.T) {
	tests := []struct {
		name  string
		xpath string
	}{
		{"absolute path", "/interfaces/interface"},
		{"relative path", "interface/name"},
		{"wildcard", "/interfaces/*"},
		{"predicate with number", "/interface[1]"},
		{"predicate with comparison", "/interface[id>100]"},
		{"predicate with string", `/interface[name="eth0"]`},
		{"multiple predicates", "/interface[name='eth0'][enabled='true']"},
		{"descendant axis", "//interface"},
		{"attribute selection", "/interface/@name"},
		{"function call", "/interface[contains(name,'eth')]"},
		{"parent axis", "/interface/.."},
		{"self axis", "/interface/."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := XPathFilter(tt.xpath)
			if filter.Type != testFilterXPath {
				t.Errorf("expected Type 'xpath', got %q", filter.Type)
			}
			if filter.Content != tt.xpath {
				t.Errorf("expected Content %q, got %q", tt.xpath, filter.Content)
			}
		})
	}
}

// TestReqZeroValues tests Req with zero values
func TestReqZeroValues(t *testing.T) {
	req := Req{}

	if req.Operation != "" {
		t.Errorf("expected empty Operation, got %q", req.Operation)
	}
	if req.Target != "" {
		t.Errorf("expected empty Target, got %q", req.Target)
	}
	if req.Config != "" {
		t.Errorf("expected empty Config, got %q", req.Config)
	}
	if req.ConfirmTimeout != 0 {
		t.Errorf("expected ConfirmTimeout 0, got %d", req.ConfirmTimeout)
	}
}

// ExampleSubtreeFilter demonstrates creating a subtree filter.
func ExampleSubtreeFilter() {
	// Create a simple subtree filter for interfaces
	filter := SubtreeFilter("<interfaces/>")

	fmt.Printf("Filter type: %s\n", filter.Type)
	fmt.Printf("Has content: %t\n", filter.Content != "")
	// Output: Filter type: subtree
	// Has content: true
}

// ExampleXPathFilter demonstrates creating an XPath filter.
func ExampleXPathFilter() {
	// Create an XPath filter to select a specific interface
	filter := XPathFilter("/interfaces/interface[name='GigabitEthernet1']")

	fmt.Printf("Filter type: %s\n", filter.Type)
	fmt.Printf("Has content: %t\n", filter.Content != "")
	// Output: Filter type: xpath
	// Has content: true
}
