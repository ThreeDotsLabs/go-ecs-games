package component

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
)

// RemainsData is what an entity leaves behind once it's destroyed.
type RemainsData struct {
	Image *ebiten.Image
}

var Remains = donburi.NewComponentType[RemainsData]()
