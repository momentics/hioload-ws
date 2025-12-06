// Package main demonstrates how to create custom middleware in hioload-ws high-level API.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/momentics/hioload-ws/highlevel"
)

// CustomAuthMiddleware is an example of user-defined authentication middleware
func CustomAuthMiddleware(next func(*highlevel.Conn)) func(*highlevel.Conn) {
	return func(conn *highlevel.Conn) {
		// In a real application, you would check headers, JWT tokens, etc.
		// Here we just log that authentication is being performed
		log.Printf("[AUTH] Authenticating connection from %s", conn.RemoteAddr())
		
		// You could add context data to the connection here if needed
		// For now, we just pass the connection to the next handler
		next(conn)
	}
}

// CustomRateLimitMiddleware is an example of user-defined rate limiting middleware
func CustomRateLimitMiddleware(next func(*highlevel.Conn)) func(*highlevel.Conn) {
	// In a real implementation, you would track requests from each connection
	// and reject connections that exceed the rate limit
	return func(conn *highlevel.Conn) {
		log.Printf("[RATE_LIMIT] Checking rate limits for %s", conn.RemoteAddr())
		// For this example, we just pass through
		next(conn)
	}
}

// CustomRequestIDMiddleware is an example of user-defined request ID middleware
func CustomRequestIDMiddleware(next func(*highlevel.Conn)) func(*highlevel.Conn) {
	// In a real implementation, you could generate or extract request IDs
	return func(conn *highlevel.Conn) {
		start := time.Now()
		log.Printf("[REQUEST_ID] Processing connection from %s", conn.RemoteAddr())
		
		// Call the next handler in the chain
		next(conn)
		
		duration := time.Since(start)
		log.Printf("[REQUEST_ID] Connection from %s completed in %v", conn.RemoteAddr(), duration)
	}
}

func main() {
	server := highlevel.NewServer(":8080")

	// Add the custom middleware to the server
	server.Use(
		CustomAuthMiddleware,
		CustomRateLimitMiddleware,
		CustomRequestIDMiddleware,
		// Also include the built-in middleware
		highlevel.LoggingMiddleware,
		highlevel.RecoveryMiddleware,
		highlevel.MetricsMiddleware,
	)

	// Register handlers to demonstrate middleware
	server.GET("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("Echo handler called with connection from %s\n", conn.RemoteAddr())

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

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server with custom middleware on :8080")
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