// Package main demonstrates parameterized routes in hioload-ws high-level API.
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

	// Register handler with path parameters
	server.HandleFunc("/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("Connection to /users/%s from %s\n", userId, conn.RemoteAddr())

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

	// Register handler with multiple parameters
	server.HandleFunc("/users/:id/messages/:messageId", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		messageId := conn.Param("messageId")
		fmt.Printf("Connection to /users/%s/messages/%s from %s\n", userId, messageId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received message %s from user %s: %s\n", messageId, userId, string(message))

			// Echo the message back with user and message IDs
			response := fmt.Sprintf("Got message %s from user %s: %s", messageId, userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register handler for static path as well
	server.HandleFunc("/echo", func(conn *highlevel.Conn) {
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
		log.Printf("Starting parameterized route server on :8080")
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