package component

import "github.com/yohamta/donburi"

var Destroyed = donburi.NewTag()

func Destroy(e *donburi.Entry) {
	if !e.Valid() {
		return
	}
	if !e.HasComponent(Destroyed) {
		e.AddComponent(Destroyed)
	}
}
