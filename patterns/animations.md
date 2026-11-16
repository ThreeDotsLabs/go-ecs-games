# Animations & tweens

## How it works

There is no tween library and no animation asset format. One component holds named
**closures**; one ~15-line system ticks them. An `Animation` is an optional frame-count
[Timer](timers.md), an `Update` closure run every tick while `Active`, and start/stop
hooks. **The animation decides when to stop** — the system only ticks the timer and
calls `Update`. One struct expresses everything: a one-shot tween (`Update` calls
`a.Stop(e)` when the timer is ready), a loop (`Update` resets the timer, never stops),
a delay (`Update` waits, then triggers the next thing). Interpolation is
`Timer.PercentDone()` fed by hand through an `engine/lerp.go` easing into
`engine.Lerp`/`LerpVec2`.

When the rules have already decided the outcome (a die roll), the animation only
presents it: play back motion that lands on the decided result, and draw any cosmetic
randomness from a separate presentation RNG so it can never change gameplay.

## Canonical code

**Component** (`component/animator.go`; `AnimatorData` also has
`SetAnimation`/`Start`/`Stop` by name, panicking on an unknown name):

```go
// AnimatorData holds an entity's animations by name. Gameplay code starts and
// stops them; system.Animation ticks the active ones every frame.
type AnimatorData struct {
    Animations map[string]*Animation
}

// Animation runs Update every tick while Active, alongside an optional Timer
// that Start resets — Update decides itself when to call Stop (one-shot tweens
// stop when the timer is ready; loops never do). The OneShot hooks fire once
// and clear themselves; the plain hooks fire on every Start/Stop.
type Animation struct {
    Active         bool
    Timer          *engine.Timer
    Update         func(e *donburi.Entry, a *Animation)
    OnStart        func(e *donburi.Entry)
    OnStartOneShot []func(e *donburi.Entry)
    OnStop         func(e *donburi.Entry)
    OnStopOneShot  []func(e *donburi.Entry)
}

func (a *Animation) Start(e *donburi.Entry) {
    if a.Active {
        return
    }
    a.Active = true
    if a.Timer != nil {
        a.Timer.Reset()
    }
    if a.OnStart != nil {
        a.OnStart(e)
    }
    for _, f := range a.OnStartOneShot {
        f(e)
    }
    a.OnStartOneShot = nil
}

var Animator = donburi.NewComponentType[AnimatorData]()
```

`Start` **no-ops when already active** (restart = `Stop` first), resets the timer, and
drains the one-shots; `Stop` mirrors it — clears `Active`, fires `OnStop`, drains `OnStopOneShot`.

**System** (`system/animation.go`, the whole `Update`):

```go
func (s *Animation) Update(w donburi.World) {
    s.query.Each(w, func(entry *donburi.Entry) {
        animator := component.Animator.Get(entry)
        for _, animation := range animator.Animations {
            if !animation.Active {
                continue
            }
            if animation.Timer != nil {
                animation.Timer.Update()
            }
            if animation.Update != nil {
                animation.Update(entry, animation)
            }
        }
    })
}
```

Register it **after** the systems that start animations, so tweens advance the same tick (ordering comments in `scene/game.go`).

## Choreography patterns

**One-shot tween** (`system/interact.go`):

```go
animator.SetAnimation("glide", &component.Animation{
    Timer: engine.NewTimer(duration),
    Update: func(e *donburi.Entry, a *component.Animation) {
        transform.GetTransform(e).LocalPosition = engine.LerpVec2(from, to, ease(a.Timer.PercentDone()))
        if a.Timer.IsReady() {
            a.Stop(e)
        }
    },
})
```

**Shared-slot preemption** — animations that fight over one property share one *name*.
In `system/interact.go` the pick-up scoop and the put-down fall are both named
`"glide"`, so starting either replaces the other mid-flight from the item's current
position; the looping `"carry"` animation *yields* while a glide owns the position — it
checks `Animations["glide"].Active` each tick and waits, then resumes flush.

**Self-resetting loop** (`scene/game.go`, selected-tile pulse):

```go
selectedAnimator.SetAnimation("pulse", &component.Animation{
    Timer: engine.NewTimer(1200 * time.Millisecond),
    Update: func(e *donburi.Entry, a *component.Animation) {
        if a.Timer.IsReady() {
            a.Timer.Reset()
        }
        phase := stdmath.Sin(2 * stdmath.Pi * a.Timer.PercentDone())
        component.Sprite.Get(e).AlphaOverride.A = 0.55 + 0.15*phase
    },
})
```

**Lazily-built, capturing rest state** — `system/rustle.go` builds the
foliage shiver on first trigger, capturing the rest `LocalScale` into the closure then
(safe because foliage never animates otherwise); `OnStop` restores it.

**Delay → show chain** — each dialog paragraph gets a `"delay"` animation
(built `Active: true`, so it runs without a `Start`) whose `Update` only waits for its
timer, then calls `animator.Animations["show"].Start(e)` — the visible typing or fade-in
effect — and stops itself (`archetype/dialog.go`).

**`OnStopOneShot` for "then do X"** — sequencing without a scheduler
(`archetype/dialog.go`):

```go
anim := component.Animator.Get(dialogCamera).Animations["fade-out"]
anim.Start(dialogCamera)
if onHide != nil {
    anim.OnStopOneShot = append(anim.OnStopOneShot, onHide)
}
```

Callers pass `hideDialog(w, func(e *donburi.Entry) { ChangeLevel(w, lvl) })` — the level switch runs once, when the fade completes.

## More uses of the same animator

- **Level cross-fade** (`archetype/level.go`): `"transition"` fades `cam.TransitionAlpha`
  up; `ChangeLevel` appends an `OnStop` doing `transform.RemoveRecursive(e)` +
  `NewLevel(w, level)`; the new level's own transition (built `Active: true`) fades back
  in. `OnStart`/`OnStop` disable and re-enable input. See [cameras](cameras.md).
- **Dialog camera fades**: `"fade-in"`/`"fade-out"` ease `cam.AlphaOverride` with
  `engine.EaseInOut` (`archetype/dialog.go`); **POI fades** start on story events —
  `StoryFactSetEvent.Subscribe` re-checks conditions (`archetype/poi.go`).
- **Walk cycle as frame-swap closure** (`archetype/character.go`): a 150ms self-resetting
  timer advances `currentFrame` through the frames; `OnStop` restores the idle frame.

## Rules

- Archetypes and gameplay systems build and `Start` animations; **only
  `system.Animation` ticks them**. Never call `a.Timer.Update()` inside your own
  `Update` — doing both runs the animation at double speed.
- The `Update` closure owns the stop decision — no "done" flags on other components.
- Restore rest state in `OnStop`, not on the last `Update` frame — preemption and external `Stop` calls skip it.
- Capture plain values (rest scale, endpoints) freely, but **re-`Get` component data
  inside `Update`** — captured component pointers go stale when the entity's component
  set changes (documented in `system/interact.go`; see [ecs-core](ecs-core.md)).

## Pitfalls

- **No data-driven spritesheet frame animation exists** (frames + fps + loop mode) —
  walk cycles are hand-rolled closures. Design fresh when first needed ([catalog](README.md) staple).
- The typewriter effect (`Text.Streaming`) must slice **runes**, not bytes:
  `utf8.RuneCountInString` scaled by `PercentDone()`, then `t.Text[:length]` breaks
  non-ASCII ([fonts](fonts.md)).
- Keep both nil-guards: `Timer` in `Start`, `Update` in the system.
