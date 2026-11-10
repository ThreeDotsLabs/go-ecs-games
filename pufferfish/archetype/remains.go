package archetype

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/features/transform"
)

func NewRemains(w donburi.World, from *donburi.Entry) *donburi.Entry {
	scale := transform.WorldScale(from)

	remains := New(w).
		WithPosition(transform.WorldPosition(from)).
		WithSprite(component.SpriteData{
			Image: component.Remains.Get(from).Image,
			Layer: component.LayerBackground,
			FlipX: component.Sprite.Get(from).FlipX,
		}).
		WithScale(scale).
		WithSpriteBounds().
		WithTimeToLive(domain.RemainsTimeToLive).
		With(component.Velocity).
		With(component.PositionLimit).
		Entry()

	component.Velocity.SetValue(remains, component.VelocityData{
		Velocity: math.Vec2{
			X: 0,
			Y: scale.Y,
		},
	})

	component.PositionLimit.SetValue(remains, component.PositionLimitData{
		Min: math.Vec2{},
		Max: math.Vec2{X: domain.MapWidth, Y: domain.MapHeight},
	})

	return remains
}
