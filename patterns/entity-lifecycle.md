# Entity lifecycle

## How it works

Entities die by **mark-and-sweep**: anything that wants an entity gone calls
`component.Destroy(e)`, which only adds a `Destroyed` marker. One sweep system —
registered **last** — removes every marked entity (and its subtree) at the end of the
tick. Nothing ever calls `entry.Remove()` or `transform.RemoveRecursive` directly, so
within a tick every system sees a consistent world, marked entities can still be read,
and nothing is removed mid-iteration.

Soft-disable is separate: `component.Active{Active: bool}` hides an entity without
killing it. The renderer's tree walk prunes an inactive entity's **whole subtree**
(`system/render.go` `getAllChildren`/`isActive`), and systems
respect it too (`system/button.go` skips inactive buttons; `hover`/`select` toggle it on
overlay entities).

## Canonical code

**The marker** — idempotent and safe on dead entries (`component/destroyed.go`):

```go
type DestroyedData struct{}

var Destroyed = donburi.NewComponentType[DestroyedData]()

func Destroy(e *donburi.Entry) {
    if !e.Valid() {
        return
    }
    if !e.HasComponent(Destroyed) {
        e.AddComponent(Destroyed)
    }
}
```

**The sweep** (`system/destroy.go`):

```go
func (d *Destroy) Update(w donburi.World) {
    d.query.Each(w, func(entry *donburi.Entry) {
        transform.RemoveRecursive(entry)
    })
}
```

Registered last, always (`scene/game.go`):

```go
g.systems = []System{
    system.NewCursor(),
    // ...
    system.NewTimeToLive(),
    system.NewDestroy(), // always last
}
```

**Timed death** — `TimeToLiveData{Timer *engine.Timer}` plus a system that marks, never
removes (`system/timetolive.go`; see [timers](timers.md)):

```go
func (t *TimeToLive) Update(w donburi.World) {
    t.query.Each(w, func(entry *donburi.Entry) {
        ttl := component.TimeToLive.Get(entry)
        ttl.Timer.Update()
        if ttl.Timer.IsReady() {
            component.Destroy(entry)
        }
    })
}
```

**Skipping the dying** — a query that must not see this tick's casualties filters them out
(`archetype/build_road_panel.go`):

```go
q := donburi.NewQuery(
    filter.And(
        filter.Contains(component.RoadPlaceholder),
        filter.Not(filter.Contains(component.Destroyed)),
    ),
)
```

**Off-screen despawn** — a scrolling shooter adds two more drivers next to
`TimeToLive`. `Despawnable` destroys a unit once it has fully left the camera rect — but
only after it has fully *entered* it once; the `Spawned` flag also gates collisions, so
units gliding in from off-screen aren't shootable yet
(`component/despawnable.go`, `system/despawn.go`):

```go
type DespawnableData struct {
    // Set when the unit first appears on-screen
    Spawned bool
}

// system/despawn.go, per entity:
if !despawnable.Spawned {
    if /* entity rect fully inside camera rect */ { despawnable.Spawned = true }
    return
}
if /* entity rect fully outside camera rect */ { component.Destroy(entry) }
```

**Distance-limited death** — bullets die after a total distance traveled, not a duration
(`system/maxdistance.go`):

```go
dl.DistanceTraveled += pos.Distance(dl.PreviousPosition)
if dl.DistanceTraveled > dl.MaxDistance {
    component.Destroy(entry)
} else {
    dl.PreviousPosition = pos
}
```

**Destroyed as a death event** — because marked entities stay readable until the sweep,
a system registered between the marker and `NewDestroy` can react to a death *in the
death frame*: query for `Destroyed` **plus** a reaction component. Dropping experience
orbs on death (`system/drop_experience.go`):

```go
query: donburi.NewQuery(
    filter.Contains(component.Destroyed, component.DropExperience),
),

// Update:
s.query.Each(w, func(e *donburi.Entry) {
    pos := transform.WorldPosition(e)
    exp := component.DropExperience.Get(e).Experience
    for i := 0; i < exp; i++ {
        archetype.NewExperiencePickup(w, /* pos + jitter */)
    }
})
```

Order this deliberately: collision marks `Destroyed`, then the
reaction systems run, then `NewDestroy()` sweeps.

**Whole-world teardown** — to reset, don't destroy entities one by one; drop the world
and rebuild it (`scene/game.go`):

```go
// Press R to rebuild the world from scratch — handy while iterating.
if inpututil.IsKeyJustPressed(ebiten.KeyR) {
    g.createWorld()
    g.init()
}
```

Scene switches work the same way — each scene owns its world; a switch builds a fresh
one (see [scenes](scenes.md)).

## Rules

- **Only `component.Destroy(e)` kills entities.** Never `entry.Remove()`, never
  `transform.RemoveRecursive` outside the destroy system, never during `query.Each`.
- `system.NewDestroy()` is registered **last**. Every despawn driver (TimeToLive,
  Despawnable, DistanceLimit, gameplay code) only marks.
- Destroying a parent destroys the subtree (`RemoveRecursive`) — held items and child
  sprites go with their owner. Reparent first if something must survive
  ([transform](transform.md)).
- A query that spawns from or renders live entities filters
  `filter.Not(filter.Contains(component.Destroyed))`.
- Death reactions are systems that query `Destroyed` + a payload component, registered
  after the markers and before the sweep — no event bus needed for "on death" logic.
- Use `Active` for temporary hiding (UI overlays, toggled highlights); it prunes the
  subtree in the renderer but the entity keeps ticking unless its systems also check it.
- Reset = rebuild the world, never a hand-written "destroy everything" pass.
