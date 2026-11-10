package system

import (
	"fmt"

	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/event"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

type Scoring struct {
	query *donburi.Query
}

func NewScoring() *Scoring {
	return &Scoring{
		query: donburi.NewQuery(
			filter.Contains(component.Score),
		),
	}
}

func (s *Scoring) Subscribe(w donburi.World) {
	event.Killed.Subscribe(w, s.onKilled)
	event.Collected.Subscribe(w, s.onCollected)
}

func (s *Scoring) onKilled(w donburi.World, e event.KilledData) {
	s.updateScore(w, e.Points)
}

func (s *Scoring) onCollected(w donburi.World, e event.CollectedData) {
	s.updateScore(w, e.Points)
}

func (s *Scoring) updateScore(w donburi.World, delta int) {
	entry, ok := s.query.First(w)
	if !ok {
		return
	}
	score := component.Score.Get(entry)

	score.Score += delta
	component.Text.Get(entry).Text = fmt.Sprintf("%v", score.Score)

	// Update UI position
	player, ok := transform.GetParent(entry)
	if !ok {
		return
	}

	spriteHeight := float64(component.Sprite.Get(player).Image.Bounds().Dy())
	scale := transform.GetTransform(player).LocalScale
	transform.GetTransform(entry).LocalPosition.Y = spriteHeight * scale.Y / 2
}
