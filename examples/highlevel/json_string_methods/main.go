// Package main demonstrates the new JSON and string methods in hioload-ws high-level API.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/momentics/hioload-ws/highlevel"
)

func main() {
	server := highlevel.NewServer(":8080")

	// Register handler that uses the new JSON and string methods
	server.GET("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("Echo connection from %s\n", conn.RemoteAddr())

		for {
			// Use ReadString to read a string message
			message, err := conn.ReadString()
			if err != nil {
				fmt.Printf("Read string error: %v\n", err)
				return
			}

			fmt.Printf("Received string message: %s\n", message)

			// Echo it back using WriteString
			err = conn.WriteString("Echo: " + message)
			if err != nil {
				fmt.Printf("Write string error: %v\n", err)
				return
			}
		}
	})

	// Register handler for JSON communication
	server.GET("/json", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("JSON connection from %s\n", conn.RemoteAddr())

		for {
			// Use ReadJSON to read JSON data
			var jsonData map[string]interface{}
			err := conn.ReadJSON(&jsonData)
			if err != nil {
				fmt.Printf("Read JSON error: %v\n", err)
				return
			}

			fmt.Printf("Received JSON message: %+v\n", jsonData)

			// Echo it back using WriteJSON
			err = conn.WriteJSON(jsonData)
			if err != nil {
				fmt.Printf("Write JSON error: %v\n", err)
				return
			}
		}
	})

	// Register handler that mixes string and JSON
	server.GET("/mixed", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("Mixed connection from %s\n", conn.RemoteAddr())

		for {
			// First, try to read a string command
			command, err := conn.ReadString()
			if err != nil {
				fmt.Printf("Read command error: %v\n", err)
				return
			}

			fmt.Printf("Received command: %s\n", command)

			if command == "json" {
				// If command is "json", read and respond with JSON
				var jsonData map[string]interface{}
				err := conn.ReadJSON(&jsonData)
				if err != nil {
					fmt.Printf("Read JSON error: %v\n", err)
					return
				}

				fmt.Printf("Received JSON data: %+v\n", jsonData)

				// Respond with mixed data
				response := map[string]interface{}{
					"command":   command,
					"received":  jsonData,
					"timestamp": "now",
				}

				err = conn.WriteJSON(response)
				if err != nil {
					fmt.Printf("Write JSON response error: %v\n", err)
					return
				}
			} else {
				// Otherwise, respond with string
				response := fmt.Sprintf("Processed command: %s", command)
				err = conn.WriteString(response)
				if err != nil {
					fmt.Printf("Write string response error: %v\n", err)
					return
				}
			}
		}
	})

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server with JSON and string methods on :8080")
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down server...")

	// Gracefully shutdown the server
	if err := server.Shutdown(); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	fmt.Println("Server stopped")
}