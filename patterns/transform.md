# Transform & hierarchy

## How it works

Position, rotation, scale, and the parent/child tree all come from donburi's
`features/transform`, used **wholesale** — no game defines its own transform. Every
entity gets one at creation (`archetype.New`, see [archetypes](archetypes.md)), so the
world is one tree: cameras render from a root, children move with parents, and
"A owns B" is expressed as "B is a child of A".

The API surface actually used:

- `transform.AppendChild(parent, child, keepWorldPosition)` — attach (the builder's `WithParent`).
- `transform.ChangeParent(child, newParent, keepWorldPosition)` — reparent.
- `transform.GetParent(e)` / `transform.GetChildren(e)` — walk the tree (ok-returning).
- `transform.WorldPosition(e)` / `WorldScale(e)` / `WorldRotation(e)` — resolved through all ancestors.
- `transform.GetTransform(e).LocalPosition / LocalScale / LocalRotation` — the local values you mutate.
- `transform.Right(e)` / `transform.LookAt(e, target)` — rotation as a direction.
- `transform.FindChildWithComponent(e, c)` — find a child by [tag](tags.md).
- `transform.RemoveRecursive(e)` — delete a subtree (only from the destroy system — see
  [entity-lifecycle](entity-lifecycle.md)).

Two propagation rules to internalize: **scale multiplies down the tree; position does
not get scaled** — a child's `LocalPosition` is a plain offset in world pixels, whatever
the parent's scale. Rotation is in **degrees**, `0` pointing +X (right); with Y-down
screen coordinates, `-90` points up.

## Canonical code

**Reparenting as lifecycle** — picking an item up means moving it in the tree, and the
comments carry the propagation rules (`system/interact.go`):

```go
// pickUp reparents the item into the player's hands. Local scale resets to 1
// (donburi multiplies scales down the hierarchy, so the player's scale now
// applies), and the item moves to LayerHeldItem ...
func pickUp(player, item *donburi.Entry) {
    // Child positions aren't scaled by the parent, so the world-space delta is
    // the item's local position under the player as-is.
    from := transform.WorldPosition(item).Sub(transform.WorldPosition(player))

    transform.ChangeParent(item, player, false)

    t := transform.GetTransform(item)
    t.LocalPosition = from
    t.LocalScale = math.Vec2{X: 1, Y: 1}
    ...
}
```

Putting down is the mirror image — back under the world root, restoring the ground
scale (`system/interact.go`, trimmed):

```go
root := engine.MustGetParent(player)
transform.ChangeParent(item, root, false)

scale := board.CellSize / assets.TileSize
t := transform.GetTransform(item)
t.LocalPosition = from
t.LocalScale = math.Vec2{X: scale, Y: scale}
```

**Rotation-driven movement** — set the rotation, then derive velocity from it with
`transform.Right`; the art's authored facing goes in `SpriteData.OriginalRotation`
(`archetype/bullet.go`, trimmed):

```go
originalRotation := -90.0 // the laser sprite is drawn pointing up

t := transform.Transform.Get(bullet)
t.LocalPosition = position
t.LocalRotation = originalRotation + localRotation

component.Velocity.SetValue(bullet, component.VelocityData{
    Velocity: transform.Right(bullet).MulScalar(playerBulletSpeed),
})

component.Sprite.SetValue(bullet, component.SpriteData{
    Image:            image,
    Pivot:            component.SpritePivotCenter,
    OriginalRotation: originalRotation,
})
```

The renderer draws at `WorldRotation(entry) - sprite.OriginalRotation`, so logic always
works in world angles and never compensates for how the art was drawn. Enemies that
"fly straight ahead" are one line (`system/ai.go`):

```go
velocity.Velocity = transform.Right(entry).MulScalar(ai.Speed)
```

**Turrets** aim with `LookAt` (`system/observer.go`, trimmed):

```go
observer.Target = component.ClosestTarget(w, entry, observer.LookFor)
targetPos := transform.WorldPosition(observer.Target)
transform.LookAt(entry, targetPos)
```

**Find helpers** — `engine/find.go` wraps the ok-returning tree lookups in panicking
variants for relationships that must exist (`engine/find.go`):

```go
func MustGetParent(entry *donburi.Entry) *donburi.Entry
func MustFindChildWithComponent(parent *donburi.Entry, c component.IComponentType) *donburi.Entry
func MustFindGrandchildWithComponent(parent *donburi.Entry, c component.IComponentType) *donburi.Entry  // recursive
func FindChildrenWithComponent(parent *donburi.Entry, c component.IComponentType) []*donburi.Entry      // direct children only
```

## Rules

- **Never write a custom transform.** Any position/rotation/scale/hierarchy need is
  donburi's `features/transform`; a second source of spatial truth is a bug.
- Degrees everywhere; `transform.Right(e)` is the facing vector; art facing goes in
  `OriginalRotation`, compensated in the renderer only.
- On reparent, re-derive `LocalPosition` from world positions and **reset `LocalScale`**
  for the new parent's scale — remember: scale inherits, position offset doesn't scale.
- Ownership changes are `ChangeParent`, not position-copying; find owned entities with
  `FindChildWithComponent` + a [tag](tags.md).
- Deleting is `RemoveRecursive`, and only the destroy system calls it
  ([entity-lifecycle](entity-lifecycle.md)) — a dangling child left by a plain
  `entry.Remove()` still haunts its parent's children list.
- Use the `Must*` find helpers when absence is an authoring error; the ok-returning forms
  when absence is a legal state (e.g. "is the player holding anything?").
