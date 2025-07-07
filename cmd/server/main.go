package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zeusync/zeusync/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := server.NewServer(server.DefaultServerConfig())

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	// Start the server
	if err := srv.Start(ctx); err != nil {
		fmt.Println("Error starting server:", err)
	}

	<-stopCh
	cancel()
	stopCtx, stopCtxCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer stopCtxCancel()

	if err := srv.Stop(stopCtx); err != nil {
		fmt.Println("Error stopping server:", err)
	}
}
