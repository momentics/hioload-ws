// Package main demonstrates route groups in hioload-ws high-level API, similar to Gin framework.
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

	// Create API version groups
	v1 := server.Group("/api/v1")
	v2 := server.Group("/api/v2")

	// Register routes under v1 group
	v1.GET("/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("WebSocket connection to /api/v1/users/%s from %s\n", userId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message from user %s (v1): %s\n", userId, string(message))

			// Echo the message back with user ID
			response := fmt.Sprintf("Hello user %s (v1), got your message: %s", userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	v1.GET("/users/:id/messages/:messageId", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		messageId := conn.Param("messageId")
		fmt.Printf("WebSocket connection to /api/v1/users/%s/messages/%s from %s\n", userId, messageId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message %s from user %s (v1): %s\n", messageId, userId, string(message))

			// Echo the message back with user and message IDs
			response := fmt.Sprintf("Got message %s from user %s (v1): %s", messageId, userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register routes under v2 group
	v2.GET("/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("WebSocket connection to /api/v2/users/%s from %s\n", userId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message from user %s (v2): %s\n", userId, string(message))

			// Echo the message back with user ID (v2 version)
			response := fmt.Sprintf("Hello user %s (v2), got your message: %s", userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Create nested groups
	chatGroup := v1.Group("/chat")
	chatGroup.GET("/room/:roomId", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		roomId := conn.Param("roomId")
		fmt.Printf("WebSocket connection to /api/v1/chat/room/%s from %s\n", roomId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received chat message from room %s: %s\n", roomId, string(message))

			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register non-grouped routes as well (for comparison)
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
		log.Printf("Starting server with route groups on :8080")
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