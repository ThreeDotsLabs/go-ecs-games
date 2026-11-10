package archetype

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/assets"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
)

type fishKind struct {
	Sprite   *ebiten.Image
	Skeleton *ebiten.Image
	AI       donburi.IComponentType
}

var fishKinds = []fishKind{
	{Sprite: assets.BlueFish, Skeleton: assets.BlueFishSkeleton, AI: component.IdleAI},
	{Sprite: assets.RedFish, Skeleton: assets.RedFishSkeleton, AI: component.ChasePlayerAI},
	{Sprite: assets.GreenFish, Skeleton: assets.GreenFishSkeleton, AI: component.FleePlayerAI},
	{Sprite: assets.OrangeFish, Skeleton: assets.OrangeFishSkeleton, AI: component.RandomMovementAI},
}

func NewFish(w donburi.World, pos math.Vec2) *donburi.Entry {
	kind := engine.RandomFrom(fishKinds)
	scale := engine.RandomFloatRange(domain.FishMinScale, domain.FishMaxScale)

	fish := New(w).
		WithPosition(pos).
		WithSprite(component.SpriteData{
			Image: kind.Sprite,
			Layer: component.LayerBackground,
		}).
		WithScale(math.Vec2{
			X: scale,
			Y: scale,
		}).
		With(component.Velocity).
		With(component.Drag).
		With(component.PositionLimit).
		With(component.Killable).
		With(kind.AI).
		With(component.Remains).
		WithSpriteBounds().
		Entry()

	// Slightly smaller bounds so that the fish can overlap a bit before colliding
	bounds := component.Bounds.Get(fish)
	bounds.Width -= 20
	bounds.Height -= 20

	component.Killable.SetValue(fish, component.KillableData{
		Points: domain.FishKillPenalty,
	})

	component.Remains.SetValue(fish, component.RemainsData{
		Image: kind.Skeleton,
	})

	component.PositionLimit.SetValue(fish, component.PositionLimitData{
		Min: math.Vec2{},
		Max: math.Vec2{X: domain.MapWidth, Y: domain.MapHeight},
	})

	return fish
}
