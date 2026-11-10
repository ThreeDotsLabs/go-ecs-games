package domain

import "time"

const (
	FishKillPenalty = -10

	FishMinScale = 0.4
	FishMaxScale = 1.2

	ChaseFallBackDistance = 75.0
	ChaseRange            = 400.0
	FleeDistance          = 150.0

	// RemainsTimeToLive is how long a skeleton stays on the map after the fish is killed.
	RemainsTimeToLive = 3 * time.Minute
)

// FishSpeed makes bigger fish slower.
func FishSpeed(base float64, scale float64) float64 {
	return base / scale
}
