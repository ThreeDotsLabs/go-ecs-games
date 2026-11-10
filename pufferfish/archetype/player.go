package archetype

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/assets"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/features/transform"
)

func NewPlayer(w donburi.World, pos math.Vec2) *donburi.Entry {
	player := New(w).
		WithPosition(pos).
		WithSprite(component.SpriteData{
			Image: assets.BrownFish,
			Layer: component.LayerPlayer,
		}).
		With(component.Input).
		With(component.Velocity).
		With(component.PositionLimit).
		With(component.Drag).
		With(component.Player).
		With(component.Killer).
		WithSpriteBounds().
		Entry()

	transform.GetTransform(player).LocalScale = math.Vec2{X: domain.PlayerStartScale, Y: domain.PlayerStartScale}

	// Slightly smaller bounds so that the fish can overlap a bit before colliding
	bounds := component.Bounds.Get(player)
	bounds.Width -= 20
	bounds.Height -= 20

	component.PositionLimit.SetValue(player, component.PositionLimitData{
		Min: math.Vec2{},
		Max: math.Vec2{X: domain.MapWidth, Y: domain.MapHeight},
	})

	component.Input.SetValue(player, component.InputData{
		KeyUp:    ebiten.KeyW,
		KeyDown:  ebiten.KeyS,
		KeyLeft:  ebiten.KeyA,
		KeyRight: ebiten.KeyD,
	})

	spriteHeight := float64(component.Sprite.Get(player).Image.Bounds().Dy())
	scale := transform.GetTransform(player).LocalScale

	New(w).
		WithParent(player).
		WithPosition(math.Vec2{X: 0, Y: spriteHeight * scale.Y / 2}).
		With(component.Text).
		With(component.Score).
		Entry()

	return player
}
