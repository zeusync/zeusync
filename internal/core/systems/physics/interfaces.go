package physics

// Lightweight physics abstractions for 2D/3D vectors and transforms.
// These are intentionally minimal to avoid introducing heavy dependencies
// while providing a common shape for sensors and AI logic.

// Vector2 represents a 2D vector.
type Vector2 interface {
	X() float64
	Y() float64
}

// Vector3 represents a 3D vector.
type Vector3 interface {
	X() float64
	Y() float64
	Z() float64
}

// Transform provides spatial information (position, rotation not used yet).
// Only Position is used by current sensors.
type Transform interface {
	Position2() (x, y float64)    // 2D shortcut
	Position3() (x, y, z float64) // 3D shortcut
}
