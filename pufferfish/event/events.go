package event

import (
	"github.com/yohamta/donburi/features/events"
)

type CollectedData struct {
	Points int
}

var Collected = events.NewEventType[CollectedData]()

type KilledData struct {
	Points int
}

var Killed = events.NewEventType[KilledData]()
