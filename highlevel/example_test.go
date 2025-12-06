// Package highlevel provides tests and examples for the high-level API.
package highlevel_test

import (
	"fmt"

	"github.com/momentics/hioload-ws/highlevel"
)

func ExampleNewServer() {
	// Create a new server instance
	s := highlevel.NewServer(":8080")

	// Verify the server was created
	if s != nil {
		fmt.Println("Server created successfully")
	}

	// Output: Server created successfully
}