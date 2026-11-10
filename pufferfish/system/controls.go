package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

type Controls struct {
	query *donburi.Query
}

func NewControls() *Controls {
	return &Controls{
		query: donburi.NewQuery(filter.Contains(component.Input)),
	}
}

func (s *Controls) Update(w donburi.World) {
	for entry := range s.query.Iter(w) {
		input := component.Input.Get(entry)
		velocity := component.Velocity.Get(entry)
		sprite := component.Sprite.Get(entry)

		speed := domain.PlayerSpeed

		if ebiten.IsKeyPressed(input.KeyUp) {
			velocity.Velocity.Y = -speed
		} else if ebiten.IsKeyPressed(input.KeyDown) {
			velocity.Velocity.Y = speed
		}

		if ebiten.IsKeyPressed(input.KeyLeft) {
			velocity.Velocity.X = -speed
			sprite.FlipX = true
		} else if ebiten.IsKeyPressed(input.KeyRight) {
			velocity.Velocity.X = speed
			sprite.FlipX = false
		}
	}
}
