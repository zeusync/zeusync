# ZeuSync Protocol Examples Makefile

.PHONY: help ws-server ws-client quic-server quic-client clean deps test benchmark

# Default target
help:
	@echo "ZeuSync Protocol Examples"
	@echo ""
	@echo "Available targets:"
	@echo "  deps        - Install dependencies"
	@echo "  ws-server   - Run WebSocket server"
	@echo "  ws-client   - Run WebSocket client"
	@echo "  quic-server - Run QUIC server"
	@echo "  quic-client - Run QUIC client"
	@echo "  benchmark   - Run performance comparison"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo ""
	@echo "Usage examples:"
	@echo "  make ws-server    # Start WebSocket server on :8080"
	@echo "  make ws-client    # Connect WebSocket client to server"
	@echo "  make quic-server  # Start QUIC server on :9090"
	@echo "  make quic-client  # Connect QUIC client to server"
	@echo "  make benchmark    # Compare WebSocket vs QUIC performance"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	cd .. && go mod tidy
	cd .. && go mod download

# WebSocket server
ws-server:
	@echo "Starting WebSocket server on localhost:8080..."
	@echo "Endpoints:"
	@echo "  - ws://localhost:8080/ws (WebSocket)"
	@echo "  - http://localhost:8080/health (Health check)"
	@echo "  - http://localhost:8080/metrics (Metrics)"
	@echo ""
	cd .. && go run examples/websocket_server.go

# WebSocket client
ws-client:
	@echo "Starting WebSocket client..."
	@echo "Connecting to ws://localhost:8080/ws"
	@echo ""
	cd .. && go run examples/websocket_client.go

# QUIC server
quic-server:
	@echo "Starting QUIC server on localhost:9090..."
	@echo "Features:"
	@echo "  - Multiplexed streams"
	@echo "  - Built-in TLS 1.3 encryption"
	@echo "  - 0-RTT handshake"
	@echo "  - Connection migration"
	@echo ""
	cd .. && go run examples/quic_server.go

# QUIC client
quic-client:
	@echo "Starting QUIC client..."
	@echo "Connecting to localhost:9090"
	@echo ""
	cd .. && go run examples/quic_client.go

# Performance benchmark
benchmark:
	@echo "Running WebSocket vs QUIC performance comparison..."
	@echo "This will test both protocols with multiple clients and messages"
	@echo ""
	cd .. && go run examples/benchmark_comparison.go

# Run tests
test:
	@echo "Running protocol tests..."
	cd .. && go test ./internal/core/protocol/... -v

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd .. && go clean -cache
	cd .. && go clean -modcache

# Development helpers
.PHONY: dev-ws dev-quic

# Start WebSocket server and client in parallel (for development)
dev-ws:
	@echo "Starting WebSocket development environment..."
	@echo "Server will start on :8080, client will connect automatically"
	@echo "Press Ctrl+C to stop both"
	@(make ws-server &) && sleep 2 && make ws-client

# Start QUIC server and client in parallel (for development)
dev-quic:
	@echo "Starting QUIC development environment..."
	@echo "Server will start on :9090, client will connect automatically"
	@echo "Press Ctrl+C to stop both"
	@(make quic-server &) && sleep 2 && make quic-client