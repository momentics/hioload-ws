// Package main demonstrates middleware in hioload-ws high-level API.
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

func main() {
	server := NewServer(":8080")

	// Add logging middleware
	loggingMiddleware := func(next func(*highlevel.Conn)) func(*highlevel.Conn) {
		return func(conn *highlevel.Conn) {
			start := time.Now()
			// Note: In real implementation, you might not have access to the original path in middleware
			// This is a simplified logging example
			log.Printf("Connection started from %s at %v", conn.RemoteAddr(), start)

			// Call the next handler in the chain
			next(conn)

			log.Printf("Connection from %s ended, duration: %v", conn.RemoteAddr(), time.Since(start))
		}
	}

	// Add recovery middleware
	recoveryMiddleware := func(next func(*highlevel.Conn)) func(*highlevel.Conn) {
		return func(conn *highlevel.Conn) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic recovered in handler: %v", r)
				}
			}()
			next(conn)
		}
	}

	// Add authentication middleware example
	authMiddleware := func(next func(*highlevel.Conn)) func(*highlevel.Conn) {
		return func(conn *highlevel.Conn) {
			// In a real application, you would check headers, tokens, etc.
			userId := conn.Param("id")
			if userId != "" {
				log.Printf("User %s authenticated", userId)
			}
			next(conn)
		}
	}

	// Register middleware
	server.Use(loggingMiddleware, recoveryMiddleware)

	// Register handlers with parameterized routes
	server.GET("/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("WebSocket connection to /users/%s from %s\n", userId, conn.RemoteAddr())

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

	// Create API group with its own middleware
	apiV1 := server.Group("/api/v1")
	
	// Add auth middleware to the group
	apiV1.Use(authMiddleware)
	
	apiV1.GET("/protected/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("Protected WebSocket connection to /api/v1/protected/%s from %s\n", userId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received protected message from user %s: %s\n", userId, string(message))

			// Echo the message back with user ID
			response := fmt.Sprintf("Hello user %s, received your protected message: %s", userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register handler for static route
	server.GET("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("Connection to /echo from %s\n", conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received echo message: %s\n", string(message))

			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server with middleware on :8080")
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

// NewServer creates a new high-level server
func NewServer(addr string) *highlevel.Server {
	return highlevel.NewServer(addr)
}