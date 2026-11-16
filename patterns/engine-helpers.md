# engine/ helpers

## How it works

`engine/` is the game-agnostic toolbox: math, geometry, timing, RNG, and generic
donburi lookups. It imports **only** donburi + stdlib — never `component/`, `assets/`,
or anything game-specific. The test: **if a helper would need to import `component/`,
it doesn't belong in `engine/`** — it belongs in that component's file or in a system.
Before writing a helper, check this list; it probably already exists. Extend rarely.

## File by file

**`timer.go`** — frame-based countdown, the only sanctioned way to track time
(see [timers](timers.md)). Verbatim core (`engine/timer.go`):

```go
// TimeMultiplier can be used to speed up or slow down the timer in development or testing.
var TimeMultiplier = 1

type Timer struct {
	currentFrames int
	targetFrames  int
}

func (t *Timer) Update() {
	if t.currentFrames < t.targetFrames {
		t.currentFrames += TimeMultiplier
	}
}
```

Full API: `NewTimer(d time.Duration)`, `Update()`, `IsReady()`, `IsStarted()`,
`Reset()`, `Finish()`, `SetTarget(d)`, `SetProgress(p)`, `PercentDone()`,
`Duration()`, `DurationLeft()`. `SetTarget` converts duration to frames at exactly
60 TPS: `int(target.Milliseconds()) * 60 / 1000`.

**`lerp.go`** — interpolation and easing (`engine/lerp.go`):

```go
func Lerp(a, b, t float64) float64
func LerpVec2(a, b math.Vec2, t float64) math.Vec2
func EaseIn(t float64) float64      // plus EaseOut, EaseInOut
func CubicEaseIn(t float64) float64 // plus CubicEaseOut
func EaseOutBack(t float64) float64 // overshoots past 1, settles back
```

**`rect.go`** — the float AABB used by colliders (`engine/rect.go`):

```go
type Rect struct{ X, Y, Width, Height float64 }

func NewRect(x, y, width, height float64) Rect
func (r Rect) MaxX() float64 // plus MaxY, Position, Size
func (r Rect) Intersects(other Rect) bool
func (r Rect) Move(pos math.Vec2) Rect
func (r Rect) Scale(scale float64) Rect
func (r Rect) ToImageRectangle() image.Rectangle
```

**`range.go`** — min/max pairs that roll themselves (`engine/range.go`):

```go
type IntRange struct{ Min, Max int }

func (r IntRange) Random() int
// FloatRange and DurationRange follow the same shape
```

**`random.go`** — RNG helpers (`engine/random.go`):

```go
func RandomFloatRange(min, max float64) float64
func RandomIntRange(min, max int) int // max-INCLUSIVE: min + rand.Intn(max-min+1)
func RandomFrom[T comparable](list []T) T
func RandomFromOrEmpty[T comparable](list []T) *T
```

**`clamp.go`** `Clamp(value, min, max float64) float64` · **`percent.go`**
`IntPercent(value int, percent float64) int` · **`size.go`**
`Size{Width, Height int}` · **`snap.go`** `Snap(value, grid float64) float64` —
rounds to the nearest multiple of `grid`.

**`find.go`** — donburi lookups; `Find*` return `(value, ok)`, `Must*` panic — use
`Must*` for guaranteed singletons and setup, `Find*` in the game loop
(see [singletons](singletons.md)) (`engine/find.go`):

```go
func MustFindComponent[T any](w donburi.World, c Component[T]) *T // generic: singleton's data or panic
func FindWithComponent(w donburi.World, componentType component.IComponentType) (*donburi.Entry, bool)
func MustFindWithComponent(w donburi.World, componentType component.IComponentType) *donburi.Entry
func MustGetParent(entry *donburi.Entry) *donburi.Entry
func MustFindChildWithComponent(parent *donburi.Entry, componentType component.IComponentType) *donburi.Entry
func MustFindGrandchildWithComponent(parent *donburi.Entry, componentType component.IComponentType) *donburi.Entry
func FindChildrenWithComponent(parent *donburi.Entry, componentType component.IComponentType) []*donburi.Entry
```

`MustFindComponent` backs `component.MustFindGame`; the grandchild variant searches
depth-first through the whole subtree; `FindChildrenWithComponent` checks direct
children only and never returns nil.

## Rules

- `engine/` imports only donburi + stdlib. No `component/`, ever.
- `RandomIntRange` (and therefore `IntRange.Random()` / `DurationRange.Random()`) is
  **max-inclusive**. This is the prescribed behavior — document it, don't "fix" it.
- New generic code goes here only when it stays generic; anything reading game
  components each tick is a system ([ecs-core](ecs-core.md)).

## Gotchas

- **Easing functions don't clamp `t`** — they are plain polynomials; clamp the input
  yourself when `t` can leave `[0,1]` (`EaseOutBack` leaves it deliberately).
- **`Rect` is AABB-only** — no rotation; `Intersects` uses `<=`, so touching edges
  count as intersecting.
- **`Timer` assumes exactly 60 TPS** — a documented rule of the whole codebase, not an
  accident; it's what makes the global `TimeMultiplier` speed-up work. A zero-duration
  timer is `IsReady()` immediately, but its `PercentDone()` is NaN (0/0).
- **`RandomFrom` panics on an empty slice** (`rand.Intn(0)` panics). `RandomFromOrEmpty`
  never panics but is not merely the safe variant: it rolls one slot past the end by
  design and returns `nil` with probability `1/(len+1)` even for non-empty input.

## Pitfalls

- Don't keep tombstone files ("the function is now in find.go") — they get copied
  from game to game forever.
- `engine/input/` needs only a build-tagged `IsTouchPrimaryInput()` pair
  (`touch.go` / `touch_mobile.go`) and only when shipping
  [touch](input-touch.md)/[mobile](mobile.md).
- **`RandomIntRange` is max-inclusive.** A max-exclusive version
  (`min + rand.Intn(max-min)`) is an easy slip; mixing the two shifts every range
  constant by one.
