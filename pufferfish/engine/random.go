package engine

import (
	"math/rand"
	"time"
)

func RandomFloatRange(min, max float64) float64 {
	return min + rand.Float64()*(max-min)
}

func RandomIntRange(min, max int) int {
	return min + rand.Intn(max-min+1)
}

func RandomDuration(min, max time.Duration) time.Duration {
	return time.Duration(RandomFloatRange(float64(min), float64(max)))
}

func RandomFrom[T any](list []T) T {
	index := rand.Intn(len(list))
	return list[index]
}
