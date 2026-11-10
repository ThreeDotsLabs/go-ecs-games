package component

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
)

type BoundsData struct {
	Width  float64
	Height float64
}

// Rect returns the world-space bounds, with the entity's current scale applied.
func (d BoundsData) Rect(e *donburi.Entry) engine.Rect {
	scale := transform.WorldScale(e)
	pos := transform.WorldPosition(e)
	w := d.Width * scale.X
	h := d.Height * scale.Y
	return engine.NewRect(pos.X-w/2, pos.Y-h/2, w, h)
}

var Bounds = donburi.NewComponentType[BoundsData]()
