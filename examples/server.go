package main

import (
	"context"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"log"
	"time"
)

func main() {
	// 1. Create a codec for message serialization.
	codec := &protocol.JSONCodec{}

	// 2. Create the main engine.
	engine := protocol.NewEngine(codec)

	// 3. Define connection and disconnection handlers.
	engine.OnConnect(func(client protocol.Client) {
		log.Printf("Client connected: %s", client.ID())

		// 4. Create scopes and groups for the client.
		playerScope := client.Scope("player-state")
		worldEventsGroup := engine.Group("world-events")

		// 5. Add the client's scope to the group.
		//_ = worldEventsGroup.Add(playerScope)
		log.Printf("Scope '%s' for client '%s' added to group '%s'", playerScope.ID(), client.ID(), worldEventsGroup.ID())

		// 6. Handle incoming messages from the client in a separate goroutine.
		go func() {
			for {
				msg, err := client.Receive(client.Context())
				if err != nil {
					// The client has disconnected.
					break
				}
				log.Printf("Received message from %s: Type=%s, Payload=%s", client.ID(), msg.Type(), string(msg.Payload()))

				// Echo the message back to the client.
				_ = client.Send(protocol.NewMessage("echo_response", msg.Payload()))
			}
		}()
	})

	engine.OnDisconnect(func(client protocol.Client, reason error) {
		log.Printf("Client disconnected: %s, Reason: %v", client.ID(), reason)
	})

	// 7. Periodically broadcast a message to the world-events group.
	go func() {
		for {
			time.Sleep(5 * time.Second)
			msg := protocol.NewMessage("world_update", []byte("The world is changing!"))
			log.Printf("Broadcasting to group 'world-events'...")
			engine.Group("world-events").Broadcast(msg)
		}
	}()

	// 8. Start the engine with a QUIC transport.
	transport := protocol.NewQuicTransport()
	addr := "localhost:8080"
	if err := transport.Listen(context.Background(), addr); err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	log.Printf("Server starting on %s using QUIC", addr)
	if err := engine.Start(context.Background(), transport); err != nil {
		log.Fatalf("Engine failed to start: %v", err)
	}

	// Keep the server running.
	select {}
}
