# Enemy AI

## How it works

Two shapes cover real-time NPCs:

1. **The FSM NPC** (shop customers): a state enum and per-phase timers **on the
   component**; one system whose `Update` is a switch dispatching a `handleX` method per
   state ([state-machines](state-machines.md) covers the pattern in general). Movement
   is "set velocity toward `TargetPosition`, arrive when within a threshold".
2. **The steering NPC** (shooter enemies): tiny single-purpose components — a movement policy
   (`AI`), a sensor (`Observer`), actuators (`Shooter`, `Follower`) — each with its own
   system. Behavior is composed by attaching components, never by subclassing.

Spawners are components holding a timer plus spawn parameters, ticked by a system.
There is **no pathfinding** — levels are authored so straight lines work.

## Canonical code: the FSM

State and timers live on the component (`component/customer.go`):

```go
const (
    CustomerStateWandering CustomerState = iota
    CustomerStateEntering
    CustomerStateInQueue
    CustomerStateThinking
    CustomerStateWaitingForService  // Customer finished thinking, waiting for player to interact
    CustomerStateWaitingForProducts // Player has interacted, customer is waiting for products
    CustomerStateLeaving
    CustomerStateWalkingAway // Walking horizontally after leaving the bakery
)

type CustomerData struct {
    State          CustomerState
    TargetPosition math.Vec2
    MovementSpeed  float64
    ThinkingTimer  *engine.Timer
    ServiceTimer   *engine.Timer
    ProductsTimer  *engine.Timer
    // ... QueuePosition, WillEnterBakery, Order, emote/progress-bar entity refs
}
```

The system only dispatches (`system/customer.go`):

```go
switch customer.State {
case component.CustomerStateWandering:
    c.handleWandering(w, customerEntry, customer, velocity, customerTransform)
case component.CustomerStateInQueue:
    c.handleInQueue(w, customerEntry, customer, velocity, customerTransform)
case component.CustomerStateThinking:
    c.handleThinking(w, customerEntry, customer, velocity)
// ... one case per state ...
case component.CustomerStateWalkingAway:
    c.handleWalkingAway(w, customerEntry, customer, velocity, customerTransform)
}
```

One handler shows the movement idiom — steer at `TargetPosition`, transition on arrival (`system/customer.go` `handleLeaving`, trimmed):

```go
worldPos := transform.WorldPosition(customerEntry)
direction := customer.TargetPosition.Sub(worldPos)
distance := direction.Distance(dmath.Vec2{})

if distance < MovementThreshold {
    customer.State = component.CustomerStateWalkingAway
    // ... pick a random walk-away direction, set velocity ...
} else {
    velocity.Velocity = dmath.Vec2{
        X: (direction.X / distance) * customer.MovementSpeed,
        Y: (direction.Y / distance) * customer.MovementSpeed,
    }
}
```

**Queue slots are entities**: `QueueSpotData{ID, Occupied, Customer}` placed in the
level; the system caches them by ID (`queueSpotCache`, rebuilt when an entry goes
invalid) and customers claim/release spots as they advance — world structure lives in
entities, not in a system-owned list.

**The spawner is event-driven** (`system/customer.go`): the day-clock
[event](events.md) starts and stops it — at 5:45
`spawner.SpawnTimer = engine.NewTimer(5 * time.Second)`, at 16:45 `nil`. Each ready tick
rolls `rand.Float64() < spawner.EnterChance`; `CurrentCustomers >= MaxCustomers` gates spawning.

## Canonical code: steering

`AIData` holds the policy: `Type` (`AITypeConstantVelocity` | `AITypeFollowPath`),
`Speed`, `Path []math.Vec2`, `PathLoops`, `NextTarget`
(`component/ai.go`). Constant velocity is a one-shot —
`velocity.Velocity = transform.Right(entry).MulScalar(ai.Speed)` once, guarded by a
`StartedMoving` flag. Follow-path steers with a clamped turn rate
(`system/ai.go`, trimmed of TODOs):

```go
target := ai.Path[ai.NextTarget]
x := target.X - position.X
y := target.Y - position.Y
dist := math.Sqrt(x*x + y*y)
if dist < 1 {
    ai.NextTarget++
    if ai.PathLoops && ai.NextTarget >= len(ai.Path) {
        ai.NextTarget = 0
    }
    return
}

worldRotation := transform.WorldRotation(entry)
angle := math.Round(dmath.ToDegrees(math.Atan2(y, x)))
if worldRotation-angle > 180.0 {
    angle = float64(int(angle+360.0) % 360)
} else if worldRotation-angle < -180.0 {
    angle = float64(int(angle-360.0) % 360)
}

maxRotation := 2.0 * ai.Speed
diff := angle - worldRotation
// ... clamp diff to ±maxRotation ...
transform.Transform.Get(entry).LocalRotation += diff

radians := dmath.ToRadians(angle)
velocity.Velocity.X = math.Cos(radians) * ai.Speed
velocity.Velocity.Y = math.Sin(radians) * ai.Speed
```

The ±180° wrap keeps the turn on the short side; the clamp (`2.0 * ai.Speed` degrees a
tick) makes fast planes turn wider. **Paths are authored in Tiled** as polylines,
loaded into `assets.Path{Points, Loops}` (see [tiled](tiled.md)).

**Sense and act are separate components.** The sensor is
`ObserverData{LookFor *donburi.Query, Target *donburi.Entry}`; its system fills the
target every tick (`system/observer.go`):

```go
observer.Target = component.ClosestTarget(w, entry, observer.LookFor)
if observer.Target == nil {
    return
}
transform.LookAt(entry, transform.WorldPosition(observer.Target))
```

Actuators read what the sensor wrote: `system/shooter.go` fires on a `ShootTimer` only
if the entity's `Observer` has a `Target` (turrets hold fire with nobody in sight);
`system/follower.go` homes a missile via `LookAt` + `Right()·speed` until its
`FollowingTimer` expires. Register `Observer` **before** the actuators and comment the
ordering — `shooter.go`'s own TODO admits the implicit dependency.

## Seeker → Targeter → AI, and wave spawners

The **Seeker → Targeter → AI** pipeline splits steering further: `Seeker`
(*how to pick*, e.g. `SeekerTypeNearestPlayer`) writes `Targeter.Target`; `AI`
(`AITypeMoveTowardsTarget`) reads it and sets velocity from `Mover.Speed` — so aggro
rules and movement swap independently. A
**`Spawner{Count engine.IntRange, Interval engine.DurationRange, SpawnFunc func(w) *donburi.Entry}`**
re-rolls the interval each cycle (`Timer.SetTarget(Interval.Random())`) and spawns
`Count.Random()` entities through the closure — the shape for wave spawners ([timers](timers.md)).

**Turn-based games** need neither shape: a CPU player there is a planner over the pure
[domain](domain.md) that sends the same commands a human seat sends, computed off the
game loop ([background-work](background-work.md)).

## Rules

- FSM state and per-phase timers live on the component; the system only dispatches.
- Sensing and acting are separate components and systems; the sensor writes, actuators
  read. Order sensor-before-actuator in the scene's system slice, with a comment.
- Spawners are components with `SpawnFunc` closures ticked by a system — never
  goroutines, never spawn logic buried in scene code.
- Movement is straight-line toward `TargetPosition` with one arrival-threshold constant per game (`MovementThreshold = 10.0`).

## Pitfalls

- **No pathfinding (A* / nav grid)** — a [catalog](README.md) staple
  to design fresh. Until then, author levels so straight lines work; say so explicitly.
- Drop states nothing dispatches (a declared `CustomerStateEntering` with no handler).
- Cached entity lookups in a system (a `queueSpotCache`) must self-heal on invalid
  entries; game state itself stays on components ([ecs-core](ecs-core.md)).
