package component

import "github.com/yohamta/donburi"

var Killer = donburi.NewTag()

type KillableData struct {
	Points int
}

var Killable = donburi.NewComponentType[KillableData]()
