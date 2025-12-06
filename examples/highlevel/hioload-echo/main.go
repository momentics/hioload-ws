package main

import (
	"flag"
	"log"
	"time"

	"github.com/momentics/hioload-ws/highlevel"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	flag.Parse()

	server := highlevel.NewServer(*addr)

	// Register a simple echo handler
	server.HandleFunc("/echo", func(conn *highlevel.Conn) {
		defer conn.Close()

		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				log.Println("Read error:", err)
				break
			}

			log.Printf("Received %d bytes", len(message))

			err = conn.WriteMessage(mt, message)
			if err != nil {
				log.Println("Write error:", err)
				break
			}
		}
	})

	// Register a JSON handler
	server.HandleFunc("/json", func(conn *highlevel.Conn) {
		defer conn.Close()

		var request map[string]interface{}
		if err := conn.ReadJSON(&request); err != nil {
			log.Println("Read JSON error:", err)
			return
		}

		response := map[string]interface{}{
			"received": request,
			"timestamp": time.Now().Unix(),
		}

		if err := conn.WriteJSON(response); err != nil {
			log.Println("Write JSON error:", err)
			return
		}
	})

	log.Printf("Starting server on %s", *addr)

	// Start the server in a goroutine so we can handle graceful shutdown
	go func() {
		if err := server.ListenAndServe(); err != nil {
			log.Fatal("Server error:", err)
		}
	}()

	// Wait indefinitely for now
	select {}
}