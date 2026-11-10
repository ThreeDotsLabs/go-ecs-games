package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/event"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

type Kill struct {
	killerQuery   *donburi.Query
	killableQuery *donburi.Query
}

func NewKill() *Kill {
	return &Kill{
		killerQuery: donburi.NewQuery(filter.Contains(component.Killer)),
		killableQuery: donburi.NewQuery(
			filter.And(
				filter.Contains(component.Killable),
				filter.Not(
					filter.Contains(component.Destroyed),
				),
			),
		),
	}
}

func (k *Kill) Update(w donburi.World) {
	for killerEntry := range k.killerQuery.Iter(w) {
		for killableEntry := range k.killableQuery.Iter(w) {
			killerBounds := component.Bounds.Get(killerEntry)
			killableBounds := component.Bounds.Get(killableEntry)

			if killerBounds.Rect(killerEntry).Intersects(killableBounds.Rect(killableEntry)) {
				component.Destroy(killableEntry)

				event.Killed.Publish(w, event.KilledData{
					Points: component.Killable.Get(killableEntry).Points,
				})
			}
		}
	}
}
