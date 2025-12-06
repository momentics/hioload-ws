// Package main demonstrates built-in middleware in hioload-ws high-level API.
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

	// Add built-in middleware
	server.Use(
		highlevel.LoggingMiddleware,
		highlevel.RecoveryMiddleware,
		highlevel.MetricsMiddleware,
	)

	// Register a simple echo handler
	server.GET("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("Echo connection from %s\n", conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message: %s\n", string(message))

			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register a handler with parameters
	server.GET("/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("User %s connection from %s\n", userId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message from user %s: %s\n", userId, string(message))

			// Echo the message back with user ID
			response := fmt.Sprintf("Hello user %s, got your message: %s", userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Demonstrate metrics
	go func() {
		// In a real example, you'd run this periodically or on demand
		// For demo purposes we'll just log metrics initially and before shutdown
	}()

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server with built-in middleware on :8080")
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down server...")

	// Print final metrics
	metrics := highlevel.GetMetrics()
	log.Printf("Final metrics: %v", metrics)

	// Gracefully shutdown the server
	if err := server.Shutdown(); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	fmt.Println("Server stopped")
}