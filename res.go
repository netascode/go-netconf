// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import "github.com/netascode/xmldot"

// Res represents a NETCONF response wrapping an xmldot.Result for XML parsing
type Res struct {
	// Res is the xmldot.Result containing the parsed XML response
	// Use the fluent Get() method to query the response data using dot notation:
	//   ifName := res.Res.Get("data.interfaces.interface.name").String()
	Res xmldot.Result

	// OK indicates if the operation succeeded (received <ok/> response)
	OK bool

	// Errors contains any NETCONF rpc-error elements from the response
	Errors []ErrorModel

	// MessageID from the rpc-reply message-id attribute
	MessageID string
}
