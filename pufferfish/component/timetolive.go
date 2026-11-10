package component

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/yohamta/donburi"
)

// TimeToLiveData destroys the entity once the timer is done.
type TimeToLiveData struct {
	Timer *engine.Timer
}

var TimeToLive = donburi.NewComponentType[TimeToLiveData]()
