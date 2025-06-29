package main

import (
	"context"
	"github.com/zeusync/zeusync/internal/server"
)

func main() {
	ctx := context.Background()
	srv := server.NewServer(ctx)

	// Start the server
	_ = srv.Start(ctx)
	_ = srv.Stop()
}
