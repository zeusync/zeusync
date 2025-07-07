package main

import (
	"context"
	"fmt"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/core/protocol"
	"github.com/zeusync/zeusync/sdk/go/client"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Create client configuration
	config := client.DefaultClientConfig()
	config.ServerAddr = "localhost:8080"
	config.LogLevel = log.LevelInfo
	config.HeartbeatInterval = 10 * time.Second

	// Create client
	c := client.NewClient(config)

	// Set up event handlers
	c.OnEvent(client.EventTypeConnected, func(event client.Event) error {
		fmt.Printf("✅ Connected to server: %s\n", event.Data["server_addr"])
		return nil
	})

	c.OnEvent(client.EventTypeDisconnected, func(event client.Event) error {
		fmt.Println("❌ Disconnected from server")
		return nil
	})

	c.OnEvent(client.EventTypeReconnecting, func(event client.Event) error {
		fmt.Println("🔄 Reconnecting to server...")
		return nil
	})

	c.OnEvent(client.EventTypeJoinedGroup, func(event client.Event) error {
		fmt.Printf("👥 Joined group: %s\n", event.Data["group"])
		return nil
	})

	c.OnEvent(client.EventTypeJoinedScope, func(event client.Event) error {
		fmt.Printf("📡 Subscribed to scope: %s\n", event.Data["scope"])
		return nil
	})

	// Set up message handlers
	c.OnMessage(protocol.MessageTypeData, func(msg *protocol.Message) error {
		fmt.Printf("📨 Received data message: %s\n", msg.AsString())
		return nil
	})

	c.OnMessage(protocol.MessageTypeHeartbeat, func(msg *protocol.Message) error {
		fmt.Printf("💓 Received heartbeat: %s\n", msg.AsString())
		return nil
	})

	// Connect to server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("🔌 Connecting to ZeuSync server...")
	err := c.Connect(ctx)
	if err != nil {
		log.Provide().Fatal("Failed to connect", log.Error(err))
	}

	// Set client metadata
	c.SetMetadata("player_name", "TestPlayer")
	c.SetMetadata("game_version", "1.0.0")
	c.SetMetadata("platform", "desktop")

	// Demonstrate different message types
	go func() {
		time.Sleep(2 * time.Second)

		// Send a simple string message
		fmt.Println("📤 Sending string message...")
		err = c.SendString(protocol.MessageTypeData, "Hello from Go client!")
		if err != nil {
			fmt.Printf("Failed to send string message: %v\n", err)
		}

		time.Sleep(1 * time.Second)

		// Send a JSON message
		fmt.Println("📤 Sending JSON message...")
		gameData := map[string]interface{}{
			"player_id": c.ID(),
			"position":  map[string]float64{"x": 100.5, "y": 200.3, "z": 50.0},
			"health":    100,
			"timestamp": time.Now().Unix(),
		}
		err = c.SendJSON(protocol.MessageTypeData, gameData)
		if err != nil {
			fmt.Printf("Failed to send JSON message: %v\n", err)
		}

		time.Sleep(1 * time.Second)

		// Send binary data
		fmt.Println("📤 Sending binary message...")
		binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
		err = c.SendBytes(protocol.MessageTypeData, binaryData)
		if err != nil {
			fmt.Printf("Failed to send binary message: %v\n", err)
		}

		time.Sleep(1 * time.Second)

		// Join a group
		fmt.Println("👥 Joining game lobby...")
		err = c.JoinGroup("game_lobby")
		if err != nil {
			fmt.Printf("Failed to join group: %v\n", err)
		}

		time.Sleep(1 * time.Second)

		// Subscribe to a scope
		fmt.Println("📡 Subscribing to game state...")
		err = c.SubscribeToScope("game_state")
		if err != nil {
			fmt.Printf("Failed to subscribe to scope: %v\n", err)
		}

		// Send periodic messages
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		counter := 0
		for {
			select {
			case <-ticker.C:
				counter++

				// Send periodic update
				updateData := map[string]interface{}{
					"type":      "player_update",
					"counter":   counter,
					"client_id": string(c.ID()),
					"timestamp": time.Now().Unix(),
				}

				err = c.SendJSON(protocol.MessageTypeData, updateData)
				if err != nil {
					fmt.Printf("Failed to send periodic update: %v\n", err)
				} else {
					fmt.Printf("📤 Sent periodic update #%d\n", counter)
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	// Demonstrate ping functionality
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if c.IsConnected() {
					latency, err := c.Ping()
					if err != nil {
						fmt.Printf("❌ Ping failed: %v\n", err)
					} else {
						fmt.Printf("🏓 Ping: %v\n", latency)
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Show connection info
	go func() {
		time.Sleep(3 * time.Second)

		if c.IsConnected() {
			var info protocol.ConnectionInfo
			info, err = c.ConnectionInfo()
			if err != nil {
				fmt.Printf("Failed to get connection info: %v\n", err)
			} else {
				fmt.Printf("📊 Connection Info:\n")
				fmt.Printf("   ID: %s\n", info.ID)
				fmt.Printf("   Remote: %s\n", info.RemoteAddr)
				fmt.Printf("   Local: %s\n", info.LocalAddr)
				fmt.Printf("   Transport: %s\n", info.Transport)
				fmt.Printf("   Connected: %s\n", info.ConnectedAt.Format(time.RFC3339))
				fmt.Printf("   Bytes Sent: %d\n", info.BytesSent)
				fmt.Printf("   Bytes Received: %d\n", info.BytesReceived)
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("🚀 Client is running. Press Ctrl+C to exit...")
	fmt.Printf("📋 Client ID: %s\n", c.ID())

	<-sigChan

	fmt.Println("\n🛑 Shutting down client...")

	// Leave group and unsubscribe from scope
	if c.IsConnected() {
		fmt.Println("👋 Leaving group...")
		_ = c.LeaveGroup("game_lobby")

		fmt.Println("📡 Unsubscribing from scope...")
		_ = c.UnsubscribeFromScope("game_state")

		time.Sleep(1 * time.Second)
	}

	// Disconnect and close
	_ = c.Disconnect()
	_ = c.Close()

	fmt.Println("✅ Client shutdown complete")
}
