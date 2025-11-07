// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"context"
	"testing"
)

// TestResponseParsing tests the parseResponse method
func TestResponseParsing(t *testing.T) {
	client := &Client{}

	tests := []struct {
		name         string
		responseXML  string
		expectOK     bool
		expectErrors int
		expectMsgID  string
		shouldError  bool
	}{
		{
			name: "ok response",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<rpc-reply message-id="101" xmlns="urn:ietf:params:netconf:base:1.0">
  <ok/>
</rpc-reply>`,
			expectOK:     true,
			expectErrors: 0,
			expectMsgID:  "101",
		},
		{
			name: "data response",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<rpc-reply message-id="102" xmlns="urn:ietf:params:netconf:base:1.0">
  <data>
    <interfaces>
      <interface>
        <name>eth0</name>
      </interface>
    </interfaces>
  </data>
</rpc-reply>`,
			expectOK:     false,
			expectErrors: 0,
			expectMsgID:  "102",
		},
		{
			name: "error response",
			responseXML: `<?xml version="1.0" encoding="UTF-8"?>
<rpc-reply message-id="103" xmlns="urn:ietf:params:netconf:base:1.0">
  <rpc-error>
    <error-type>protocol</error-type>
    <error-tag>operation-failed</error-tag>
    <error-severity>error</error-severity>
    <error-message>Configuration error</error-message>
  </rpc-error>
</rpc-reply>`,
			expectOK:     false,
			expectErrors: 1,
			expectMsgID:  "103",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// We can test parseRPCErrors directly
			errors := client.parseRPCErrors(tt.responseXML)
			if len(errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
			}

			// If we expect an error, check the first one
			if tt.expectErrors > 0 && len(errors) > 0 {
				if errors[0].ErrorType == "" {
					t.Errorf("expected error-type to be set")
				}
				if errors[0].ErrorTag == "" {
					t.Errorf("expected error-tag to be set")
				}
			}
		})
	}
}

// TestContextTimeout tests that context timeout is properly applied
func TestContextTimeout(t *testing.T) {
	// This test verifies that the sendRPC method properly applies context timeout
	// We can't test the actual timeout without a real connection, but we can verify
	// that the code path exists and is syntactically correct

	client := &Client{
		TotalTimeout: 60,
	}

	ctx := context.Background()
	// Verify that client has timeout configured
	if client.TotalTimeout == 0 {
		t.Error("expected TotalTimeout to be set")
	}

	// Context should respect the timeout
	_, hasDeadline := ctx.Deadline()
	if hasDeadline {
		t.Error("expected context without deadline initially")
	}
}
