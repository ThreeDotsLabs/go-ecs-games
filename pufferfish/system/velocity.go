package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

type Velocity struct {
	query *donburi.Query
}

func NewVelocity() *Velocity {
	return &Velocity{
		query: donburi.NewQuery(
			filter.Contains(component.Velocity),
		),
	}
}

func (v *Velocity) Update(w donburi.World) {
	for entry := range v.query.Iter(w) {
		t := transform.Transform.Get(entry)
		velocity := component.Velocity.Get(entry)

		t.LocalPosition = t.LocalPosition.Add(velocity.Velocity)
		t.LocalRotation += velocity.RotationVelocity
	}
}
