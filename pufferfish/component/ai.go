package component

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
)

type IdleAIData struct {
	MovingLeft bool
	MovingUp   bool

	HorizontalTimer *engine.Timer
	VerticalTimer   *engine.Timer
}

var IdleAI = donburi.NewComponentType[IdleAIData]()

type RandomMovementAIData struct {
	CurrentVelocity math.Vec2
	Timer           *engine.Timer
}

var RandomMovementAI = donburi.NewComponentType[RandomMovementAIData]()

type ChaseState int

const (
	ChaseStateChasing ChaseState = iota
	ChaseStateFallingBack
	ChaseStateWaiting
)

type ChasePlayerAIData struct {
	State         ChaseState
	FallBackTimer *engine.Timer
	WaitTimer     *engine.Timer
}

var ChasePlayerAI = donburi.NewComponentType[ChasePlayerAIData]()

type FleeState int

const (
	FleeStateFleeing FleeState = iota
	FleeStateReturning
)

type FleePlayerAIData struct {
	State     FleeState
	FleeTimer *engine.Timer
}

var FleePlayerAI = donburi.NewComponentType[FleePlayerAIData]()
