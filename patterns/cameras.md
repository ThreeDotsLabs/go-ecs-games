# Cameras

## How it works

A camera is an ordinary entity carrying `component.Camera`. It owns an offscreen
`Viewport` image and a `Root` entry — **only that subtree renders** into the viewport.
The render system iterates cameras ordered by `Index` (`donburi.NewOrderedQuery` +
`EachOrdered`), draws each subtree with [layers & Y-sort](layers-ysort.md) applied,
then composites the finished viewport onto the screen at the camera entity's own
transform position. `ViewportPosition` (world-space top-left) and `ViewportZoom`
define the world→viewport mapping; everything else is plain data on the component.

## Canonical code

**The component** (`component/camera.go`):

```go
type CameraData struct {
    Viewport         *ebiten.Image
    ViewportPosition math.Vec2
    ViewportZoom     float64
    ViewportTarget   *donburi.Entry
    ViewportBounds   ViewportBounds

    Root  *donburi.Entry
    Index int
    Mask  *ebiten.Image

    TransitionOverlay *ebiten.Image
    TransitionAlpha   float64

    ColorOverride *ColorOverride
    AlphaOverride *AlphaOverride
}

type ViewportBounds struct {
    X *engine.FloatRange
    Y *engine.FloatRange
}

func (d *CameraData) SetViewportPosition(pos math.Vec2) {
    if d.ViewportBounds.X != nil {
        pos.X = engine.Clamp(pos.X, d.ViewportBounds.X.Min, d.ViewportBounds.X.Max)
    }
    if d.ViewportBounds.Y != nil {
        pos.Y = engine.Clamp(pos.Y, d.ViewportBounds.Y.Min, d.ViewportBounds.Y.Max)
    }
    d.ViewportPosition = pos
}

func (d CameraData) Order() int { return d.Index }

func (d *CameraData) WorldPositionToViewportPosition(e *donburi.Entry) math.Vec2 {
    pos := transform.WorldPosition(e)
    pos = pos.Sub(d.ViewportPosition)
    pos = pos.MulScalar(d.ViewportZoom)
    return pos
}
```

Nil bounds mean "unclamped on that axis"; move the camera through
`SetViewportPosition`, never by assigning `ViewportPosition` directly. `Mask` shapes
the viewport, `TransitionOverlay`/`TransitionAlpha` drive fades, and the color/alpha
overrides tint the composited viewport.

**The world/UI split** — two cameras, two roots, ordered by `Index`; the world camera
gets `ViewportTarget = player`, the UI camera never moves (`scene/game.go`):

```go
ui := archetype.NewNamed(world, "UI").
    WithLayer(domain.SpriteUILayerUI).
    Entry()
...
bg := archetype.New(world).WithLayer(domain.SpriteLayerBackground).Entry()

levelCam := archetype.NewCamera(world, math.Vec2{X: 0, Y: 0}, engine.Size{Width: g.screenWidth, Height: g.screenHeight}, 0, bg)
...
archetype.NewCamera(world, math.Vec2{X: 0, Y: 0}, engine.Size{Width: g.screenWidth, Height: g.screenHeight}, 1, ui)
```

**Follow** — a tiny system centers each camera on its `ViewportTarget` through the
clamping setter; a side-scroller follows X only
(`system/camerafollow.go`):

```go
pos := transform.WorldPosition(cam.ViewportTarget)
viewportWorldWidth := float64(cam.Viewport.Bounds().Dx()) / cam.ViewportZoom
targetCameraX := pos.X - viewportWorldWidth/2.0

viewportPos := cam.ViewportPosition
viewportPos.X = targetCameraX
cam.SetViewportPosition(viewportPos)
```

The top-down variant hard-locks on **both**
axes, plus `Init(w)` running one `Update` before the first frame — ebiten draws once
before the first tick, so without it the target starts off-center.

**Screen→world** is the inverse of `WorldPositionToViewportPosition`. Make it a
`CameraData` method — every moving camera needs it, and inline copies drift:

```go
func (d *CameraData) ViewportPositionToWorldPosition(pos math.Vec2) math.Vec2 {
    return pos.DivScalar(d.ViewportZoom).Add(d.ViewportPosition)
}
```

**Nested cameras** — the renderer recurses when it meets a camera entity inside a
subtree, drawing it into the parent's viewport; the `InnerCamera` tag keeps it out of
the top-level pass (`system/render.go`):

```go
r.camerasQuery.EachOrdered(w, component.Camera, func(entry *donburi.Entry) {
    if !entry.HasComponent(component.InnerCamera) {
        count += r.renderCamera(entry, r.offscreen, game)
    }
})
...
if child.entry.HasComponent(component.Camera) {
    count += r.renderCamera(child.entry, camera.Viewport, game)
}
```

This is how a dialog log scrolls inside the dialog panel
(`archetype/dialog.go`): a child camera with `ViewportBounds.Y` as scroll limits and a
gradient `Mask` (composited with `ebiten.BlendDestinationIn`) fading the log's edges.

**Temporary zoom (BriefZoom)** — snapshot the *whole* `CameraData` before a POI
close-up, then tween position and zoom back to the snapshot on exit, restoring the
follow target and bounds it also disabled
(`archetype/dialog.go`, `zoomInOnPOI`):

```go
camCopy := *cam
bz.OriginCamera = &camCopy

cam.ViewportBounds = component.ViewportBounds{}
cam.ViewportTarget = nil
```

## Rules

- Cameras are entities; `Index` decides compositing order (world 0, UI 1) — never
  creation order.
- A camera's `Root` must carry a `Layer` — the renderer panics otherwise.
- UI lives under its own camera root, not on high layers of the world camera, so it
  never inherits the world camera's position or zoom.
- All world↔screen math goes through the `CameraData` methods. If you find yourself
  writing `pos - cameraPos` anywhere else, stop and use the method.
- Hide a camera by giving it `component.Active` and toggling it — the renderer skips
  inactive cameras.

## Pitfalls

- **Camera math duplicated ad hoc** between render and controls (screen→world written
  inline, `// TODO deduplicate with render`) is the failure mode
  `ViewportPositionToWorldPosition` exists to prevent.
- **Hit-tests that assume world == screen** ("camera doesn't move, so no need to
  transform from world space") break the day the camera starts following the player.
  Always convert the cursor through the camera (see [input-cursor.md](input-cursor.md)).
- No smoothing, deadzone, or lookahead exists anywhere — every follow is a hard lock.
  Design fresh when a game needs game-feel camera work (see the staples list in
  [README.md](README.md)).
- Zoom through `ViewportZoom`, not by scaling the world's root transform — scaled
  transforms leak into every world-position calculation.
