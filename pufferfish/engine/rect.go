package engine

import (
	gomath "math"

	"github.com/yohamta/donburi/features/math"
)

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

func NewRect(x, y, width, height float64) Rect {
	return Rect{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
	}
}

func (r Rect) Position() math.Vec2 {
	return math.Vec2{
		X: r.X,
		Y: r.Y,
	}
}

func (r Rect) MaxX() float64 {
	return r.X + r.Width
}

func (r Rect) MaxY() float64 {
	return r.Y + r.Height
}

func (r Rect) Intersects(other Rect) bool {
	return r.X <= other.MaxX() &&
		other.X <= r.MaxX() &&
		r.Y <= other.MaxY() &&
		other.Y <= r.MaxY()
}

func (r Rect) Distance(other Rect) float64 {
	dx := gomath.Max(0, gomath.Max(other.X-r.MaxX(), r.X-other.MaxX()))
	dy := gomath.Max(0, gomath.Max(other.Y-r.MaxY(), r.Y-other.MaxY()))
	return gomath.Hypot(dx, dy)
}
