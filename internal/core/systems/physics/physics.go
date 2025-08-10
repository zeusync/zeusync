package physics

import "math"

// Simple concrete implementations for convenience.

type Vec2 struct{ Xv, Yv float64 }

func (v Vec2) X() float64 { return v.Xv }
func (v Vec2) Y() float64 { return v.Yv }

type Vec3 struct{ Xv, Yv, Zv float64 }

func (v Vec3) X() float64 { return v.Xv }
func (v Vec3) Y() float64 { return v.Yv }
func (v Vec3) Z() float64 { return v.Zv }

type Transform2D struct{ Pos Vec2 }

func (t Transform2D) Position2() (x, y float64)    { return t.Pos.Xv, t.Pos.Yv }
func (t Transform2D) Position3() (x, y, z float64) { return t.Pos.Xv, t.Pos.Yv, 0 }

type Transform3D struct{ Pos Vec3 }

func (t Transform3D) Position2() (x, y float64)    { return t.Pos.Xv, t.Pos.Yv }
func (t Transform3D) Position3() (x, y, z float64) { return t.Pos.Xv, t.Pos.Yv, t.Pos.Zv }

// Distance2 computes Euclidean distance between two 2D points.
func Distance2(x1, y1, x2, y2 float64) float64 { return math.Hypot(x2-x1, y2-y1) }

// Distance2V computes distance from two Vector2.
func Distance2V(a, b Vector2) float64 { return math.Hypot(b.X()-a.X(), b.Y()-a.Y()) }

// DistanceT computes distance between two transforms using 2D positions.
func DistanceT(a, b Transform) float64 {
	x1, y1 := a.Position2()
	x2, y2 := b.Position2()
	return Distance2(x1, y1, x2, y2)
}
