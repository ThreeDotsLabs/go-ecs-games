# Timers

## How it works

`engine.Timer` counts **frames, not wall-clock time**. `NewTimer(d time.Duration)`
converts the duration to a frame target once (`ms * 60 / 1000` — hardcoding 60 TPS) and
`Update()` advances a counter. Nothing in any game measures delta time; a system or
animation calls `Update()` exactly once per tick on each timer it owns, then reads
`IsReady()` / `PercentDone()`.

**The 60-TPS rule is the invariant, not an accident.** Frame-count timers everywhere —
never wall-clock, never delta time — are what make two things cheap: the global
`TimeMultiplier` debug speed-up (see [debug](debug.md)) and deterministic
[replay](replay.md), where re-running the same inputs over the same ticks reproduces
the game.

## Canonical code

`engine/timer.go` (also `Duration()`, `DurationLeft()`,
`CurrentFrames()`, `TargetFrames()` accessors):

```go
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
```

`TimeMultiplier` is set from the debug UI (a game-speed slider in
`scene/game_debug.go`) — every timer in the game speeds up together because they all share this one
package var.

## Semantic uses to keep

- **Start ready via `Finish()`** — a cooldown-style timer that should allow the first
  use immediately. The player's turn grace (`archetype/player.go`):

  ```go
  turnTimer := engine.NewTimer(150 * time.Millisecond)
  // Start ready, so the first press in the spawn facing steps immediately.
  turnTimer.Finish()
  ```

  Same idea for spells: `s.CooldownTimer.Finish()` in `component.NewSpell` — spells
  start off cooldown (`component/caster.go`).
- **`IsStarted()` distinguishes in-progress from idle** — a cast wind-up
  uses it to tell "cast underway" (interrupt resets it) from "not casting" (tick the
  cooldown instead); see [state-machines](state-machines.md).
- **`SetTarget` to re-randomize an interval** — a spawner re-rolls each cycle:
  `spawner.Timer.Reset(); spawner.Timer.SetTarget(spawner.Interval.Random())`
  (`system/spawner.go`; see [ai](ai.md)).
- **`PercentDone()` drives all interpolation** — every tween and progress bar reads it;
  see [animations](animations.md).

## Rules

- Durations are written as `time.Duration` at call sites (`250 * time.Millisecond`),
  converted to frames once. Never do manual delta-time math.
- Timers live on components (or inside `Animation`s), never in systems; the system that
  owns the phase ticks its timer **exactly once per Update**.
- Assume 60 TPS everywhere. Don't "fix" it per-timer — if the tick rate ever changes,
  change the conversion in this one file.
- Keep `TimeMultiplier` applied only inside `Timer.Update` — that's why speeding the
  game up can't desync anything that uses timers.

## Pitfalls

- **Shared-pointer footgun**: putting a `*engine.Timer` in a component's donburi
  *default value* shares one timer across every entity (see [ecs-core](ecs-core.md)).
  Construct timers per entity in the archetype.
