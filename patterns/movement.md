# Movement

## How it works

Two canonical modes, both pure kinematics — **no acceleration, mass, or physics engine,
deliberately**. Speeds are px-per-tick at a fixed 60 TPS;
there is no delta time (that assumption is what makes frame-count timers and
`engine.TimeMultiplier` work — see [timers.md](timers.md)). Input systems write intent
(`Velocity`, or a glide target); movement systems are the only ones integrating it into
the transform.

**(a) Grid glide** — the entity's logical position is a board cell; movement is a
constant-speed glide between cell centers, snapping on arrival. The mover claims the
*target* cell on the occupancy grid before the glide starts, so its claimed cell is
always the one it is entering (see [grid-occupancy.md](grid-occupancy.md)).

**(b) Free velocity** — a `Velocity` component integrated into the transform every tick,
with facing driving the velocity vector for bullets/missiles.

## Canonical code

**Grid glide** (`system/movable.go`):

```go
func (s *Movable) Update(w donburi.World) {
    s.query.Each(w, func(entry *donburi.Entry) {
        movable := component.Movable.Get(entry)
        if !movable.Moving {
            return
        }

        if movable.Speed <= 0 {
            panic("movable speed must be greater than 0")
        }

        t := transform.GetTransform(entry)
        currentPos := t.LocalPosition

        dist := movable.TargetPosition.Distance(currentPos)
        if dist < movable.Speed {
            t.LocalPosition = movable.TargetPosition
            movable.Moving = false
        } else {
            dir := movable.TargetPosition.Sub(currentPos).Normalized()
            t.LocalPosition = currentPos.Add(dir.MulScalar(movable.Speed))
        }
    })
}
```

A step starts elsewhere (`system/player.go`): check `board.IsFree(to)`, claim
via `board.Move(from, to, entity)`, then set `TargetPosition` + `Moving = true`. The
`Speed <= 0` panic is deliberate — a zero-speed glide would hang forever, silently.

**Free velocity** (`system/velocity.go`, the whole system):

```go
func (v *Velocity) Update(w donburi.World) {
    v.query.Each(w, func(entry *donburi.Entry) {
        t := transform.Transform.Get(entry)
        velocity := component.Velocity.Get(entry)

        t.LocalPosition = t.LocalPosition.Add(velocity.Velocity)
        t.LocalRotation += velocity.RotationVelocity
    })
}
```

**Rotation-driven velocity** — set rotation first, then derive the vector from the
transform's basis instead of trigonometry by hand (`archetype/bullet.go`):

```go
t := transform.Transform.Get(bullet)
t.LocalPosition = position
t.LocalRotation = originalRotation + localRotation

component.Velocity.SetValue(bullet, component.VelocityData{
    Velocity: transform.Right(bullet).MulScalar(playerBulletSpeed),
})
```

Homing missiles keep steering by re-deriving `transform.Right` each tick as a follower
system turns `LocalRotation` (see [transform.md](transform.md)).

## Movement bounds as data

Bounds live on the bounded entity, not in the movement system.

- **Clamp to the camera rect, pivot-aware** (a shooter) — a `Bounds` component
  (with `Disabled` for cutscenes) and a system that computes min/max from camera
  position + screen size, switching on `sprite.Pivot` (top-left vs center offsets the
  clamp by size vs half-size), then `engine.Clamp`s both axes
  (`system/bounds.go`).
- **A side-scroller needs only a 1-D range**
  (`component/movementbounds.go`):

```go
type MovementBoundsData struct {
    Range engine.FloatRange
}
```

Its controls system checks the collider's world X against `Range.Min`/`Max` before
applying velocity.

## Rules

- Input writes `Velocity` or starts a glide; **only** the movement system touches the
  transform. One integrator per scene.
- Grid mode: never move without claiming the destination cell first; `IsFree` → `Move`
  → glide, and the glide target *is* the claimed cell
  ([grid-occupancy.md](grid-occupancy.md)).
- Fail loud on nonsense data (`Speed <= 0` panics).
- Keep speeds in px/tick constants; no delta time. Platformer physics (gravity, jumping,
  coyote time) exists nowhere — it's the biggest genre hole, design fresh from the
  staples list in [README.md](README.md).
- Wall response (slide along blocked axes) is part of the movement step — see
  [collisions.md](collisions.md) for the X-then-Y swept test. Keep last positions on a
  component, never in a package-global.

## Pitfalls

- Don't special-case the player layer inline in the `Velocity` system for wall sliding —
  drive the swept test by a component (e.g. a `BlockedBy` layer set), not by hardcoded
  layer checks inside the integrator.
- Don't cache `*component.GameData` in a bounds system's field — systems hold queries,
  not state ([ecs-core.md](ecs-core.md)); look the singleton up per tick or in `Init`.
- The glide compares `dist < movable.Speed` in **local** space — fine while parents sit
  at origin; use the same space for `TargetPosition` and the transform, and document
  which.
