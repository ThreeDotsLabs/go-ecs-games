package component

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/yohamta/donburi"
)

type BubbleSpawnerData struct {
	SpawnTimer *engine.Timer
}

var BubbleSpawner = donburi.NewComponentType[BubbleSpawnerData]()
