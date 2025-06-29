package interfaces

type Node interface {
	ID() string
	Address() string
	Status() NodeStatus
	Health() HealthStatus
	Metadata() map[string]any

	Start() error
	Stop() error
	Restart() error

	Send(message ClusterMessage) error
	Broadcast(message ClusterMessage) error

	GetLoad() LoadMetrics
	CanAccept() bool
}

type ClusterManager interface {
	RegisterNode(Node) error
	UnregisterNode(Node) error
	GetNode(string) (Node, error)
	ListNodes() []Node

	DiscoverServices() ([]ServiceInfo, error)
	RegisterService(ServiceInfo) error

	SelectNode(criteria SelectionCriteria) (Node, error)
	DistributeLoad(LoadContributionStrategy) error

	MonitorHealth() error
	GetClusterHealth() ClusterHealthStatus
}

type ClusterCoordinator interface {
	ElectLeader() error
	IsLeader() bool
	GetLeader() (Node, error)

	Propose(proposal Proposal) error
	Vote(proposalID string, vote Vote) error
	GetConsensus(proposalID string) (ConsensusResult, error)

	UpdateConfig(config ClusterConfig) error
	SyncConfig() error
}

type (
	ClusterMessage interface{}
	NodeStatus     struct{}
	HealthStatus   struct{}
	LoadMetrics    struct{}
)

type ServiceInfo struct{}

type SelectionCriteria interface{}

type (
	LoadContributionStrategy interface{}
	ClusterHealthStatus      struct{}
	ClusterConfig            struct{}
)

type (
	Proposal        interface{}
	Vote            interface{}
	ConsensusResult struct{}
)
