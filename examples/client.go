package main

import (
	"context"
	"log"
	"time"

	"zeusync/internal/core/protocol"
)

func main() {
	// 1. Create a QUIC transport.
	transport := protocol.NewQuicTransport()

	// 2. Dial the server.
	addr := "localhost:8080"
	conn, err := transport.Dial(context.Background(), addr)
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}

	// 3. Create a codec and a client.
	codec := &protocol.JSONCodec{}
	client := protocol.NewDefaultClient("my-client", conn, codec)
	log.Printf("Connected to server as %s", client.ID())

	// 4. Start a goroutine to listen for incoming messages.
	go func() {
		for {
			msg, err := client.Receive(client.Context())
			if err != nil {
				log.Printf("Failed to receive message: %v", err)
				return
			}

			switch msg.Type() {
			case "echo_response":
				log.Printf("Received echo: %s", string(msg.Payload()))
			case "world_update":
				log.Printf("Received world update: %s", string(msg.Payload()))
			default:
				log.Printf("Received unknown message type: %s", msg.Type())
			}
		}
	}()

	// 5. Send a message to the server every 2 seconds.
	for {
		time.Sleep(2 * time.Second)
		msg := protocol.NewMessage("ping", []byte("Hello from client!"))
		if err := client.Send(msg); err != nil {
			log.Printf("Failed to send message: %v", err)
		} else {
			log.Printf("Sent message: %s", string(msg.Payload()))
		}
	}
}
