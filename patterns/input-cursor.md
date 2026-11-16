# Input: cursor abstraction

## How it works

A `Cursor` **singleton** unifies mouse and touch into one pointer: a position, a
one-tick `JustInteracted` flag, and the touch ID currently being tracked. The Cursor
system runs **first** in the scene's systems slice (`scene/game.go`), so every
later system sees this tick's pointer state. Consumers (hover, select, buttons) read only
the singleton — they never call `ebiten.CursorPosition` or touch APIs themselves. This is
the single-pointer little sibling of the gesture machine
([input-touch.md](input-touch.md)).

Semantics differ by device on purpose: mouse *press* interacts immediately; touch
interacts on *release* (press picks the touch ID to track, moves update the position),
which is how taps feel right on phones.

## Canonical code

**The component** (`component/cursor.go`):

```go
type CursorData struct {
    Position       math.Vec2
    JustInteracted bool
    LastTouchID    *ebiten.TouchID
}

var Cursor = donburi.NewComponentType[CursorData]()

func MustFindCursor(w donburi.World) *CursorData {
    entry := engine.MustFindWithComponent(w, Cursor)
    return Cursor.Get(entry)
}
```

**The system** — one touch tracked press→move→release (`system/cursor.go`):

```go
func (s *Cursor) Update(w donburi.World) {
    cursor := component.MustFindCursor(w)
    cursor.JustInteracted = false

    touchIDs := ebiten.AppendTouchIDs(nil)
    if cursor.LastTouchID == nil && len(touchIDs) > 0 {
        // Touch started - this is the ID we want to track now
        cursor.LastTouchID = &touchIDs[0]
        x, y := ebiten.TouchPosition(touchIDs[0])
        cursor.Position.X = float64(x)
        cursor.Position.Y = float64(y)
        return
    }

    if cursor.LastTouchID == nil {
        x, y := ebiten.CursorPosition()
        cursor.Position.X = float64(x)
        cursor.Position.Y = float64(y)
        if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
            cursor.JustInteracted = true
        }
    } else {
        var found bool
        for _, id := range touchIDs {
            if id == *cursor.LastTouchID {
                found = true
                break
            }
        }
        if found {
            // Still touching, update position
            x, y := ebiten.TouchPosition(*cursor.LastTouchID)
            cursor.Position.X = float64(x)
            cursor.Position.Y = float64(y)
        } else {
            // Touch ended
            cursor.JustInteracted = true
            cursor.LastTouchID = nil
        }
    }
}
```

**Consumers read only the singleton** (`system/select.go`):

```go
cursor := component.MustFindCursor(w)
if !cursor.JustInteracted {
    return
}
mouseRect := engine.Rect{X: cursor.Position.X, Y: cursor.Position.Y, Width: 1, Height: 1}
// ...hit-test colliders at transform.WorldPosition(entry)...
```

## Rules

- Exactly one `Cursor` entity, created up front in `createWorld()`;
  `MustFindCursor` panics when absent (see [singletons.md](singletons.md)).
- The Cursor system is **first** in the systems slice — pointer state must be resolved
  before anything consumes it.
- Consumers never poll raw pointer input. If a system needs the mouse or a touch, it
  needs the Cursor.
- `JustInteracted` is cleared at the top of every tick; it is only ever true for the one
  tick the interaction resolved.

## Pitfalls

- **Every consumer goes through the Cursor.** A button system that reads
  `inpututil.IsMouseButtonJustPressed` + `ebiten.CursorPosition` directly ignores touch
  entirely. Use `Cursor.JustInteracted` / `Cursor.Position` like `select.go` and
  `hover.go` do.
- **Don't assume world == screen.** Comparing `cursor.Position` straight against
  `transform.WorldPosition(entry)` ("camera doesn't move, so no need to transform") works
  until the camera moves or zooms; then every hit-test breaks — convert through the camera's
  screen→world helper instead (see [cameras.md](cameras.md); known pitfall #2 in
  [README.md](README.md)).
