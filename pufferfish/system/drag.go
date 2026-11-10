package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/filter"
)

var drag = math.Vec2{X: domain.WaterDragX, Y: domain.WaterDragY}

type Drag struct {
	query *donburi.Query
}

func NewDrag() *Drag {
	return &Drag{
		query: donburi.NewQuery(filter.Contains(component.Velocity, component.Drag)),
	}
}

func (d *Drag) Update(w donburi.World) {
	for entry := range d.query.Iter(w) {
		velocity := component.Velocity.Get(entry)

		velocity.Velocity.X = slowDown(velocity.Velocity.X, drag.X)
		velocity.Velocity.Y = slowDown(velocity.Velocity.Y, drag.Y)
	}
}

func slowDown(v float64, amount float64) float64 {
	if v > 0 {
		return max(0, v-amount)
	}

	return min(0, v+amount)
}
