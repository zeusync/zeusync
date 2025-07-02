package fields

// Type defines different implementations for convertible types
type Type uint8

const (
	Int Type = iota
	Int64
	Int32
	Uint
	Uint64
	Uint32
	Float64
	Float32
	String
	Bool
	Bytes
	Time
	Map
	Array
	Set
	Enum
	Any
	Duration
	JSON
	XML
)
