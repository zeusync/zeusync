package encoding

// Serializable provides a clean, simple interface for serializing and deserializing values.
type Serializable[T any] interface {
	Serialize() ([]byte, error)
	Deserialize([]byte) error
}
