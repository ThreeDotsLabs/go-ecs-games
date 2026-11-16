# ECS core

## How it works

Everything is donburi. A **component** is pure data registered as a typed component.
A **system** is a struct with cached queries and an `Update(w donburi.World)` method.
A **scene** owns one `donburi.World` plus two hand-ordered slices — `[]System` (ticked)
and `[]Drawable` (rendered) — and slice order is the only ordering mechanism. There is no
system registry, no priorities, no manager objects, no `init()` magic: adding behavior is
"write a struct, append it to the slice".

## Canonical code

**Component** — one per file, `XData` struct + component type var, no behavior
(`component/movable.go`):

```go
type MovableData struct {
    Moving         bool
    Speed          float64
    TargetPosition math.Vec2
}

var Movable = donburi.NewComponentType[MovableData]()
```

Pure helper methods on the data are fine (`BoardData.IsFree`, `SpriteData.Hide`);
queries and world mutation are not.

**System** — struct + constructor that builds queries once + `Update`
(`system/destroy.go`):

```go
type Destroy struct {
    query *donburi.Query
}

func NewDestroy() *Destroy {
    return &Destroy{
        query: donburi.NewQuery(
            filter.Contains(component.Destroyed),
        ),
    }
}

func (d *Destroy) Update(w donburi.World) {
    d.query.Each(w, func(entry *donburi.Entry) {
        transform.RemoveRecursive(entry)
    })
}
```

Component access inside the loop: `movable := component.Movable.Get(entry)` — mutate
through the pointer. Filters compose: `filter.And`, `filter.Or`,
`filter.Not(filter.Contains(component.Destroyed))` to skip dying entities.

**Interfaces** — declared once in `scene/scene.go`; systems satisfy them structurally
(`scene/scene.go`):

```go
type System interface {
    Update(w donburi.World)
}

type Drawable interface {
    Draw(w donburi.World, screen *ebiten.Image)
}

type Initializer interface {
    Init(w donburi.World)
}
```

A system that draws implements both and is registered in **both** slices. `Init(w)` is
for one-time setup: event subscriptions, caching singleton pointers, or running `Update`
once before the first frame (ebiten draws once before the first tick —
`system/camera_follow.go` uses this).

**Registration & ordering** — the ordered slice in the scene constructor, with every
non-obvious dependency commented (`scene/game.go`):

```go
g.systems = []System{
    system.NewCursor(),
    system.NewDebug(),
    system.NewHover(),
    system.NewSelect(),
    system.NewButton(),
    system.NewPlayer(),
    system.NewInteract(), // after NewPlayer, so it sees this tick's facing
    system.NewMovable(),
    system.NewRustle(),    // reads the post-move position, so a step triggers its rustle immediately
    system.NewAnimation(), // after the systems that start animations, so tweens advance the same tick
    system.NewSelectedTile(),
    system.NewCameraFollow(), // after NewMovable, so the camera reads the post-move position
    system.NewTimeToLive(),
    system.NewDestroy(),
}

g.drawables = []Drawable{
    system.NewRender(),
}
```

**Singleton access** — `engine.MustFindComponent[T](w, c)` / `component.MustFindGame(w)`,
panicking when absent. Only for true singletons (`Game`, `Cursor`, `Board`); everything
else goes through queries.

## Rules

- One component per file; one system per file; names `XData` / `X` / `NewX`.
- Systems hold queries, not state — game state lives on components. State kept on a
  system survives nothing and hides from the debug tools.
- Build queries in the constructor, never inside `Update` — an inline
  `donburi.NewQuery(...)` in a hot path allocates every tick.
- **Registering the system is part of writing it** — an unregistered system silently
  never runs, with no error.
- `system.NewDestroy()` is always last, so everything else sees consistently-alive
  entities within a tick (see [entity-lifecycle.md](entity-lifecycle.md)).
- Order-sensitive registration gets an inline comment, always.
- New behavior = new system. Resist stuffing logic into an existing system because it's
  adjacent (a 683-line `controls.go` god-system is how that ends).

## Pitfalls

- **Re-`Get` component data after anything changes an entity's component set** — data
  can relocate; captured pointers go stale (documented in `system/interact.go`).
- Don't use donburi's default-value component form with pointer fields — the pointer is
  shared across every entity (a `Health` that embeds a `*engine.Timer` this way gives
  every health entity the same timer).
- Don't park queries as package vars in `component/` (`component.WorkingQuery`) —
  queries belong to the system that uses them.
- Older donburi APIs (`query.NewQuery`, `transform.GetTransform`, v1.3.9) are removed;
  code written against them won't compile.

## Variations

- A `Stopper{Stop(w)}` optional interface for scene teardown (audio players closing) —
  adopt when a system holds OS resources.
