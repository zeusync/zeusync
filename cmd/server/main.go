package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/zeusync/zeusync/internal/server"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := server.NewServer(ctx)

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGINT)

	// Start the server
	if err := srv.Start(ctx); err != nil {
		fmt.Println("Error starting server:", err)
	}

	<-stopCh
	cancel()
	if err := srv.Stop(); err != nil {
		fmt.Println("Error stopping server:", err)
	}
}
