package intrefaces

import "time"

// ClientInfo contains client connection information
type ClientInfo struct {
	ID              string
	RemoteAddress   string
	ConnectedAt     time.Time
	LastActivity    time.Time
	UserAgent       string
	Version         string
	Groups          []string
	Metadata        map[string]any
	IsAuthenticated bool
	UserID          string
	Permissions     []string
}

type ClientMetrics struct {
	ActiveConnections    uint64
	TotalConnections     uint64
	FailedConnections    uint64
	ConnectionsPerSecond float64
	MessagesSent         uint64
	MessagesReceived     uint64
	MessagesPerSecond    float64
	AverageMessageSize   float64
}
