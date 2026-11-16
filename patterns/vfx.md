# Procedural VFX

## How it works

There is no particle system and no shader. Every effect is
**entities plus tweens**: an [Animator animation](animations.md) mutating ordinary
components — scale, position, alpha, layer — or a short-lived entity assembled from
the same parts as everything else (sprite, velocity, time-to-live). That keeps VFX in
the same pipeline as gameplay: they Y-sort, they layer, they show up in the debug
overlay, and they cost nothing until triggered.

## Canonical code

**Squash-stretch on movement** — a parabolic scale bulge spanning one grid step. Note
*why* it's scale-only (`archetype/player.go`):

```go
// The bob is scale-only: the camera hard-locks to the player's position, so
// animating the transform position would shake the world, not the farmer.
...
animator.SetAnimation("step", &component.Animation{
    Timer: engine.NewTimer(stepDuration),
    Update: func(e *donburi.Entry, a *component.Animation) {
        if !component.Movable.Get(e).Moving {
            a.Stop(e)
            return
        }
        q := a.Timer.PercentDone()
        stretch := 4 * q * (1 - q) * stepStretch
        transform.GetTransform(e).LocalScale = math.Vec2{
            X: scale * (1 - stretch),
            Y: scale * (1 + stretch),
        }
    },
    OnStop: func(e *donburi.Entry) {
        transform.GetTransform(e).LocalScale = math.Vec2{X: scale, Y: scale}
    },
})
```

`4*q*(1-q)` is the house parabola: 0 at both ends, peak mid-tween. Every `OnStop`
restores the exact rest value — tweens never accumulate error.

**Rustle** — foliage shivers when the player brushes it: three decaying side-to-side
swings, X-scale only, anchored by the bottom-center pivot. The animation is built
lazily on first trigger, capturing the rest scale then
(`system/rustle.go`):

```go
p := a.Timer.PercentDone()
// Three side-to-side swings, dying out as p reaches 1. The
// bottom-center pivot keeps the sway anchored at the base.
sway := (1 - p) * stdmath.Sin(3*2*stdmath.Pi*p) * 0.12
transform.GetTransform(e).LocalScale = math.Vec2{X: base.X * (1 + sway), Y: base.Y}
```

**Carry & drop** — a chained choreography in `system/interact.go`: pick-up
scoops the item to the hands with `engine.EaseOutBack` (overshoot sells the grab),
the carry loop eases the hold point side-to-side as the farmer turns and adds a sine
bob on top, and the drop falls with `EaseIn`, then squashes on contact — swapping
render layer at the landing frame (see [layers-ysort.md](layers-ysort.md)):

```go
base = engine.LerpVec2(base, handOffset(facingLeft), 0.18)
...
bob := 3 * stdmath.Sin(2*stdmath.Pi*a.Timer.PercentDone())
transform.GetTransform(e).LocalPosition = math.Vec2{X: base.X, Y: base.Y + bob}
```

**Alpha pulse** — the selected-tile highlight breathes forever by resetting its own
timer; the sprite spawns with `AlphaOverride` already set (see
[sprite-effects.md](sprite-effects.md)) (`scene/game.go`):

```go
phase := stdmath.Sin(2 * stdmath.Pi * a.Timer.PercentDone())
component.Sprite.Get(e).AlphaOverride.A = 0.55 + 0.15*phase
```

**Explosion by slicing the corpse** — cut the dying sprite into four
sub-images at a random cut point, pre-rotate a copy to the world rotation, and spawn
each quadrant as a wreck entity with random velocity, spin, falling altitude, and its
own shadow (`archetype/wreck.go`):

```go
cutpointX := float64(int(width * engine.RandomFloatRange(0.3, 0.7)))
cutpointY := float64(int(height * engine.RandomFloatRange(0.3, 0.7)))

pieces := []engine.Rect{
    {X: 0, Y: 0, Width: cutpointX, Height: cutpointY},
    {X: cutpointX, Y: 0, Width: width - cutpointX, Height: cutpointY},
    {X: 0, Y: cutpointY, Width: cutpointX, Height: height - cutpointY},
    {X: cutpointX, Y: cutpointY, Width: width - cutpointX, Height: height - cutpointY},
}
...
for _, p := range pieces {
    img := baseImage.SubImage(p.ToImageRectangle()).(*ebiten.Image)
    ...
    velocity := transform.Right(parent)
    velocity.X *= engine.RandomFloatRange(0.5, 0.8)
    velocity.Y *= engine.RandomFloatRange(0.5, 0.8)

    component.Velocity.SetValue(wreck, component.VelocityData{
        Velocity:         velocity,
        RotationVelocity: engine.RandomFloatRange(-2, 2),
    })
    component.Altitude.SetValue(wreck, component.AltitudeData{
        Altitude: 1.0,
        Velocity: -0.01,
        Falling:  true,
    })
    NewShadow(w, wreck)
}
```

**Fake 3D from one float** — `AltitudeData.Altitude` (0 ground … 1 sky) drives scale,
shadow offset, and the landing layer swap in one system
(`system/altitude.go`):

```go
scale := 0.8 + 0.2*altitude.Altitude
t.LocalScale.X = scale
t.LocalScale.Y = scale

shadowTransform.LocalPosition.X = -archetype.MaxShadowPosition * altitude.Altitude
shadowTransform.LocalPosition.Y = archetype.MaxShadowPosition * altitude.Altitude
...
if altitude.Falling && altitude.Altitude == 0 {
    ...
    sprite.Layer = component.SpriteLayerDebris
}
```

**Fog-reveal ripple**. Revealing sight range doesn't pop a region — each
tile's fog fade gets a duration of `fogRevealDuration * time.Duration(distance)` where
`distance` is the Manhattan distance from the revealing tile
(`system/buildings.go`, `markTileInSight`), so the reveal ripples outward at
150ms per tile. The fade itself is a per-tile animation lowering `AlphaOverride.A` to
`1 - PercentDone()`, destroying the fog entity when done.

**Floating "+resource" icons**. A one-second toast is just three stock
components: `Velocity` `{Y: -1}` to drift up, `TimeToLive` to despawn, and an
animation fading `AlphaOverride.A` out (`archetype/tile.go`,
`NewTileFloatingAnimation`). No dedicated system — the movement, expiry, and render
systems already do everything.

## Rules

- Compose effects from what exists: tween scale/position/alpha/layer, or spawn
  entities with `Velocity` + `TimeToLive`. New VFX should need **zero** new systems.
- Scale tricks anchor at the sprite pivot — squash/sway reads right only with
  bottom-center pivots on world objects.
- `OnStop` restores exact rest values, always. Interruptible effects share an
  animation slot so a retrigger replaces the tween instead of fighting it
  (the `"glide"` slot).
- Effects that outlive their trigger get their own entity parented to the target, not
  bolted onto the target's sprite.

## Pitfalls

- **Don't cross-fade images per pixel on the CPU** (interpolating two tile images with
  `image.Set`, cached per from/to/percent) — a two-draw alpha blend or a trivial Kage
  shader does it for free. Treat any per-pixel `image.Set` loop in a frame path as a bug.
- Particles and shaders are staples not covered here (see [README.md](README.md)).
