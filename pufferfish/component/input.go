package component

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
)

type InputData struct {
	KeyUp    ebiten.Key
	KeyDown  ebiten.Key
	KeyLeft  ebiten.Key
	KeyRight ebiten.Key
}

var Input = donburi.NewComponentType[InputData]()
