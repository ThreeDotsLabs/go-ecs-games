package archetype

import (
	"time"

	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/assets"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
)

var bubbleImages = []*ebiten.Image{
	assets.Bubble1,
	assets.Bubble2,
	assets.Bubble3,
}

func NewBubbleSpawner(w donburi.World, pos math.Vec2) *donburi.Entry {
	spawner := New(w).
		WithPosition(pos).
		With(component.BubbleSpawner).
		Entry()

	component.BubbleSpawner.SetValue(spawner, component.BubbleSpawnerData{
		SpawnTimer: engine.NewTimer(engine.RandomDuration(10*time.Second, 15*time.Second)),
	})

	return spawner
}

func NewBubbleCluster(w donburi.World, parent *donburi.Entry) {
	for range engine.RandomIntRange(1, 3) {
		newBubble(w, parent)
	}
}

func newBubble(w donburi.World, parent *donburi.Entry) *donburi.Entry {
	pos := math.Vec2{
		X: engine.RandomFloatRange(-200, 200),
		Y: engine.RandomFloatRange(-200, 200),
	}

	bubble := New(w).
		WithParent(parent).
		WithPosition(pos).
		WithSprite(component.SpriteData{
			Image: engine.RandomFrom(bubbleImages),
			Layer: component.LayerForeground,
		}).
		WithSpriteBounds().
		WithTimeToLive(domain.BubbleTimeToLive).
		With(component.Velocity).
		With(component.Collectable).
		Entry()

	component.Collectable.SetValue(bubble, component.CollectableData{
		Points: engine.RandomIntRange(domain.BubbleMinPoints, domain.BubbleMaxPoints),
	})

	component.Velocity.SetValue(bubble, component.VelocityData{
		Velocity: math.Vec2{
			X: 0,
			Y: engine.RandomFloatRange(-1.5, -0.5),
		},
	})

	return bubble
}
