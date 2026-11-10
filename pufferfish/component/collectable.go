package component

import "github.com/yohamta/donburi"

type CollectableData struct {
	Points int
}

var Collectable = donburi.NewComponentType[CollectableData]()
