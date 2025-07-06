package server

import (
	"context"
	"fmt"
	"log"

	"zeusync/internal/core/protocol"
)

// Run starts the server.
func Run() {
	// Create a new QUIC transport.
	transport := protocol.NewQuicTransport()

	// Create a new engine.
	engine := &protocol.Engine{
		OnConnect: func(peer protocol.Peer) {
			fmt.Printf("Peer connected: %s\n", peer.ID())
			defer engine.OnDisconnect(peer)

			for {
				msg, err := peer.Receive(context.Background())
				if err != nil {
					fmt.Printf("Error receiving message from peer %s: %v\n", peer.ID(), err)
					return
				}

				fmt.Printf("Received message from peer %s: %s\n", peer.ID(), msg.Payload())

				// Echo the message back to the peer.
				if err := peer.Send(context.Background(), msg); err != nil {
					fmt.Printf("Error sending message to peer %s: %v\n", peer.ID(), err)
				}
			}
		},
		OnDisconnect: func(peer protocol.Peer) {
			fmt.Printf("Peer disconnected: %s\n", peer.ID())
		},
	}

	// Start listening for incoming connections.
	if err := transport.Listen(context.Background(), ":8080"); err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	fmt.Println("Server started on :8080")

	// Accept incoming connections.
	for {
		conn, err := transport.Accept(context.Background())
		if err != nil {
			log.Printf("Failed to accept connection: %v", err)
			continue
		}

		// Create a new peer for the connection.
		// In a real application, you would have a more robust way of generating peer IDs.
		peerID := fmt.Sprintf("peer-%s", conn.RemoteAddr().String())
		peer := protocol.NewDefaultPeer(peerID, conn, &protocol.JSONCodec{})

		// Handle the new peer in a separate goroutine to avoid blocking the accept loop.
		go engine.OnConnect(peer)
	}
}
