package main

import (
	"context"
	"fmt"
	"github.com/zeusync/zeusync/internal/core/observability/log"
	"github.com/zeusync/zeusync/internal/server"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Create server configuration
	config := server.DefaultServerConfig()
	config.ListenAddr = ":8080"
	config.MaxClients = 1000
	config.LogLevel = log.LevelInfo
	config.HealthCheckInterval = 30 * time.Second
	config.ClientTimeout = 2 * time.Minute

	// Create server
	srv := server.NewServer(config)

	// Start server
	ctx := context.Background()

	fmt.Println("🚀 Starting ZeuSync server...")
	err := srv.Start(ctx)
	if err != nil {
		log.Provide().Fatal("Failed to start server", log.Error(err))
	}

	fmt.Printf("✅ ZeuSync server started on %s\n", config.ListenAddr)
	fmt.Println("📊 Server is ready to accept connections")

	// Start statistics reporter
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stats := srv.GetStats()
				fmt.Printf("📈 Server Stats - Clients: %d, Groups: %d, Scopes: %d\n",
					stats.ClientCount, stats.GroupCount, stats.ScopeCount)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("🎮 Server is running. Press Ctrl+C to stop...")

	<-sigChan

	fmt.Println("\n🛑 Shutting down server...")

	// Stop server gracefully
	stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err = srv.Stop(stopCtx)
	if err != nil {
		fmt.Printf("Error stopping server: %v\n", err)
	}

	// Close server
	_ = srv.Close()

	fmt.Println("✅ Server shutdown complete")
}
