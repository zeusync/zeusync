package interfaces

import (
	"context"
	"time"

	"github.com/zeusync/zeusync/internal/core/models/interfaces"
)

// Storage provides abstract data storage interface
// Supports various backends: memory, SQL, NoSQL, hybrid
type Storage interface {
	// Basic CRUD operations

	Create(ctx context.Context, key string, value any) error
	Read(ctx context.Context, key string) (any, error)
	Update(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)

	// Query operations
	Query(ctx context.Context, query Query) (QueryResult, error)
	Stream(ctx context.Context, query Query) (QueryStream, error)
	Count(ctx context.Context, query Query) (int64, error)

	// Schema management

	// Monitoring and maintenance

	GetStatistics(ctx context.Context) (StorageStatistics, error)
	Compact(ctx context.Context) error
	Backup(ctx context.Context, path string) error
	Restore(ctx context.Context, path string) error

	// Connection management

	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Ping(ctx context.Context) error
	IsConnected() bool

	// Configuration

	GetConfig() StorageConfig
	SetConfig(StorageConfig) error
}

// BatchedStorage provides batch operations for performance for a storage.
type BatchedStorage interface {
	Storage

	BatchCreate(ctx context.Context, items map[string]any) error
	BatchRead(ctx context.Context, keys []string) (map[string]any, error)
	BatchUpdate(ctx context.Context, items map[string]any) error
	BatchDelete(ctx context.Context, keys []string) error
}

// TransactionalStorage provides transactional operations for a storage.
type TransactionalStorage interface {
	Storage

	// Transaction support

	BeginTransaction(ctx context.Context) (Transaction, error)
	WithTransaction(ctx context.Context, fn func(Transaction) error) error
	Rollback(ctx context.Context) error
	Snapshot(ctx context.Context) (Snapshot, error)
}

// CollectionalStorage provides operations collection with for a storage that supports it.
type CollectionalStorage interface {
	Storage

	CreateCollection(ctx context.Context, name string, schema CollectionSchema) error
	DropCollection(ctx context.Context, name string) error
	GetSchema(ctx context.Context, collection string) (CollectionSchema, error)
	UpdateSchema(ctx context.Context, collection string, schema CollectionSchema) error
}

// IndexicalStorage provides operations for indexical storage.
type IndexicalStorage interface {
	CreateIndex(ctx context.Context, collection string, index IndexDefinition) error
	DropIndex(ctx context.Context, collection string, indexName string) error
	ListIndexes(ctx context.Context, collection string) ([]IndexDefinition, error)
}

// Transaction provides ACID transaction support
type Transaction interface {
	// CRUD operations within transaction

	Create(key string, value any) error
	Read(key string) (any, error)
	Update(key string, value any) error
	Delete(key string) error

	// Batch operations

	BatchCreate(items map[string]any) error
	BatchUpdate(items map[string]any) error
	BatchDelete(keys []string) error

	// Query within transaction

	Query(query Query) (QueryResult, error)

	// Transaction control

	Commit() error
	Rollback() error
	Savepoint(name string) error
	RollbackToSavepoint(name string) error

	// State

	IsActive() bool
	IsolationLevel() IsolationLevel
}

// Operator defines query comparison operators
type Operator int

const (
	OpEqual Operator = iota
	OpNotEqual
	OpGreaterThan
	OpGreaterThanEqual
	OpLessThan
	OpLessThanEqual
	OpLike
	OpNotLike
	OpRegex
	OpIn
	OpNotIn
	OpBetween
	OpNotBetween
	OpIsNull
	OpIsNotNull
	OpExists
	OpNotExists
)

// SortDirection defines sort order
type SortDirection int

const (
	Ascending SortDirection = iota
	Descending
)

// IsolationLevel defines transaction isolation levels
type IsolationLevel int

const (
	ReadUncommitted IsolationLevel = iota
	ReadCommitted
	RepeatableRead
	Serializable
)

// CollectionSchema defines structure of a data collection
type CollectionSchema interface {
	Name() string
	Fields() []FieldSchema
	Indexes() []IndexDefinition
	Constraints() []ConstraintDefinition
	Version() string
	Metadata() map[string]any

	// Validation

	Validate(data any) error
	ValidateField(fieldName string, value any) error

	// Evolution

	CanMigrateTo(newSchema CollectionSchema) bool
	GenerateMigration(newSchema CollectionSchema) (Migration, error)
}

// FieldSchema defines a field in a collection
type FieldSchema interface {
	Name() string
	Type() interfaces.FieldType
	Required() bool
	Default() any
	Constraints() []interfaces.FieldConstraint
	Indexed() bool
	Unique() bool
	Metadata() map[string]any
}

// IndexDefinition defines database indexes
type IndexDefinition struct {
	Name    string
	Fields  []IndexField
	Type    IndexType
	Unique  bool
	Sparse  bool
	Options map[string]any
}

// IndexField defines a field in an index
type IndexField struct {
	Name      string
	Direction SortDirection
	Weight    float64 // For text indexes
}

// IndexType defines different index types
type IndexType uint8

const (
	IndexBTree IndexType = iota
	IndexHash
	IndexText
	IndexGeospatial
	IndexCompound
	IndexPartial
)

type StorageSchema interface{}

// StorageStatistics provides runtime statistics
type StorageStatistics struct {
	// Connection stats
	ConnectionsActive uint32
	ConnectionsTotal  uint32
	ConnectionErrors  uint32

	// Operation stats
	ReadsPerSecond   float64
	WritesPerSecond  float64
	QueriesPerSecond float64
	AverageLatency   time.Duration

	// Storage stats
	TotalSize       int64
	UsedSize        int64
	FreeSize        int64
	IndexSize       int64
	CollectionCount int
	DocumentCount   int64

	// Performance metrics
	CacheHitRatio     float64
	IndexUsageRatio   float64
	QueryOptimization float64

	// System health
	LastBackup     time.Time
	LastCompaction time.Time
	ErrorRate      float64
	UptimeSeconds  uint64

	// Storage Metadata
	Metadata map[string]any
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	// Connection settings
	Host             string
	Port             int
	Database         string
	Username         string
	Password         string
	ConnectionString string

	// Performance tuning
	MaxConnections    int
	ConnectionTimeout time.Duration
	QueryTimeout      time.Duration
	IdleTimeout       time.Duration
	RetryAttempts     int
	RetryDelay        time.Duration

	// Caching
	CacheEnabled        bool
	CacheSize           uint64
	CacheTTL            time.Duration
	CachePolicy         string // ENUM
	CacheEvictionPolicy string // ENUM
	CacheList           []string

	// Backup and maintenance
	AutoBackup         bool
	BackupInterval     time.Duration
	BackupRetention    time.Duration
	AutoCompaction     bool
	CompactionInterval time.Duration

	// Custom settings
	Options map[string]any
}

type Migration struct{}

type ConstraintDefinition struct{}

type Snapshot struct{}
