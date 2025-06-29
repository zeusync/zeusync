package interfaces

import (
	"context"
)

type Storage interface {
	Create(ctx context.Context, key string, value any) error
	Read(ctx context.Context, key string) (any, error)
	Update(ctx context.Context, key string, value any) error
	Delete(ctx context.Context, key string) error

	Schema() StorageSchema
	Statistics() StorageStatistics
}

type BatchedStorage interface {
	Storage

	BatchCreate(ctx context.Context, values map[string]any) error
	BatchRead(ctx context.Context, keys []string) (map[string]any, error)
	BatchUpdate(ctx context.Context, values map[string]any) error
	BatchDelete(ctx context.Context, keys []string) error
}

type QueryableStorage interface {
	Storage

	Query(ctx context.Context, query Query) (QueryResult, error)
	Stream(ctx context.Context, query Query) (QueryResultStream, error)
}

type TransactionalStorage interface {
	Storage

	Begin(ctx context.Context) (Transaction, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Snapshot(ctx context.Context) (Snapshot, error)
}

type Query interface {
	Select(field string, value any) Query
	Insert(field string, value any) Query
	Update(field string, value any) Query
	Delete(field string, value any) Query
	Where(field string, operator string, value any) Query
	OrderBy(field string, direction string) Query
	Limit(limit uint32) Query
	Offset(offset uint32) Query
	Count() (uint32, error)
	Join(table string, joinType string, condition string) Query
	GroupBy(field string) Query
	Order(field string, direction string) Query
}

type QueryResult interface{}

type QueryResultStream interface{}

type Transaction interface{}

type StorageSchema interface{}

type StorageStatistics interface{}

type Snapshot interface{}
