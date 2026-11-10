package engine

import (
	"time"
)

// TimeMultiplier can be used to speed up or slow down the timer in development or testing.
var TimeMultiplier = 1

type Timer struct {
	currentFrames int
	targetFrames  int
}

func NewTimer(d time.Duration) *Timer {
	t := &Timer{}
	t.SetTarget(d)

	return t
}

func (t *Timer) Update() {
	if t.currentFrames < t.targetFrames {
		t.currentFrames += TimeMultiplier
	}
}

func (t *Timer) SetTarget(target time.Duration) {
	t.targetFrames = int(target.Milliseconds()) * 60 / 1000
}

func (t *Timer) Duration() time.Duration {
	return time.Duration(t.targetFrames) * time.Millisecond * 1000 / 60
}

func (t *Timer) DurationLeft() time.Duration {
	if t.currentFrames >= t.targetFrames {
		return 0
	}
	return time.Duration(t.targetFrames-t.currentFrames) * time.Millisecond * 1000 / 60
}

func (t *Timer) IsReady() bool {
	return t.currentFrames >= t.targetFrames
}

func (t *Timer) Reset() {
	t.currentFrames = 0
}

func (t *Timer) Finish() {
	t.currentFrames = t.targetFrames
}

func (t *Timer) IsStarted() bool {
	return t.currentFrames > 0
}

func (t *Timer) PercentDone() float64 {
	return float64(t.currentFrames) / float64(t.targetFrames)
}

func (t *Timer) SetProgress(progress float64) {
	t.currentFrames = int(progress * float64(t.targetFrames))
}

func (t *Timer) CurrentFrames() int {
	return t.currentFrames
}

func (t *Timer) TargetFrames() int {
	return t.targetFrames
}
