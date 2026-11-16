# Events

## How it works

donburi's `features/events` gives typed pub/sub scoped to a world. Declare a payload
struct plus an `events.NewEventType[T]()` var once; anything holding the world
publishes with `.Publish(w, T{...})`; systems subscribe from their `Init(w)`. Dispatch
is **deferred**: `Publish` only queues — handlers run when the scene calls
`events.ProcessAllEvents(w)` at the **end** of its `Update`, after every system ticked.

Two vocabularies, and the name carries the contract:

- **Commands** — imperative `XRequested`, published by UI/input, consumed by exactly
  one system that decides whether it happens (`NewRoadRequested` → `Buildings`).
- **Facts** — past-tense `XSpawned` / `XUpgraded` / `XUpdated`, published by the
  owning system after the change; any number of listeners react.

The split is what enables [replay](replay.md) (record commands, replay commands, facts
re-derive) and keeps audio/UI reactive: a `SettlementUpgraded` event fans out to
Audio, UI, Progress, Save, and Tutorial — none of which the publisher knows about
(see [audio](audio.md)).

## Canonical code

**Declaration** — an `events/` package holding vocabulary only, one file per topic
(`events/buildings.go`; siblings `resources.go`, `progress.go`, `ui.go`,
`save.go`, …):

```go
type SettlementUpgradeRequested struct {
	Settlement *donburi.Entry
}

var SettlementUpgradeRequestedEvent = events.NewEventType[SettlementUpgradeRequested]()

type SettlementUpgraded struct {
	Settlement *donburi.Entry
	Tile       *donburi.Entry
}

var SettlementUpgradedEvent = events.NewEventType[SettlementUpgraded]()
```

**Subscribe in Init** (`system/audio.go`):

```go
func (a *Audio) Init(w donburi.World) {
	events.RoadSpawnedEvent.Subscribe(w, a.onRoadSpawned)
	events.SettlementSpawnedEvent.Subscribe(w, a.onSettlementSpawned)
	events.SettlementUpgradedEvent.Subscribe(w, a.onSettlementUpgraded)
	events.ButtonClickedEvent.Subscribe(w, a.onButtonClicked)
	// ...
}

func (a *Audio) onRoadSpawned(w donburi.World, event events.RoadSpawned) {
	_ = a.click2Player.Rewind()
	a.click2Player.Play()
}
```

**Publish** — a command from a UI callback (`scene/board.go`):

```go
quitButton := archetype.NewButtonWithWidth(w, quitText, math.Vec2{X: buttonMargin, Y: 192}, false,
	func(w donburi.World, e *donburi.Entry) {
		gameevents.QuitGameRequestedEvent.Publish(w, gameevents.QuitGameRequested{})
	}, buttonWidth)
```

**Flush once, at the end of scene Update** (`scene/board.go`):

```go
func (b *Board) Update() {
	for _, s := range b.systems {
		s.Update(b.world)
	}

	events.ProcessAllEvents(b.world)
}
```

**Fan-out** — one fact, four independent single-purpose handlers (`system/events.go`):

```go
type EnemyKilled struct {
	Enemy *donburi.Entry
}

var EnemyKilledEvent = events.NewEventType[EnemyKilled]()

func OnEnemyKilledWreck(w donburi.World, event EnemyKilled) {
	if event.Enemy.Valid() && event.Enemy.HasComponent(component.Wreckable) {
		archetype.NewAirplaneWreck(w, event.Enemy, component.Sprite.Get(event.Enemy))
	}
}

func OnEnemyKilledAddScore(w donburi.World, event EnemyKilled) {
	component.MustFindGame(w).AddScore(1)
}

func OnEnemyKilledSpawnCollectible(w donburi.World, event EnemyKilled) {
	if !event.Enemy.Valid() {
		return
	}

	r := rand.Intn(10)
	if r < 2 {
		archetype.NewRandomCollectible(w, transform.Transform.Get(event.Enemy).LocalPosition)
	}
}

func OnEnemyKilledDestroyEnemy(w donburi.World, event EnemyKilled) {
	component.Destroy(event.Enemy)
}
```

Killing an enemy is one `Publish`; wreck VFX, score, loot, and destruction each stay a
function you can add or remove independently.

## Rules

- The `events/` package holds **vocabulary only** — payload structs + event type vars,
  split into topic files. No logic, no subscriptions.
- Name by contract: `XRequested` commands (one decider) vs past-tense facts (many
  listeners). UI publishes requests; owning systems publish facts.
- Subscribe in `Init(w)`; anything that publishes runs only after all systems
  initialized — the two-phase construction rule in [scenes](scenes.md).
- One `events.ProcessAllEvents(w)` at the end of scene `Update`; no per-system flushing.
- A new cross-system reaction (a sound, a UI refresh, a save-dirty mark) = a new
  subscription, not a new call site inside the publishing system.

## Pitfalls

- **donburi has no unsubscribe.** A handler that captures an entity outlives it —
  guard every such handler (`archetype/board.go`):

  ```go
  updateText := func() {
      // There's no way to unsubscribe, so check just in case if the entry still exists
      if !entry.Valid() {
          return
      }
      // ...
  }
  ```

- **Subscriptions are per-world.** A rebuilt world (R-restart, scene switch) starts
  with zero subscribers — always re-run `init()` after `createWorld()`, and never
  `Init` the same system twice on one world (handlers double up).
- **Early-flush hacks** — calling `domain.MoneyReceivedEvent.ProcessEvents(world)`
  mid-setup in `createWorld` (`// TODO a hack to not show the initial money/item
  received event`) exists only because setup published events before the UI should
  react. Avoid needing it: set initial state directly during setup and publish
  only for things that happen after init.

## Variations

- **One central file** (`domain/events.go` with all 23 event types) instead of an
  `events/` package — defensible when payloads are domain types, but one file scales
  worse than a per-topic split. Either way: one package, never scattered.
- An `events/` package that imports `component/` (payloads carry `*donburi.Entry` and
  `*component.ResourcesData`) — the events package sits above components, unlike
  `domain/`, which must stay ECS-free.
- **Per-type flushing** from a dedicated `Events` system
  (`EnemyKilledEvent.ProcessEvents(w)` in its `Update`) works, but prefer the single
  scene-level `ProcessAllEvents` so no type is forgotten.
- **Subscribing from archetype constructors** (closures updating hover
  text, with events declared in `component/`) — it works, but makes the `Valid()`
  guard mandatory; prefer system-owned subscriptions in `Init`.
