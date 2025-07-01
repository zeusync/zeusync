package vars

import "unsafe"

// FastString represents a string that is optimized for fast access.
type FastString struct {
	data   unsafe.Pointer
	length int
}

// NewFastString creates new FastString from provided string
func NewFastString(s string) *FastString {
	if len(s) == 0 {
		return &FastString{
			data:   nil,
			length: 0,
		}
	}

	data := make([]byte, len(s))
	copy(data, s)

	return &FastString{
		data:   unsafe.Pointer(&data[0]),
		length: len(s),
	}
}

// String converts FastString back to a regular string
func (fs *FastString) String() string {
	if fs.length == 0 || fs.data == nil {
		return ""
	}

	// Create a slice from unsafe.Pointer
	bytes := (*[1 << 30]byte)(fs.data)[:fs.length:fs.length]
	return string(bytes)
}
