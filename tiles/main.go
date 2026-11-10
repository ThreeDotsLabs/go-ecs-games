package main

import (
	"image/color"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
)

const (
	ScreenWidth  = 1024
	ScreenHeight = 960

	TileSize = 64
)

type Game struct {
	world   donburi.World
	systems []System
	drawers []Drawer
}

type System interface {
	Update(w donburi.World)
}

type Drawer interface {
	Draw(w donburi.World, screen *ebiten.Image)
}

func NewGame() *Game {
	g := &Game{}

	g.systems = []System{
		NewClickSystem(),
		NewVelocitySystem(),
	}
	g.drawers = []Drawer{
		NewRenderingSystem(),
	}

	g.world = createWorld()

	return g
}

func createWorld() donburi.World {
	w := donburi.NewWorld()

	for x := range ScreenWidth / TileSize {
		for y := range ScreenHeight / TileSize {
			newTile(w, float64(x*TileSize), float64(y*TileSize))
		}
	}

	return w
}

var tileColors = []color.Color{
	color.RGBA{R: 0x00, G: 0x81, B: 0xa7, A: 0xff},
	color.RGBA{R: 0x00, G: 0xaf, B: 0xb9, A: 0xff},
	color.RGBA{R: 0xfd, G: 0xfc, B: 0xdc, A: 0xff},
	color.RGBA{R: 0xfe, G: 0xd9, B: 0xb7, A: 0xff},
	color.RGBA{R: 0xf0, G: 0x71, B: 0x67, A: 0xff},
}

func newTile(w donburi.World, x, y float64) {
	tile := w.Create(Position, Rotation, Color, Velocity, Clickable)
	entry := w.Entry(tile)

	Position.SetValue(entry, PositionData{
		X: x,
		Y: y,
	})

	Color.SetValue(entry, ColorData{
		Color: tileColors[rand.Intn(len(tileColors))],
	})
}

func (g *Game) Update() error {
	for _, s := range g.systems {
		s.Update(g.world)
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	for _, d := range g.drawers {
		d.Draw(g.world, screen)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return ScreenWidth, ScreenHeight
}

func main() {
	g := NewGame()

	ebiten.SetWindowSize(ScreenWidth, ScreenHeight)

	err := ebiten.RunGame(g)
	if err != nil {
		panic(err)
	}
}
