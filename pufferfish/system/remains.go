package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/archetype"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

type Remains struct {
	query *donburi.Query
}

func NewRemains() *Remains {
	return &Remains{
		query: donburi.NewQuery(filter.Contains(component.Destroyed, component.Remains)),
	}
}

func (r *Remains) Update(w donburi.World) {
	for entry := range r.query.Iter(w) {
		archetype.NewRemains(w, entry)
	}
}
