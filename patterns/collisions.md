# Collisions

## How it works

Colliders are axis-aligned rects as component data, tagged with a **layer**; a hardcoded
layer-pair matrix decides which pairs are even tested. **Detection is one system**: it
brute-force intersects allowed pairs each tick and maintains stateful collision records
on both colliders, deriving *enter* (`JustCollidedWith`), *stay* (`CollidesWith`), and
*exit* (`JustOutOfCollisionWith`) sets. **Reactions are separate systems**, each with its
own query filtering on the components it needs, reading those sets. That separation is
the prescription: damage, auras, and XP pickups are three independent systems over one
detector.

## Canonical code

**Data model** (`component/collider.go`):

```go
type ColliderData struct {
    Rect     engine.Rect
    Layer    domain.ColliderLayer
    Disabled bool

    CollidesWith           map[CollisionKey]Collision
    JustCollidedWith       map[CollisionKey]struct{}
    JustOutOfCollisionWith map[CollisionKey]struct{}
}

type CollisionKey struct {
    Layer domain.ColliderLayer
    Other donburi.Entity
}
```

`Collision` records carry `{TimesSeen int; Detected bool}` — the detector's working
state for deriving enter/stay/exit.

**Layer-pair allow matrix** (`system/collision.go`) — pairs not listed are never
tested:

```go
var collisions = map[domain.ColliderLayer]map[domain.ColliderLayer]struct{}{
    domain.CollisionLayerPlayer: {
        domain.CollisionLayerPOI:   {},
        domain.CollisionLayerWalls: {},
    },
    domain.CollisionLayerHands: {
        domain.CollisionLayerInteractable: {},
    },
    // ...CollisionLayerCustomer: Entrance, Walls...
}
```

A variant maps layer pairs straight to **effect closures** — matrix and reaction in one
table (`system/collision.go`); fine for arcade games with instant reactions:

```go
var collisionEffects = map[component.ColliderLayer]map[component.ColliderLayer]collisionEffect{
    component.CollisionLayerPlayerBullets: {
        component.CollisionLayerAirEnemies: func(w donburi.World, entry, other *donburi.Entry) {
            component.Destroy(entry)
            component.Health.Get(other).Damage()
        },
    },
    component.CollisionLayerEnemyBullets: {
        component.CollisionLayerPlayers: func(w donburi.World, entry, other *donburi.Entry) {
            component.Destroy(entry)
            damagePlayer(w, other)
        },
    }, // ...
}
```

**The detector** (`system/collision.go`). Three passes per tick:

1. **Clear**: every collider's `Just*` sets are emptied; every existing `Collisions`
   record gets `Detected = false`.
2. **Detect**: for each allowed intersecting pair, mark `Detected = true` and write the
   record **symmetrically to both colliders** (`TimesSeen` 1 on first contact, capped
   at 2 after).
3. **Post-process** into the enter/exit sets:

```go
for _, entry := range entries {
    collider := component.Collider.Get(entry)
    for k, v := range collider.Collisions {
        if v.Detected {
            if v.TimesSeen == 1 {
                collider.JustCollidedWith[k] = struct{}{}
            }
        } else {
            collider.JustOutOfCollisionWith[k] = struct{}{}
            delete(collider.Collisions, k)
        }
    }
}
```

The collider's rect respects transform scale and an offset (`component/collider.go`):

```go
func (d *ColliderData) Rect(entry *donburi.Entry) engine.Rect {
    pos := transform.WorldPosition(entry)
    scale := transform.WorldScale(entry)
    return engine.Rect{
        X:      pos.X + d.Offset.X*scale.X,
        Y:      pos.Y + d.Offset.Y*scale.Y,
        Width:  d.Width * scale.X,
        Height: d.Height * scale.Y,
    }
}
```

**A reaction system** — own query, reads the detector's output, no detection of its own
(`system/collision_experience_pickups.go`, Update body):

```go
c.query.Each(w, func(entry *donburi.Entry) {          // query: ExperiencePickup + Collider
    collider := component.Collider.Get(entry)
    for collision := range collider.JustCollidedWith { // enter, exactly once
        other := w.Entry(collision.Other)
        if !other.HasComponent(component.Collector) {
            continue
        }
        // ...credit experience...
        component.Destroy(entry)
    }
})
```
Auras react to both edges: apply on `JustCollidedWith`, remove on
`JustOutOfCollisionWith` (`system/collision_aura.go`).

**Events on enter/exit**: the detector can also publish `domain.JustCollidedEvent` /
`domain.JustOutOfCollisionEvent` so non-collider systems can subscribe — see
[events.md](events.md).

**Movement response** (`system/velocity.go`): the velocity system tests the X and
Y components of the move **separately** against wall colliders before applying — a
blocked axis zeroes only that component, so the player slides along walls instead of
sticking. Keep any snap-back state (last positions) on a component, never in a
package-global map ([movement.md](movement.md)).

## Rules

- Detection is **one** system; every reaction is its own system filtering on its
  required components. Never react inside the detector.
- Reactions to one-shot effects read `JustCollidedWith` (enter), not `CollidesWith`
  (stay) — stay-reactions re-fire every tick.
- Write collision records to **both** colliders. Writing only the "active" side means
  the target never sees the collision.
- Every tested pair must be in the layer matrix; adding a collider layer means adding
  its matrix rows deliberately.
- Respect `Disabled` in the detector; intersect at `transform.WorldPosition` plus scale
  via `ColliderData.Rect`.
- Run detection **after** movement (`PlayerInput` → `Velocity` → `Collision`) so
  records describe post-move reality; destroy last ([entity-lifecycle.md](entity-lifecycle.md)).

## Pitfalls

- **Brute-force O(n²)** over collider pairs, and colliders ignore rotation. Pre-bucket
  entities by layer to skip never-colliding pairs; beyond that, broadphase (spatial
  hash) is a write-fresh staple, see [README.md](README.md).
- **Use world position, not `LocalPosition`**: intersecting at `LocalPosition` works
  only while colliders sit at the hierarchy root. The same goes for the wall test in
  `velocity.go`.
- A damage reaction that "sometimes triggers twice and sometimes not at all" has two
  usual causes: it reacts to the *stay* set instead of `JustCollidedWith`, or it
  `AddComponent`s mid-iteration, relocating component data under live pointers
  (donburi footgun, [ecs-core.md](ecs-core.md)).
