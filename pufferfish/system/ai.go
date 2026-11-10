package system

import (
	"time"

	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

var cardinalDirections = []math.Vec2{
	{X: 0, Y: -1}, // Up
	{X: 0, Y: 1},  // Down
	{X: 1, Y: 0},  // Right
	{X: -1, Y: 0}, // Left
}

type AI struct {
	query       *donburi.Query
	playerQuery *donburi.Query
}

func NewAI() *AI {
	return &AI{
		query: donburi.NewQuery(
			filter.Or(
				filter.Contains(component.IdleAI),
				filter.Contains(component.RandomMovementAI),
				filter.Contains(component.ChasePlayerAI),
				filter.Contains(component.FleePlayerAI),
			),
		),
		playerQuery: donburi.NewQuery(
			filter.Contains(component.Player),
		),
	}
}

func (a *AI) Update(w donburi.World) {
	for entry := range a.query.Iter(w) {
		if entry.HasComponent(component.IdleAI) {
			a.handleIdleAI(entry)
		} else if entry.HasComponent(component.RandomMovementAI) {
			a.handleRandomMovementAI(entry)
		} else if entry.HasComponent(component.ChasePlayerAI) {
			a.handleChasePlayerAI(w, entry)
		} else if entry.HasComponent(component.FleePlayerAI) {
			a.handleFleePlayerAI(w, entry)
		} else {
			panic("entity has AI component but no known AI type")
		}
	}
}

func (a *AI) handleIdleAI(entry *donburi.Entry) {
	ai := component.IdleAI.Get(entry)
	velocity := component.Velocity.Get(entry)

	if ai.HorizontalTimer == nil {
		ai.HorizontalTimer = engine.NewTimer(2 * time.Second)
		ai.VerticalTimer = engine.NewTimer(time.Second)
		ai.HorizontalTimer.Finish()
		ai.VerticalTimer.Finish()
	}

	// Idle behavior - swim up and down a bit, and left and right on the timer
	ai.HorizontalTimer.Update()
	ai.VerticalTimer.Update()

	if ai.HorizontalTimer.IsReady() {
		ai.HorizontalTimer.Reset()
		ai.HorizontalTimer.SetTarget(engine.RandomDuration(1*time.Second, 4*time.Second))
		ai.MovingLeft = !ai.MovingLeft

		component.Sprite.Get(entry).FlipX = ai.MovingLeft

		horizontalAcc := engine.RandomFloatRange(1.0, 3.0)
		if ai.MovingLeft {
			horizontalAcc *= -1
		}
		velocity.Velocity.X = horizontalAcc
	}

	if ai.VerticalTimer.IsReady() {
		ai.VerticalTimer.Reset()
		ai.VerticalTimer.SetTarget(engine.RandomDuration(500*time.Millisecond, 1500*time.Millisecond))
		ai.MovingUp = !ai.MovingUp

		verticalAcc := engine.RandomFloatRange(0.3, 0.7)
		if ai.MovingUp {
			verticalAcc *= -1
		}
		velocity.Velocity.Y = verticalAcc
	}
}

func (a *AI) handleRandomMovementAI(entry *donburi.Entry) {
	ai := component.RandomMovementAI.Get(entry)

	if ai.Timer == nil {
		ai.Timer = engine.NewTimer(time.Second)
		ai.Timer.Finish()
	}

	velocity := component.Velocity.Get(entry)

	ai.Timer.Update()
	if !ai.Timer.IsReady() {
		velocity.Velocity = ai.CurrentVelocity
		return
	}

	t := transform.GetTransform(entry)

	direction := engine.RandomFrom(cardinalDirections)
	speed := domain.FishSpeed(engine.RandomFloatRange(0.3, 1.5), t.LocalScale.X)
	ai.CurrentVelocity = direction.MulScalar(speed)
	velocity.Velocity = ai.CurrentVelocity

	component.Sprite.Get(entry).FlipX = ai.CurrentVelocity.X < 0

	ai.Timer.SetTarget(engine.RandomDuration(3*time.Second, 7*time.Second))
	ai.Timer.Reset()
}

func (a *AI) handleChasePlayerAI(w donburi.World, entry *donburi.Entry) {
	ai := component.ChasePlayerAI.Get(entry)

	if ai.FallBackTimer == nil {
		ai.State = component.ChaseStateWaiting
		ai.FallBackTimer = engine.NewTimer(engine.RandomDuration(2*time.Second, 5*time.Second))
		ai.WaitTimer = engine.NewTimer(engine.RandomDuration(1*time.Second, 4*time.Second))
	}

	player, ok := a.playerQuery.First(w)
	if !ok {
		return
	}

	velocity := component.Velocity.Get(entry)

	directionToPlayer := transform.WorldPosition(player).Sub(transform.WorldPosition(entry)).Normalized()

	t := transform.GetTransform(entry)

	switch ai.State {
	case component.ChaseStateChasing:
		distanceToPlayer := component.Bounds.Get(entry).Rect(entry).Distance(component.Bounds.Get(player).Rect(player))

		if distanceToPlayer < domain.ChaseFallBackDistance || distanceToPlayer > domain.ChaseRange {
			ai.State = component.ChaseStateFallingBack
			return
		}

		speed := domain.FishSpeed(engine.RandomFloatRange(0.7, 1.2), t.LocalScale.X)
		velocity.Velocity = directionToPlayer.MulScalar(speed)
		component.Sprite.Get(entry).FlipX = velocity.Velocity.X < 0

	case component.ChaseStateFallingBack:
		if ai.FallBackTimer.IsReady() {
			ai.State = component.ChaseStateWaiting
			ai.FallBackTimer.Reset()
			return
		}

		ai.FallBackTimer.Update()
		speed := engine.RandomFloatRange(0.3, 0.4)
		velocity.Velocity = directionToPlayer.MulScalar(-speed)
		component.Sprite.Get(entry).FlipX = velocity.Velocity.X < 0
	case component.ChaseStateWaiting:
		if ai.WaitTimer.IsReady() {
			distanceToPlayer := component.Bounds.Get(entry).Rect(entry).Distance(component.Bounds.Get(player).Rect(player))

			if distanceToPlayer < domain.ChaseRange {
				ai.State = component.ChaseStateChasing
				ai.WaitTimer.Reset()
			}

			return
		}

		ai.WaitTimer.Update()
	}
}

func (a *AI) handleFleePlayerAI(w donburi.World, entry *donburi.Entry) {
	ai := component.FleePlayerAI.Get(entry)

	if ai.FleeTimer == nil {
		ai.State = component.FleeStateFleeing
		ai.FleeTimer = engine.NewTimer(engine.RandomDuration(1*time.Second, 4*time.Second))
	}

	player, ok := a.playerQuery.First(w)
	if !ok {
		return
	}

	velocity := component.Velocity.Get(entry)

	directionToPlayer := transform.WorldPosition(player).Sub(transform.WorldPosition(entry)).Normalized()

	t := transform.GetTransform(entry)

	switch ai.State {
	case component.FleeStateFleeing:
		if ai.FleeTimer.IsReady() {
			ai.State = component.FleeStateReturning
			ai.FleeTimer.Reset()
			return
		}

		ai.FleeTimer.Update()

		speed := domain.FishSpeed(engine.RandomFloatRange(1.2, 1.5), t.LocalScale.X)
		velocity.Velocity = directionToPlayer.MulScalar(-speed)
		component.Sprite.Get(entry).FlipX = velocity.Velocity.X < 0

	case component.FleeStateReturning:
		distanceToPlayer := component.Bounds.Get(entry).Rect(entry).Distance(component.Bounds.Get(player).Rect(player))

		if distanceToPlayer < domain.FleeDistance {
			ai.State = component.FleeStateFleeing
			return
		}

		speed := domain.FishSpeed(engine.RandomFloatRange(0.1, 0.5), t.LocalScale.X)
		velocity.Velocity = directionToPlayer.MulScalar(speed)
		component.Sprite.Get(entry).FlipX = velocity.Velocity.X < 0
	}
}
