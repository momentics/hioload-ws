package main

import (
	"fmt"
	"log"
	"time"

	"github.com/momentics/hioload-ws/highlevel"
)

func main() {
	// Create echo server
	server := highlevel.NewServer(":8080")

	// Define echo handler
	server.HandleFunc("/echo", func(c *highlevel.Conn) {
		fmt.Println("Server: Client connected")
		defer c.Close() // Ensure connection is closed when handler returns

		// Echo back messages
		for {
			fmt.Println("Server: Waiting for message...")
			msgType, data, err := c.ReadMessage()
			if err != nil {
				fmt.Printf("Server: Read error: %v\n", err)
				return // Connection closed or error
			}
			fmt.Printf("Server: Received: %s\n", string(data))

			// Echo back
			err = c.WriteMessage(msgType, data)
			if err != nil {
				fmt.Printf("Server: Write error: %v\n", err)
				return
			}
			fmt.Printf("Server: Echoed: %s\n", string(data))
		}
	})

	// Start server in background
	go func() {
		fmt.Println("Starting server...")
		if err := server.ListenAndServe(); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// Wait a bit for server to start
	fmt.Println("Waiting for server to start...")
	time.Sleep(500 * time.Millisecond)

	fmt.Println("Connecting client...")
	// Connect a client
	client, err := highlevel.Dial("ws://localhost:8080/echo")
	if err != nil {
		log.Fatalf("Failed to connect client: %v", err)
	}
	fmt.Println("Client connected")

	// Send a test message
	testMsg := []byte("Hello, Server!")
	fmt.Printf("Client: Sending: %s\n", string(testMsg))
	err = client.WriteMessage(int(highlevel.BinaryMessage), testMsg)
	if err != nil {
		log.Fatalf("Client: Failed to send message: %v", err)
	}
	fmt.Println("Client: Message sent")

	// Read the echo
	fmt.Println("Client: Waiting for echo...")
	_, resp, err := client.ReadMessage()
	if err != nil {
		log.Fatalf("Client: Failed to read message: %v", err)
	}

	fmt.Printf("Client: Received echo: %s\n", string(resp))

	// Clean up
	client.Close()
	server.Shutdown()

	fmt.Println("Test completed successfully!")
}