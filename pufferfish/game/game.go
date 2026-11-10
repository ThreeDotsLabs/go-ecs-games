package game

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/archetype"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/system"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/events"
	"github.com/yohamta/donburi/features/math"
)

const (
	ScreenWidth  = 1024
	ScreenHeight = 900
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

type Subscriber interface {
	Subscribe(w donburi.World)
}

func NewGame() *Game {
	g := &Game{}

	g.systems = []System{
		system.NewDebug(),
		system.NewAI(),
		system.NewControls(),
		system.NewCameraFollow(),
		system.NewVelocity(),
		system.NewDrag(),
		system.NewPositionLimit(),
		system.NewCollecting(),
		system.NewBubbleSpawn(),
		system.NewKill(),
		system.NewTimeToLive(),
		system.NewRemains(),
		system.NewDestroy(),
	}
	g.drawers = []Drawer{
		system.NewRendering(),
		system.NewTextRendering(),
	}

	g.world = createWorld()

	subscribers := []Subscriber{
		system.NewScoring(),
	}
	for _, s := range subscribers {
		s.Subscribe(g.world)
	}

	return g
}

func createWorld() donburi.World {
	w := donburi.NewWorld()

	settings := w.Create(component.Settings)
	component.Settings.SetValue(w.Entry(settings), component.SettingsData{
		ScreenWidth:  ScreenWidth,
		ScreenHeight: ScreenHeight,
	})

	player := archetype.NewPlayer(w, math.Vec2{X: domain.MapWidth / 2, Y: domain.MapHeight / 2})

	cam := archetype.New(w).
		With(component.Camera).
		With(component.PositionLimit).
		Entry()

	component.Camera.SetValue(cam, component.CameraData{
		Target: player,
	})

	component.PositionLimit.SetValue(cam, component.PositionLimitData{
		Min: math.Vec2{X: 0, Y: 0},
		Max: math.Vec2{X: domain.MapWidth - ScreenWidth, Y: domain.MapHeight - ScreenHeight},
	})

	for x := 0.0; x < domain.MapWidth; x += 300 {
		for y := 0.0; y < domain.MapHeight; y += 300 {
			for range engine.RandomIntRange(1, 3) {
				archetype.NewFish(w, randomPositionNear(x, y))
			}
		}
	}

	for x := 0.0; x < domain.MapWidth; x += 400 {
		for y := 0.0; y < domain.MapHeight; y += 400 {
			archetype.NewBubbleSpawner(w, randomPositionNear(x, y))
		}
	}

	return w
}

func randomPositionNear(x, y float64) math.Vec2 {
	return math.Vec2{
		X: x + engine.RandomFloatRange(-50, 100),
		Y: y + engine.RandomFloatRange(-30, 30),
	}
}

func (g *Game) Update() error {
	for _, s := range g.systems {
		s.Update(g.world)
	}

	events.ProcessAllEvents(g.world)

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
