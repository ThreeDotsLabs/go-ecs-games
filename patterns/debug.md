# Debug tooling

## How it works

One flag rules everything: `Debug{Enabled}` on the `component.Game` singleton, toggled by
`/` in a dedicated `system.Debug`. Every debug feature — gizmos, stats, hierarchy dumps,
cheats — checks that flag and nothing else. Debug *rendering* is a second pass at the end
of `system/render.go`, over the entities the main pass just drew, so gizmos always sit on
top. Cheats mutate the world through the same events and functions as real play.

## Canonical code

**The toggle** — its own tiny system (`system/debug.go`):

```go
func (d *Debug) Update(w donburi.World) {
    game := component.MustFindGame(w)
    if inpututil.IsKeyJustPressed(ebiten.KeySlash) {
        game.Debug.Enabled = !game.Debug.Enabled
    }
}
```

**The second pass** — after all layers of a camera are drawn, walk the same list again
(`system/render.go`, in `renderCamera`):

```go
// Render debug information on top of everything
if game.Debug.Enabled {
    for _, child := range allRenderedEntities {
        if child.entry.HasComponent(component.Bounds) {
            renderBoundsDebug(child.entry, camera)    // magenta rect
        }
        if child.entry.HasComponent(component.Collider) {
            renderColliderDebug(child.entry, camera)  // lime rect
        }
        if child.entry.HasComponent(transform.Transform) {
            renderPositionDebug(child.entry, camera)  // yellow crosshair
            // …plus renderNameDebug (yellow label) for entities with component.Name
        }
    }
}
```

A gizmo draw is a plain `vector` stroke in viewport space (`system/render.go`):

```go
func renderColliderDebug(entry *donburi.Entry, camera *component.CameraData) {
    collider := component.Collider.Get(entry)
    pos := camera.WorldPositionToViewportPosition(entry)
    zoom := camera.ViewportZoom
    rect := collider.Rect.Move(pos)
    w := rect.Width * zoom
    h := rect.Height * zoom
    vector.StrokeRect(camera.Viewport, float32(rect.X), float32(rect.Y), float32(w), float32(h), 1, colornames.Lime, false)
}
```

Camera viewports get a red `StrokeRect` (`renderCameraDebug`); the screen corner gets
stats via `ebitenutil.DebugPrintAt`: FPS, TPS, `w.Len()` world entities, rendered count,
plus a rendered-**chunk** count and camera pos/scale in a chunked world (see
[chunks](chunks.md)). The render walk also *collects* collider/bounds-only entities while
debug is on, so sprite-less entities still get gizmos.

**Hierarchy tooling** — `P` (inside `if game.Debug.Enabled`) calls
`PrintHierarchy(w)`: find parentless `transform.Transform` roots, recursively print the
tree, indented, with entity IDs and `component.Name` strings (`system/debug.go`).
Its sibling `system/validator.go` is a **canary**: once per second it runs
`DetectMultipleParents(w)` and logs offenders — the check that caught a real donburi
hierarchy bug. Cheap always-on validators earn their slot in `g.systems`.

**Fast-forward** — timers count frames scaled by a global multiplier
(`engine/timer.go`, see [timers](timers.md)):

```go
// TimeMultiplier can be used to speed up or slow down the timer in development or testing.
var TimeMultiplier = 1
```

`Timer.Update` adds `TimeMultiplier` per tick; bind it to a 1–100x slider in a
debug ebitenui window (`scene/game_debug.go`), next to an items window handing the player
any item via the normal `archetype.NewItemByID` path. This is the payoff of the
frame-count timer rule: one global int fast-forwards the whole game.

**Cheats behind the flag** — grant resources on keys 1–6, through the same
`UpdateResources` → `events.ResourcesDelta` as gameplay (`system/debug.go`):

```go
if inpututil.IsKeyJustPressed(ebiten.Key1) {
    UpdateResources(w, events.ResourcesDelta{
        Food: 10,
    })
}
```

For touch, mirror these as a pause-menu cheat panel (resource buttons plus "Toggle
Fog", revealed by a "Toggle Debug" button — `scene/board.go`), or fire the toggle on a
long-short-short tap sequence. A speed cheat publishes `PlayerSpeedChangedEvent{±10}` —
the same event real modifiers use.

**Input markers as entities** — spawn an ephemeral X at every click/tap: lime
for touch, magenta for mouse. Visualize input *with* the ECS — a TTL entity, not an
immediate-mode overlay (`system/controls.go`, see [input-touch](input-touch.md)):

```go
debugImage := ebiten.NewImage(32, 32)
vector.StrokeLine(debugImage, 0, 0, 32, 32, 2, clr, true)
vector.StrokeLine(debugImage, 0, 32, 32, 0, 2, clr, true)

debug := archetype.New(w).
    WithPosition(math.Vec2{X: float64(cursorX - imageSize/2), Y: float64(cursorY - imageSize/2)}).
    WithLayer(component.SpriteUILayerTop).
    WithSprite(component.SpriteData{Image: debugImage}).
    With(component.TimeToLive).
    With(component.UI).
    Entry()
```

…then `component.TimeToLive.SetValue` gives it a 2-second timer and the destroy system
sweeps it up.

**Per-entity gizmos** — for every sprite, draw entity ID and world
pos/rotation labels, blue `Right()` / green `Up()` axis lines, the collider AABB, and AI
paths as red dots joined by a polyline (`system/debug.go`):

```go
length := 50.0
right := position.Add(transform.Right(entry).MulScalar(length))
up := position.Add(transform.Up(entry).MulScalar(length))

vector.StrokeLine(d.offscreen, float32(position.X), float32(position.Y), float32(right.X), float32(right.Y), 1, colornames.Blue, false)
vector.StrokeLine(d.offscreen, float32(position.X), float32(position.Y), float32(up.X), float32(up.Y), 1, colornames.Lime, false)

if entry.HasComponent(component.AI) {
    ai := component.AI.Get(entry)
    for i, p := range ai.Path {
        vector.FillRect(d.offscreen, float32(p.X-2), float32(p.Y-2), 4, 4, colornames.Red, false)
        // …DrawLine to Path[i+1], plus a closing segment to Path[0] when ai.PathLoops
    }
}
```

**Profiling** — a `run.cpuprofile=out.pprof` setting wraps the run in
`pprof.StartCPUProfile` / `StopCPUProfile` (`main.go`; see
[dev-shortcuts](dev-shortcuts.md) and [profiling](profiling.md)).

## Rules

- **Everything gates on the singleton flag.** One `/` press switches all of it; no
  per-feature flags, no build tags.
- **Gizmos for world entities live in the render/debug systems**, as a second pass. A
  system may print its *own* state in its own debug-gated `Draw` (a touch readout in
  `controls.go`) — never another system's data.
- **Cheats mutate through the same events/commands as real play** (`UpdateResources`,
  `PlayerSpeedChangedEvent`, archetype constructors — see [events](events.md)). A cheat
  with a private shortcut exercises a code path the game doesn't have.
- Color code: bounds magenta, colliders lime, positions/names yellow, cameras and AI
  paths red.

## Pitfalls

- **Never branch gameplay logic on the debug flag.** Infinite mana wired straight into
  `system/casting.go` (`if c.debug.Enabled { manaCost = 0 }`) is invisible, unlisted, and
  changes the very system being tested. Grant a huge mana pool from the debug system
  instead; keep cheats out of gameplay branches.
- Gizmos draw with `vector.StrokeLine` / `StrokeRect` / `FillRect`, not the deprecated
  `ebitenutil` shapes (known pitfall #7).
- Keep the flag in `component.Game`, not a standalone `component.Debug` singleton —
  one fewer singleton to create in every scene.

## Variations

- Register the `Debug` system in *both* slices — `Update` for input, `Draw` after
  `Render` so its ebitenui windows land on top.
- Extra dev keys behind the flag (`P` freeze camera, `R` restart level, keys that mutate
  the player) go through events like any cheat. Always-on dev keys belong in
  [dev-shortcuts](dev-shortcuts.md).
