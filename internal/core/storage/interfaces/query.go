package interfaces

import (
	"time"

	"github.com/zeusync/zeusync/internal/core/models"
)

// Query interface for flexible data querying
// Supports SQL-like operations and document-style queries
type Query interface {
	// Collection/Table selection

	Collection(name string) Query
	Table(name string) Query

	// Filtering (WHERE clauses)

	Where(field string, operator Operator, value any) Query
	WhereIn(field string, values []any) Query
	WhereNotIn(field string, values []any) Query
	WhereBetween(field string, min, max any) Query
	WhereNull(field string) Query
	WhereNotNull(field string) Query

	// Logical operators

	And(query Query) Query
	Or(query Query) Query
	Not(query Query) Query

	// Sorting

	OrderBy(field string, direction SortDirection) Query
	OrderByDesc(field string) Query
	OrderByAsc(field string) Query

	// Limiting and pagination

	Limit(count int) Query
	Offset(count int) Query
	Page(page, size int) Query

	// Field selection

	Select(fields ...string) Query
	Exclude(fields ...string) Query

	// Joins (for relational storage)

	Join(table string, condition JoinCondition) Query
	LeftJoin(table string, condition JoinCondition) Query
	RightJoin(table string, condition JoinCondition) Query

	// Aggregation

	GroupBy(fields ...string) Query
	Having(condition string) Query
	Aggregate(function AggregateFunction, field string) Query

	// Execution

	Execute() (QueryResult, error)
	Stream() (QueryStream, error)
	Count() (int64, error)
	First() (any, error)
	Exists() (bool, error)

	// Query building

	Build() (string, []any, error)
	Clone() Query
}

type JoinCondition struct {
	Table     string
	Condition string
}

type AggregateFunction func(any, any) any

// QueryResult contains query execution results
type QueryResult interface {
	// Data access

	All() ([]any, error)
	First() (any, error)
	Last() (any, error)
	Get(index int) (any, error)

	// Metadata

	Count() int
	HasMore() bool
	TotalCount() int64
	ExecutionTime() time.Duration

	// Pagination

	NextPage() (QueryResult, error)
	PreviousPage() (QueryResult, error)
	Page(page int) (QueryResult, error)

	// Iteration

	Iterator() models.Iterator[any]

	ForEach(func(any) error) error
	Map(func(any) any) ([]any, error)
	Filter(func(any) bool) ([]any, error)

	// Serialization

	ToJSON() ([]byte, error)
	ToMap() ([]map[string]interface{}, error)

	// Cleanup

	Close() error
}

// TypedQueryResult is a generic type for typed query results
type TypedQueryResult[T any] interface {
	// Data access

	All() ([]T, error)
	First() (T, error)
	Last() (T, error)
	Get(index int) (T, error)

	// Metadata

	Count() int
	HasMore() bool
	TotalCount() int64
	ExecutionTime() time.Duration

	// Pagination

	NextPage() (TypedQueryResult[T], error)
	PreviousPage() (TypedQueryResult[T], error)
	Page(page int) (TypedQueryResult[T], error)

	// Iteration

	Iterator() models.Iterator[T]

	ForEach(func(T) error) error
	Map(func(T) T) ([]T, error)
	Filter(func(T) bool) ([]T, error)

	// Serialization

	ToJSON() ([]byte, error)
	ToMap() ([]map[string]T, error)

	// Cleanup

	Close() error
}

// QueryStream provides streaming access to large result sets
type QueryStream interface {
	// Streaming

	Next() bool
	Value() any
	Error() error
	Close() error

	// Buffering

	SetBufferSize(size int)
	GetBufferSize() int

	// Progress tracking

	Progress() StreamProgress
}

// StreamProgress tracks streaming progress
type StreamProgress struct {
	ProcessedRecords   uint32
	TotalRecords       uint32
	StartTime          time.Time
	EstimatedRemaining time.Duration
}
