package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

type TimeToLive struct {
	query *donburi.Query
}

func NewTimeToLive() *TimeToLive {
	return &TimeToLive{
		query: donburi.NewQuery(filter.Contains(component.TimeToLive)),
	}
}

func (t *TimeToLive) Update(w donburi.World) {
	for entry := range t.query.Iter(w) {
		ttl := component.TimeToLive.Get(entry)

		ttl.Timer.Update()
		if ttl.Timer.IsReady() {
			component.Destroy(entry)
		}
	}
}
