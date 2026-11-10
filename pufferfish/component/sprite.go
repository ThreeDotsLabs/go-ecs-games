package component

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
)

type Layer int

const (
	LayerBackground Layer = iota
	LayerPlayer
	LayerForeground
)

type SpriteData struct {
	Image *ebiten.Image
	Layer Layer
	FlipX bool
}

func (s SpriteData) Order() int {
	// Used by ordered query
	return int(s.Layer)
}

var Sprite = donburi.NewComponentType[SpriteData]()
