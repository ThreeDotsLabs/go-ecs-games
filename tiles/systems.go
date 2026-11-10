package main

import (
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/filter"
)

type VelocitySystem struct {
	query *donburi.Query
}

func NewVelocitySystem() *VelocitySystem {
	return &VelocitySystem{
		query: donburi.NewQuery(
			filter.Contains(Velocity),
		),
	}
}

func (v *VelocitySystem) Update(w donburi.World) {
	for entry := range v.query.Iter(w) {
		pos := Position.Get(entry)
		rot := Rotation.Get(entry)
		velocity := Velocity.Get(entry)

		pos.X += velocity.X
		pos.Y += velocity.Y
		rot.Angle += velocity.Rotation
	}
}

type RenderingSystem struct {
	query     *donburi.Query
	tileImage *ebiten.Image
}

func NewRenderingSystem() *RenderingSystem {
	// A white tile that gets tinted with each tile's color when drawn.
	tileImage := ebiten.NewImage(TileSize, TileSize)
	tileImage.Fill(color.White)

	return &RenderingSystem{
		query: donburi.NewQuery(
			filter.Contains(Position, Color),
		),
		tileImage: tileImage,
	}
}

func (r *RenderingSystem) Draw(w donburi.World, screen *ebiten.Image) {
	for entry := range r.query.Iter(w) {
		pos := Position.Get(entry)
		c := Color.Get(entry)
		rot := Rotation.Get(entry)

		op := &ebiten.DrawImageOptions{}

		halfSize := float64(TileSize) / 2
		op.GeoM.Translate(-halfSize, -halfSize)
		op.GeoM.Rotate(rot.Angle)
		op.GeoM.Translate(halfSize, halfSize)

		op.GeoM.Translate(pos.X, pos.Y)

		op.ColorScale.ScaleWithColor(c.Color)

		screen.DrawImage(r.tileImage, op)
	}
}

type ClickSystem struct {
	query *donburi.Query
}

func NewClickSystem() *ClickSystem {
	return &ClickSystem{
		query: donburi.NewQuery(
			filter.Contains(Clickable, Velocity),
		),
	}
}

func (c *ClickSystem) Update(w donburi.World) {
	p, ok := pressedPoint()
	if !ok {
		return
	}

	for entry := range c.query.Iter(w) {
		pos := Position.Get(entry)

		if p.X >= pos.X && p.X <= pos.X+TileSize &&
			p.Y >= pos.Y && p.Y <= pos.Y+TileSize {

			velocity := Velocity.Get(entry)
			velocity.X = randomRange(-1, 1)
			velocity.Y = randomRange(1.5, 2.5)
			velocity.Rotation = randomRange(-0.1, 0.1)

			entry.RemoveComponent(Clickable)
		}
	}
}

// pressedPoint returns the position of the pressed left mouse button or the first active touch.
func pressedPoint() (math.Vec2, bool) {
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		return math.NewVec2(float64(x), float64(y)), true
	}

	if ids := ebiten.AppendTouchIDs(nil); len(ids) > 0 {
		x, y := ebiten.TouchPosition(ids[0])
		return math.NewVec2(float64(x), float64(y)), true
	}

	return math.Vec2{}, false
}

func randomRange(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}
