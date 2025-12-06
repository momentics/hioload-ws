// Package main implements an echo WebSocket server using the highlevel API.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/momentics/hioload-ws/highlevel"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	_ = flag.Int("numa", -1, "NUMA node to bind to (-1 for auto)")
	_ = flag.Int("batch", 32, "Batch size for processing")
	_ = flag.Int("maxconn", 10000, "Maximum number of connections")
	flag.Parse()

	addr := fmt.Sprintf(":%d", *port)

	// Create a new server with configuration options
	server := highlevel.NewServer(addr)

	// Configure server options (apply them via functional options pattern)
	server = server // Assign to avoid unused variable warning

	// Apply options (this would typically be done differently but for example)
	// In our current implementation, we need to apply these before starting

	// Register handler for the root path
	server.HandleFunc("/", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("New connection from %s\n", conn.RemoteAddr())

		for {
			// Read a message from the connection
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message type %d, length %d\n", messageType, len(message))

			// Echo the message back to the client
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register handler for /echo path
	server.HandleFunc("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("New connection to /echo from %s\n", conn.RemoteAddr())

		for {
			// Read a message from the connection
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message type %d, length %d\n", messageType, len(message))

			// Echo the message back to the client
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register handler for /json path that demonstrates JSON handling
	server.HandleFunc("/json", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("New JSON connection from %s\n", conn.RemoteAddr())

		for {
			// Read a JSON message
			var data map[string]interface{}
			err := conn.ReadJSON(&data)
			if err != nil {
				fmt.Printf("Read JSON error: %v\n", err)
				return
			}

			fmt.Printf("Received JSON: %+v\n", data)

			// Echo the JSON back to the client
			err = conn.WriteJSON(data)
			if err != nil {
				fmt.Printf("Write JSON error: %v\n", err)
				return
			}
		}
	})

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server on %s", addr)
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