package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

type PositionLimit struct {
	query *donburi.Query
}

func NewPositionLimit() *PositionLimit {
	return &PositionLimit{
		query: donburi.NewQuery(filter.Contains(component.PositionLimit)),
	}
}

func (s *PositionLimit) Update(w donburi.World) {
	for entry := range s.query.Iter(w) {
		tr := transform.GetTransform(entry)
		// Copy the limit, so the adjustments below don't modify the component.
		limit := *component.PositionLimit.Get(entry)

		// Keep the whole bounds inside the limit, using the current (possibly changing) scale.
		if entry.HasComponent(component.Bounds) {
			rect := component.Bounds.Get(entry).Rect(entry)
			limit.Min.X += rect.Width / 2
			limit.Min.Y += rect.Height / 2
			limit.Max.X -= rect.Width / 2
			limit.Max.Y -= rect.Height / 2
		}

		tr.LocalPosition.X = engine.Clamp(tr.LocalPosition.X, limit.Min.X, limit.Max.X)
		tr.LocalPosition.Y = engine.Clamp(tr.LocalPosition.Y, limit.Min.Y, limit.Max.Y)
	}
}
