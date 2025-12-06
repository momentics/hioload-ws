// Package main demonstrates HTTP methods in hioload-ws high-level API.
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

	// Register handlers using HTTP methods (though WebSocket only works with GET)
	// These are useful for API consistency and potential future extensions
	server.GET("/ws/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("WebSocket connection to /ws/users/%s from %s\n", userId, conn.RemoteAddr())

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

	// Example of registering other HTTP methods (for potential API endpoints)
	// This showcases the API consistency similar to Gin framework
	server.POST("/api/users", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("POST connection attempt to /api/users from %s\n", conn.RemoteAddr())
		fmt.Println("Note: This won't work for WebSocket since WebSocket requires GET method for upgrade")
	})

	server.PUT("/api/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		userId := conn.Param("id")
		fmt.Printf("PUT connection attempt to /api/users/%s from %s\n", userId, conn.RemoteAddr())
		fmt.Println("Note: This won't work for WebSocket since WebSocket requires GET method for upgrade")
	})

	// Traditional static route using GET method
	server.GET("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("WebSocket connection to /echo from %s\n", conn.RemoteAddr())

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

	// Register handler with default HandleFunc (still defaults to GET)
	server.HandleFunc("/chat", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("Connection to /chat from %s\n", conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received chat message: %s\n", string(message))

			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Start the server in a goroutine
	go func() {
		log.Printf("Starting server with HTTP methods on :8080")
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