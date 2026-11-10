package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/yohamta/donburi"
)

type Debug struct{}

func NewDebug() *Debug {
	return &Debug{}
}

func (d *Debug) Update(w donburi.World) {
	if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
		settings := component.MustFindSettings(w)
		settings.Debug = !settings.Debug
	}
}
