// Package main demonstrates parameterized URL routing in hioload-ws high-level API.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/momentics/hioload-ws/highlevel"
)

func main() {
	server := highlevel.NewServer(":8080")

	// Register handler with static path
	server.HandleFunc("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()
		fmt.Printf("Static /echo connection from %s\n", conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error: %v\n", err)
				return
			}

			fmt.Printf("Received static message: %s\n", string(message))

			err = conn.WriteMessage(messageType, message)
			if err != nil {
				fmt.Printf("Write error: %v\n", err)
				return
			}
		}
	})

	// Register handler with single parameter
	server.HandleFunc("/users/:id", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("User-specific connection for user %s from %s\n", userId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error for user %s: %v\n", userId, err)
				return
			}

			fmt.Printf("Received message from user %s: %s\n", userId, string(message))

			response := fmt.Sprintf("Hello user %s (ID: %s), got your message: %s", userId, userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error for user %s: %v\n", userId, err)
				return
			}
		}
	})

	// Register handler with multiple parameters
	server.HandleFunc("/users/:id/messages/:messageId", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		messageId := conn.Param("messageId")
		fmt.Printf("Message-specific connection for user %s, message %s from %s\n", userId, messageId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error for user %s, message %s: %v\n", userId, messageId, err)
				return
			}

			fmt.Printf("Received message %s from user %s: %s\n", messageId, userId, string(message))

			response := fmt.Sprintf("Got message %s from user %s: %s", messageId, userId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error for user %s, message %s: %v\n", userId, messageId, err)
				return
			}
		}
	})

	// Register handler with more complex parameters
	server.HandleFunc("/api/v1/users/:userId/orders/:orderId/items/:itemId", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("userId")
		orderId := conn.Param("orderId")
		itemId := conn.Param("itemId")
		fmt.Printf("Complex routing: user %s, order %s, item %s from %s\n", 
			userId, orderId, itemId, conn.RemoteAddr())

		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				fmt.Printf("Read error for complex route: user %s, order %s, item %s: %v\n", 
					userId, orderId, itemId, err)
				return
			}

			fmt.Printf("Received complex message: user %s, order %s, item %s: %s\n", 
				userId, orderId, itemId, string(message))

			// Convert order ID to integer and calculate
			orderNum, _ := strconv.Atoi(orderId)
			calculated := orderNum * 2

			response := fmt.Sprintf("Processed complex route - user: %s, order: %s (calc: %d), item: %s, message: %s", 
				userId, orderId, calculated, itemId, string(message))
			err = conn.WriteMessage(messageType, []byte(response))
			if err != nil {
				fmt.Printf("Write error for complex route: %v\n", err)
				return
			}
		}
	})

	// Register JSON handler with parameters
	server.HandleFunc("/api/v1/users/:id/profile", func(conn *highlevel.Conn) {
		defer conn.Close()
		
		userId := conn.Param("id")
		fmt.Printf("JSON Profile connection for user %s from %s\n", userId, conn.RemoteAddr())

		for {
			// Read JSON message
			var req map[string]interface{}
			err := conn.ReadJSON(&req)
			if err != nil {
				fmt.Printf("Read JSON error for user %s: %v\n", userId, err)
				return
			}

			fmt.Printf("Received JSON for user %s: %+v\n", userId, req)

			// Create response
			resp := map[string]interface{}{
				"userId": userId,
				"request": req,
				"endpoint": "profile",
				"status": "processed",
			}

			err = conn.WriteJSON(resp)
			if err != nil {
				fmt.Printf("Write JSON error for user %s: %v\n", userId, err)
				return
			}
		}
	})

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server with parameterized routing on :8080")
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	fmt.Println("\nShutting down server...")

	// Gracefully shutdown server
	if err := server.Shutdown(); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	fmt.Println("Server stopped")
}