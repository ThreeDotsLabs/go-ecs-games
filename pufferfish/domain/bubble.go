package domain

import "time"

const (
	BubbleMinPoints = 1
	BubbleMaxPoints = 3

	BubbleGrowth = 0.02

	// BubbleTimeToLive is long enough for the slowest bubble to float off the top of the map.
	BubbleTimeToLive = 60 * time.Second
)
