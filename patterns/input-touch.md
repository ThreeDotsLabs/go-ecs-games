# Input: touch

## How it works

All touch handling lives in **one system**. It owns the gesture state (touch IDs, timers,
flags) as system fields and each tick reduces raw `ebiten.AppendTouchIDs` input to derived
results the rest of the game consumes: a click position, camera scroll, a zoom factor.
Gameplay code never sees a `TouchID` (see [input-cursor.md](input-cursor.md) for the
single-pointer version of the same rule).

The state machine branches on **finger count** — `0` resolves taps and ends
scroll/pinch, `1` disambiguates tap vs drag, `2` pinches, more cancels everything.
Whether a touch started on UI is tracked separately (`isTouchingUI`): UI touches click
immediately on press; map touches wait to see if they become a drag.

## Canonical code

**Tap vs drag-scroll disambiguation** — the core. A touch is a *tap* if the finger lifts
within 200ms; it becomes a *drag* the moment it is held longer or moves more than 10px
(`system/controls.go`, trimmed):

```go
const (
    scrollTriggerDist = 10
    touchDuration     = 200 * time.Millisecond
)

switch len(touchIDs) {
case 0:
    if c.isTouching && !c.isTouchingUI {
        // Finger lifted within given duration, single tap
        if !c.touchTimer.IsReady() {
            c.isTouching = false
            clicked = true
            cursorX, cursorY = int(c.touchPos.X), int(c.touchPos.Y)
        }
    }
    // ...end scrolling / pinching...
case 1:
    if c.isTouching {
        if !c.isTouchingUI {
            cx, cy := ebiten.TouchPosition(touchIDs[0])
            cursor := math.Vec2{X: float64(cx), Y: float64(cy)}
            dist := c.touchPos.Distance(cursor)
            c.touchTimer.Update()
            if c.touchTimer.IsReady() || dist > scrollTriggerDist {
                c.isTouching = false
                c.scrollTouchID = touchIDs[0]
                c.isTouchScrolling = true
                c.startedScrolling = true
            }
        }
    } else if !c.isTouchScrolling {
        // New touch: record position, decide UI vs map
        c.isTouching = true
        c.touchPos = math.Vec2{X: float64(cursorX), Y: float64(cursorY)}
        if uiPanelUnderCursor == nil {
            c.touchTimer.Reset() // map touched — start the tap window
        } else {
            clicked = true       // UI touched — click immediately
            c.isTouchingUI = true
        }
    }
```

Note the asymmetry: a resolved *tap* clicks at the **press** position (`c.touchPos`), not
wherever the finger drifted to before lifting.

**Two-finger pinch** — the ratio of the between-finger distance drives the zoom factor
the shared camera-zoom code (also fed by the mouse wheel) applies:

```go
case 2:
    if c.isPinching {
        dist := distanceBetweenTouchIDs(touchIDs[0], touchIDs[1])
        if dist != 0 {
            if dist > c.pinchDistance {
                c.zoomFactor = dist / c.pinchDistance * touchZoomStep
            } else if dist < c.pinchDistance {
                c.zoomFactor = c.pinchDistance / dist * -touchZoomStep
            }
            c.pinchDistance = dist
        }
    } else if /* both fingers off UI */ {
        c.isPinching = true
        c.pinchDistance = distanceBetweenTouchIDs(touchIDs[0], touchIDs[1])
    }

func distanceBetweenTouchIDs(touchID1, touchID2 ebiten.TouchID) float64 {
    x1, y1 := ebiten.TouchPosition(touchID1)
    x2, y2 := ebiten.TouchPosition(touchID2)
    return stdmath.Hypot(float64(x2-x1), float64(y2-y1))
}
```

**Platform capability via build tags** — one exported predicate, two seven-line files
(`system/controls_desktop.go` / `controls_mobile.go`):

```go
//go:build !android && !ios
package system
func UseTouchControls() bool { return false }

//go:build android || ios
package system
func UseTouchControls() bool { return true }
```

The capability changes derived config too, not just input paths — touch gets a wider
zoom range because pinch wants headroom (`scene/board.go`):

```go
func standardCameraRange() engine.FloatRange {
    if system.UseTouchControls() {
        return engine.FloatRange{Min: 0.5, Max: 1.5}
    }
    return engine.FloatRange{Min: 0.5, Max: 1}
}
```

## Debugging gestures on device

You cannot printf a phone. Build the instrumentation in (crossref
[debug.md](debug.md)):

- The Controls system also implements `Draw` (registered in both slices) and prints its
  whole state machine — `Touching/UI/timer`, `Touch scrolling + touch ID`,
  `Map scrolled`, `Pinching + zoomFactor` — every frame in debug mode.
- `spawnClickDebug` drops an ephemeral X-marker entity (`TimeToLive`, 2s) at the event
  position, color-coded: **lime green** for a raw touch registering, **magenta** for a
  resolved click. Watching the colors on device tells you which half of the pipeline
  dropped your gesture.

**Reaching debug mode on iOS at all**: a secret gesture — one long press
(≥1s) followed by two short ones, tracked with two timers
(`system/debug.go`): `longClickTimer` (1s, reset on every press;
ready at release ⇒ that press was "long") and `betweenClicksTimer` (500ms, resets the
recorded sequence when it expires). A release appends 1 or 0; sequence `[1,0,0]`
toggles debug. Works with mouse clicks too, so it's testable on desktop.

## Rules

- **Touch handling lives in ONE system**; everything else reads derived state. The
  companion rule: that system only *recognizes* gestures. Reactions (tile selection,
  camera moves) go in their own systems, or recognition grows into a 683-line
  `controls.go` (see [ecs-core.md](ecs-core.md)).
- Track the `TouchID` you started a gesture with (`scrollTouchID`) and keep reading that
  ID — never `touchIDs[0]` mid-gesture, fingers reorder.
- Decide UI-vs-world at touch *start* and remember it; UI clicks immediately, world
  waits out the tap window.
- Gate touch paths on `UseTouchControls()`, not runtime heuristics — a laptop with a
  touchscreen still gets mouse controls, and that's fine.
- Timers, not frame counts, for gesture windows (`engine.Timer`, see
  [timers.md](timers.md)).

## Pitfalls

- Touch positions are **screen** coordinates; convert to world through the camera
  wherever you hit-test the world (see [cameras.md](cameras.md) and
  [input-cursor.md](input-cursor.md)).
- The build-tag predicate (`IsTouchPrimaryInput()`) lives in one place per game, in
  `engine/input/` or `system/`, not both.
- Multi-touch UI (two simultaneous UI touches) isn't handled — two fingers on UI just
  cancels; design fresh if ever needed.

## Variations

- **A virtual joystick that spawns where the thumb lands** (`system/controls.go`): the
  touch start sets the stick's center, the drag vector (clamped to a radius) is the move
  input, release hides it. Fits action games where tap-to-target doesn't.
