package main

import (
	"fmt"
	"log"
	"github.com/momentics/hioload-ws/highlevel"
)

func main() {
	// Connect a client to the echo server
	client, err := highlevel.Dial("ws://localhost:8080/echo")
	if err != nil {
		log.Printf("Failed to connect client: %v", err)
		return
	}
	fmt.Println("Client connected to server")

	// Send a test message
	testMsg := []byte("Hello, Echo Server!")
	fmt.Printf("Client sending: %s\n", string(testMsg))
	err = client.WriteMessage(int(highlevel.BinaryMessage), testMsg)
	if err != nil {
		log.Printf("Client failed to send message: %v", err)
		return
	}

	// Read the echo
	fmt.Println("Client waiting for echo...")
	_, resp, err := client.ReadMessage()
	if err != nil {
		log.Printf("Client failed to read message: %v", err)
		return
	}

	fmt.Printf("Client received echo: %s\n", string(resp))

	// Clean up
	client.Close()
	fmt.Println("Test completed successfully!")
}