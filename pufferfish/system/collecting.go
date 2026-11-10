package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/event"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

type Collecting struct {
	playerQuery      *donburi.Query
	collectableQuery *donburi.Query
}

func NewCollecting() *Collecting {
	return &Collecting{
		playerQuery: donburi.NewQuery(filter.Contains(component.Player)),
		collectableQuery: donburi.NewQuery(filter.And(
			filter.Contains(component.Collectable),
			filter.Not(filter.Contains(component.Destroyed)),
		)),
	}
}

func (c *Collecting) Update(w donburi.World) {
	for playerEntry := range c.playerQuery.Iter(w) {
		for collectableEntry := range c.collectableQuery.Iter(w) {
			playerBounds := component.Bounds.Get(playerEntry)
			collectableBounds := component.Bounds.Get(collectableEntry)

			if playerBounds.Rect(playerEntry).Intersects(collectableBounds.Rect(collectableEntry)) {
				component.Destroy(collectableEntry)
				transform.GetTransform(playerEntry).LocalScale.X += domain.BubbleGrowth
				transform.GetTransform(playerEntry).LocalScale.Y += domain.BubbleGrowth

				event.Collected.Publish(w, event.CollectedData{
					Points: component.Collectable.Get(collectableEntry).Points,
				})
			}
		}
	}
}
