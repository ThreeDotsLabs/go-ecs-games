package component

import "github.com/yohamta/donburi"

type CameraData struct {
	Target *donburi.Entry
}

var Camera = donburi.NewComponentType[CameraData]()
