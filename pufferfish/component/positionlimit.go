package component

import (
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/math"
)

type PositionLimitData struct {
	Min math.Vec2
	Max math.Vec2
}

var PositionLimit = donburi.NewComponentType[PositionLimitData]()
