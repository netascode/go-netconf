// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf

import (
	"fmt"

	"github.com/netascode/xmldot"
)

// Body provides a fluent interface for building XML configurations
// using xmldot for path-based manipulation.
//
// The Body builder tracks errors internally to enable method chaining
// while providing error checking through String() or Err() methods.
//
// Example:
//
//	body := netconf.Body{}.
//	    Set("interfaces.interface.name", "GigabitEthernet1").
//	    Set("interfaces.interface.description", "WAN Interface").
//	    Set("interfaces.interface.enabled", true)
//
//	config, err := body.String()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	ctx := context.Background()
//	res, err := client.EditConfig(ctx, "candidate", config)
type Body struct {
	// str contains the XML string being built
	str string
	// err tracks the first error encountered during building
	err error
}

// NewBody creates a new Body initialized with the provided XML string.
//
// Returns a Body initialized with the provided XML.
func NewBody(xml string) Body {
	return Body{str: xml, err: nil}
}

// String returns the XML string representation of the Body and any error encountered during building
//
// This method returns both the XML string and any error that occurred during the building process.
// If an error occurred during any Set/SetRaw/SetAttr/Delete operation, the error will be returned here.
//
// Example:
//
//	body := netconf.Body{}.Set("config.hostname", "router1")
//	xml, err := body.String()
//	if err != nil {
//	    log.Fatal(err)
//	}
func (body Body) String() (string, error) {
	return body.str, body.err
}

// Err returns any error that occurred during the building process
//
// This method allows checking for errors without retrieving the string value.
//
// Example:
//
//	body := netconf.Body{}.Set("config.hostname", "router1")
//	if err := body.Err(); err != nil {
//	    log.Fatal(err)
//	}
func (body Body) Err() error {
	return body.err
}

// initializeXMLIfEmpty initializes an empty XML string with a minimal root element
// inferred from the first segment of the path.
//
// If xml is empty and path is non-empty, extracts the first path segment
// (before '.', '@', or '[') and creates a root element with that name.
//
// Returns the initialized XML string (original xml if not empty, or new root element).
func initializeXMLIfEmpty(xml, path string) string {
	// If already has content or path is empty, return as-is
	if xml != "" || path == "" {
		return xml
	}

	// Extract first path segment as root element name
	firstDot := 0
	for i, ch := range path {
		if ch == '.' || ch == '@' || ch == '[' {
			firstDot = i
			break
		}
	}
	if firstDot == 0 {
		firstDot = len(path)
	}

	rootName := path[:firstDot]
	return "<" + rootName + "></" + rootName + ">"
}

// Set sets an XML element at the given path to the specified value
//
// The path uses dot notation to navigate the XML structure.
// The value can be any type that xmldot supports (string, int, bool, etc.).
//
// If the body is empty, this will automatically create a minimal root element
// based on the first segment of the path.
//
// If an error occurs, the error is stored and returned by String() or Err().
// Once an error occurs, all subsequent operations are no-ops that preserve the error.
//
// Example:
//
//	body := netconf.Body{}.
//	    Set("config.system.hostname", "router1").
//	    Set("config.system.domain-name", "example.com")
//	xml, err := body.String()
//
// Returns the Body for method chaining.
func (body Body) Set(path string, value any) Body {
	// Short-circuit if already in error state
	if body.err != nil {
		return body
	}

	// Initialize empty XML with root element if needed
	xml := initializeXMLIfEmpty(body.str, path)

	result, err := xmldot.Set(xml, path, value)
	if err != nil {
		// Store error and return body with error state
		return Body{str: body.str, err: fmt.Errorf("Set(%q): %w", path, err)}
	}
	return Body{str: result, err: nil}
}

// SetRaw sets an XML element at the given path to raw XML content
//
// This is useful for inserting pre-formatted XML or complex structures.
//
// If the body is empty, this will automatically create a minimal root element.
//
// If an error occurs, the error is stored and returned by String() or Err().
//
// Example:
//
//	body := netconf.Body{}.
//	    SetRaw("config.interfaces",
//	        "<interface><name>eth0</name><enabled>true</enabled></interface>")
//	xml, err := body.String()
//
// Returns the Body for method chaining.
func (body Body) SetRaw(path, rawXML string) Body {
	// Short-circuit if already in error state
	if body.err != nil {
		return body
	}

	// Initialize empty XML with root element if needed
	xml := initializeXMLIfEmpty(body.str, path)

	result, err := xmldot.SetRaw(xml, path, rawXML)
	if err != nil {
		return Body{str: body.str, err: fmt.Errorf("SetRaw(%q): %w", path, err)}
	}
	return Body{str: result, err: nil}
}

// SetAttr sets an XML attribute at the given path
//
// The value can be any type that xmldot supports (string, int, bool, etc.),
// just like Set().
//
// If an error occurs, the error is stored and returned by String() or Err().
//
// Example:
//
//	body := netconf.Body{}.
//	    Set("config.interface.name", "eth0").
//	    SetAttr("config.interface", "operation", "replace").
//	    SetAttr("config.interface", "mtu", 1500)
//	xml, err := body.String()
//
// Returns the Body for method chaining.
func (body Body) SetAttr(path, attr string, value any) Body {
	// Short-circuit if already in error state
	if body.err != nil {
		return body
	}

	// xmldot uses @ syntax for attributes: "path.@attribute"
	attrPath := path + ".@" + attr
	result, err := xmldot.Set(body.str, attrPath, value)
	if err != nil {
		return Body{str: body.str, err: fmt.Errorf("SetAttr(%q, %q): %w", path, attr, err)}
	}
	return Body{str: result, err: nil}
}

// Delete removes an element at the given path
//
// If an error occurs, the error is stored and returned by String() or Err().
//
// Example:
//
//	body := netconf.Body{}.
//	    Set("config.interface.name", "eth0").
//	    SetAttr("config.interface", "operation", "delete")
//	xml, err := body.String()
//
// Returns the Body for method chaining.
func (body Body) Delete(path string) Body {
	// Short-circuit if already in error state
	if body.err != nil {
		return body
	}

	result, err := xmldot.Delete(body.str, path)
	if err != nil {
		return Body{str: body.str, err: fmt.Errorf("Delete(%q): %w", path, err)}
	}
	return Body{str: result, err: nil}
}

// Res returns the XML string for further processing with xmldot.Get
//
// This allows you to query the built XML using xmldot's Get function.
// If an error occurred during building, this returns an empty string.
// Use Err() or String() to check for errors.
//
// Example:
//
//	body := netconf.Body{}.Set("config.hostname", "router1")
//	if body.Err() == nil {
//	    xml := body.Res()
//	    hostname := xmldot.Get(xml, "config.hostname").String()
//	}
//
// Returns the XML string that can be queried with xmldot.Get.
func (body Body) Res() string {
	// Return empty string if there's an error
	// (caller should check Err() first)
	if body.err != nil {
		return ""
	}
	return body.str
}
