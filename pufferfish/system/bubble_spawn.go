package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/archetype"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

type BubbleSpawn struct {
	query *donburi.Query
}

func NewBubbleSpawn() *BubbleSpawn {
	return &BubbleSpawn{
		query: donburi.NewQuery(
			filter.Contains(component.BubbleSpawner),
		),
	}
}

func (b *BubbleSpawn) Update(w donburi.World) {
	for entry := range b.query.Iter(w) {
		spawner := component.BubbleSpawner.Get(entry)

		spawner.SpawnTimer.Update()
		if spawner.SpawnTimer.IsReady() {
			archetype.NewBubbleCluster(w, entry)
			spawner.SpawnTimer.Reset()
		}
	}
}
