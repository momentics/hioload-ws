// Package main implements a WebSocket client using the highlevel API.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/momentics/hioload-ws/highlevel"
)

func main() {
	// Connect to the echo server
	conn, err := highlevel.DialWithOptions("ws://localhost:8080/echo",
		highlevel.WithClientDialTimeout(5*time.Second),
		highlevel.WithClientReadLimit(1024*1024), // 1MB
		highlevel.WithClientBatchSize(16),
	)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	fmt.Println("Connected to server")

	// Send a text message
	err = conn.WriteMessage(int(highlevel.TextMessage), []byte("Hello, Server!"))
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	// Read the echo response
	messageType, message, err := conn.ReadMessage()
	if err != nil {
		log.Fatalf("Failed to read message: %v", err)
	}

	fmt.Printf("Received: type=%d, data=%s\n", messageType, string(message))

	// Send JSON message
	jsonData := map[string]string{
		"message": "Hello from client",
		"type":    "test",
	}

	err = conn.WriteJSON(jsonData)
	if err != nil {
		log.Fatalf("Failed to send JSON: %v", err)
	}

	// Read JSON response
	var response map[string]interface{}
	err = conn.ReadJSON(&response)
	if err != nil {
		log.Fatalf("Failed to read JSON: %v", err)
	}

	fmt.Printf("Received JSON: %+v\n", response)

	// Demonstrate ping/pong if supported
	err = conn.WriteMessage(int(highlevel.PingMessage), []byte("ping"))
	if err != nil {
		fmt.Printf("Could not send ping: %v\n", err)
	} else {
		fmt.Println("Sent ping message")
	}

	// Wait a bit to see any responses
	time.Sleep(1 * time.Second)

	fmt.Println("Client finished")
}