// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"fmt"
	"strings"
	"testing"
)

func TestBodySet(t *testing.T) {
	t.Run("single set operation", func(t *testing.T) {
		body := Body{}.
			Set("config.hostname", "router1")

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "router1") {
			t.Errorf("expected XML to contain 'router1', got: %s", xml)
		}
		if !strings.Contains(xml, "<hostname>") {
			t.Errorf("expected XML to contain '<hostname>', got: %s", xml)
		}
	})

	t.Run("multiple set operations", func(t *testing.T) {
		body := Body{}.
			Set("config.hostname", "router1").
			Set("config.domain", "example.com")

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "router1") {
			t.Errorf("expected XML to contain 'router1', got: %s", xml)
		}
		if !strings.Contains(xml, "example.com") {
			t.Errorf("expected XML to contain 'example.com', got: %s", xml)
		}
	})

	t.Run("set with different types", func(t *testing.T) {
		body := Body{}.
			Set("config.port", 8080).
			Set("config.enabled", true)

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "8080") {
			t.Errorf("expected XML to contain '8080', got: %s", xml)
		}
		if !strings.Contains(xml, "true") {
			t.Errorf("expected XML to contain 'true', got: %s", xml)
		}
	})
}

func TestBodySetAttr(t *testing.T) {
	t.Run("set attribute on element", func(t *testing.T) {
		body := Body{}.
			Set("config.interface.name", "eth0").
			SetAttr("config.interface", "operation", "merge")

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "eth0") {
			t.Errorf("expected XML to contain 'eth0', got: %s", xml)
		}
		if !strings.Contains(xml, `operation="merge"`) {
			t.Errorf("expected XML to contain operation attribute, got: %s", xml)
		}
	})

	t.Run("set attribute with different types", func(t *testing.T) {
		body := Body{}.
			Set("config.interface.name", "eth0").
			SetAttr("config.interface", "mtu", 1500).
			SetAttr("config.interface", "enabled", true)

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, `mtu="1500"`) {
			t.Errorf("expected XML to contain mtu attribute with int value, got: %s", xml)
		}
		if !strings.Contains(xml, `enabled="true"`) {
			t.Errorf("expected XML to contain enabled attribute with bool value, got: %s", xml)
		}
	})
}

func TestBodySetRaw(t *testing.T) {
	t.Run("insert raw XML", func(t *testing.T) {
		rawXML := "<interface><name>eth0</name><enabled>true</enabled></interface>"
		body := Body{}.
			SetRaw("config.interfaces", rawXML)

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "<name>eth0</name>") {
			t.Errorf("expected XML to contain raw content, got: %s", xml)
		}
	})
}

func TestBodyDelete(t *testing.T) {
	t.Run("delete element", func(t *testing.T) {
		body := Body{}.
			Set("config.hostname", "router1").
			Set("config.domain", "example.com").
			Delete("config.domain")

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "router1") {
			t.Errorf("expected XML to contain 'router1', got: %s", xml)
		}
		if strings.Contains(xml, "example.com") {
			t.Errorf("did not expect XML to contain 'example.com', got: %s", xml)
		}
	})
}

func TestBodyRes(t *testing.T) {
	t.Run("inspect built XML", func(t *testing.T) {
		body := Body{}.
			Set("config.hostname", "router1")

		if body.Err() != nil {
			t.Fatalf("unexpected error: %v", body.Err())
		}

		xml := body.Res()
		if !strings.Contains(xml, "router1") {
			t.Errorf("expected XML to contain 'router1', got: %s", xml)
		}
	})
}

func TestBodyMethodChaining(t *testing.T) {
	t.Run("complex chained operations", func(t *testing.T) {
		body := Body{}.
			Set("config.system.hostname", "router1").
			Set("config.system.domain", "example.com").
			Set("config.interface.name", "eth0").
			SetAttr("config.interface", "operation", "merge").
			Set("config.interface.enabled", true)

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "router1") {
			t.Errorf("expected XML to contain 'router1'")
		}
		if !strings.Contains(xml, "example.com") {
			t.Errorf("expected XML to contain 'example.com'")
		}
		if !strings.Contains(xml, "eth0") {
			t.Errorf("expected XML to contain 'eth0'")
		}
		if !strings.Contains(xml, "true") {
			t.Errorf("expected XML to contain 'true'")
		}
	})
}

func TestBodyEmptyInitialization(t *testing.T) {
	t.Run("start with empty body", func(t *testing.T) {
		// Empty body should work fine
		body := Body{}
		body = body.Set("config.test", "value")

		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(xml, "value") {
			t.Errorf("expected XML to contain 'value', got: %s", xml)
		}
	})
}

// Error path tests

func TestBodySetError(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		body := Body{}.Set("", "value")
		_, err := body.String()
		if err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("invalid path", func(t *testing.T) {
		body := Body{}.Set("..invalid", "value")
		_, err := body.String()
		if err == nil {
			t.Error("expected error for invalid path")
		}
	})

	t.Run("error propagation", func(t *testing.T) {
		body := Body{}.
			Set("valid", "works").
			Set("", "fails").       // Should error
			Set("after", "skipped") // Should skip

		_, err := body.String()
		if err == nil {
			t.Error("expected error to propagate")
		}
		// Verify error message contains the failing operation
		if !strings.Contains(err.Error(), "Set") {
			t.Errorf("expected error message to mention 'Set', got: %v", err)
		}
	})

	t.Run("check Err method", func(t *testing.T) {
		body := Body{}.Set("", "invalid")
		if body.Err() == nil {
			t.Error("expected Err() to return error")
		}
	})

	t.Run("Res returns empty on error", func(t *testing.T) {
		body := Body{}.Set("", "invalid")
		res := body.Res()
		if res != "" {
			t.Errorf("expected Res() to return empty string on error, got: %s", res)
		}
	})
}

func TestBodySetRawXXE(t *testing.T) {
	t.Run("XXE injection attempt", func(t *testing.T) {
		xxe := `<!DOCTYPE foo [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><data>&xxe;</data>`
		body := Body{}.SetRaw("config", xxe)
		_, err := body.String()
		if err == nil {
			t.Error("expected error for XXE attempt")
		}
		// xmldot provides XXE protection by rejecting DOCTYPE declarations
		if !strings.Contains(err.Error(), "XXE") && !strings.Contains(err.Error(), "ENTITY") && !strings.Contains(err.Error(), "DOCTYPE") {
			t.Errorf("expected XXE-related error, got: %v", err)
		}
	})

	t.Run("entity expansion attempt", func(t *testing.T) {
		entity := `<!ENTITY a "test"><data>&a;</data>`
		body := Body{}.SetRaw("config", entity)
		_, err := body.String()
		if err == nil {
			t.Error("expected error for entity declaration")
		}
	})

	t.Run("valid XML should work", func(t *testing.T) {
		validXML := `<interface><name>eth0</name></interface>`
		body := Body{}.SetRaw("config", validXML)
		xml, err := body.String()
		if err != nil {
			t.Fatalf("unexpected error for valid XML: %v", err)
		}
		if !strings.Contains(xml, "eth0") {
			t.Errorf("expected XML to contain 'eth0', got: %s", xml)
		}
	})
}

func TestBodySetAttrError(t *testing.T) {
	t.Run("SetAttr on invalid path", func(t *testing.T) {
		body := Body{}.
			Set("config.interface", "test").
			SetAttr("invalid..path", "attr", "value")

		_, err := body.String()
		if err == nil {
			t.Error("expected error for invalid path in SetAttr")
		}
	})

	t.Run("SetAttr error propagation", func(t *testing.T) {
		body := Body{}.
			Set("config.test", "value").
			SetAttr("", "attr", "value"). // Should error
			Set("after", "skipped")       // Should skip

		_, err := body.String()
		if err == nil {
			t.Error("expected error to propagate from SetAttr")
		}
	})
}

func TestBodyDeleteError(t *testing.T) {
	t.Run("Delete nonexistent path", func(t *testing.T) {
		// Note: xmldot.Delete() doesn't error for nonexistent paths
		// It just returns the original XML unchanged
		body := Body{}.
			Set("config.test", "value").
			Delete("config.nonexistent")

		xml, err := body.String()
		if err != nil {
			t.Errorf("unexpected error for deleting nonexistent path: %v", err)
		}
		// Verify original content still exists
		if !strings.Contains(xml, "value") {
			t.Errorf("expected XML to still contain 'value', got: %s", xml)
		}
	})

	t.Run("Delete error propagation", func(t *testing.T) {
		body := Body{}.
			Set("config.test", "value").
			Delete("").             // Should error (empty path)
			Set("after", "skipped") // Should skip

		_, err := body.String()
		if err == nil {
			t.Error("expected error to propagate from Delete")
		}
	})
}

func TestBodyErrorImmutability(t *testing.T) {
	t.Run("error state is immutable", func(t *testing.T) {
		// Create body with error
		body1 := Body{}.Set("", "invalid")

		// Try to continue using it
		body2 := body1.Set("valid", "value")

		// Both should have error
		if body1.Err() == nil {
			t.Error("expected body1 to have error")
		}
		if body2.Err() == nil {
			t.Error("expected body2 to have error")
		}

		// Verify error is the same (first error preserved)
		if body1.Err().Error() != body2.Err().Error() {
			t.Error("expected same error in both bodies")
		}
	})
}

// Benchmark tests for Body operations

func BenchmarkBodySet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		body := Body{}.
			Set("config.hostname", "router1").
			Set("config.domain", "example.com").
			Set("config.port", 8080)
		_ = body.Res()
	}
}

func BenchmarkBodySetComplex(b *testing.B) {
	for i := 0; i < b.N; i++ {
		body := Body{}.
			Set("config.system.hostname", "router1").
			Set("config.system.domain", "example.com").
			Set("config.system.port", 8080).
			Set("config.interface.name", "eth0").
			Set("config.interface.mtu", 1500).
			Set("config.interface.enabled", true).
			Set("config.interface.speed", "1000").
			Set("config.interface.duplex", "full")
		_ = body.Res()
	}
}

func BenchmarkBodySetAttr(b *testing.B) {
	for i := 0; i < b.N; i++ {
		body := Body{}.
			Set("config.interface.name", "eth0").
			SetAttr("config.interface", "operation", "merge").
			Set("config.interface.enabled", true)
		_ = body.Res()
	}
}

func BenchmarkBodySetRaw(b *testing.B) {
	rawXML := "<interface><name>eth0</name><enabled>true</enabled><mtu>1500</mtu></interface>"
	for i := 0; i < b.N; i++ {
		body := Body{}.
			SetRaw("config.interfaces", rawXML)
		_ = body.Res()
	}
}

func BenchmarkBodyDelete(b *testing.B) {
	for i := 0; i < b.N; i++ {
		body := Body{}.
			Set("config.hostname", "router1").
			Set("config.domain", "example.com").
			Set("config.port", 8080).
			Delete("config.domain")
		_ = body.Res()
	}
}

func BenchmarkBodyString(b *testing.B) {
	body := Body{}.
		Set("config.hostname", "router1").
		Set("config.domain", "example.com").
		Set("config.port", 8080)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = body.String() //nolint:errcheck // Benchmark only measures performance
	}
}

func BenchmarkBodyChaining(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Body{}.
			Set("config.a", "1").
			Set("config.b", "2").
			Set("config.c", "3").
			Set("config.d", "4").
			Set("config.e", "5").
			Res()
	}
}

// ExampleBody_Set demonstrates the fluent Body builder API for creating XML configurations.
func ExampleBody_Set() {
	// Build configuration using fluent API
	body := Body{}.
		Set("config.system.hostname", "Router1").
		Set("config.system.domain-name", "example.com").
		Set("config.interfaces.interface.name", "GigabitEthernet1").
		Set("config.interfaces.interface.enabled", true)

	// Get the XML string
	xml, err := body.String()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Println("Configuration built successfully")
	fmt.Printf("XML length: %d bytes\n", len(xml))
	// Output: Configuration built successfully
	// XML length: 200 bytes
}
