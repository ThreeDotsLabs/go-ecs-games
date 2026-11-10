package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

type Destroy struct {
	query *donburi.Query
}

func NewDestroy() *Destroy {
	return &Destroy{
		query: donburi.NewQuery(
			filter.Contains(component.Destroyed),
		),
	}
}

func (d *Destroy) Update(w donburi.World) {
	for entry := range d.query.Iter(w) {
		transform.RemoveRecursive(entry)
	}
}
