# Tutorial

## How it works

The tutorial is the ordinary board scene plus **one extra system**. Steps are an
ordered package-level slice of data — text, dialog position, an `OnShow` hook, an
`IsComplete` predicate. The system subscribes to gameplay [events](events.md); each
event immediately calls `checkProgress` with a carrier struct holding *that one event*;
when the current step's `IsComplete` returns true, its dialog is destroyed and the next
step's is shown. `Update` is empty — there is no per-tick polling. This is the
data-driven variant of a [state machine](state-machines.md): the slice order is the
transition table, the step index is the state.

## Canonical code

The step shape and the event carrier (`system/tutorial.go`):

```go
type tutorialStep struct {
    Text           string
    DialogPosition math.Vec2
    OnShow         func(w donburi.World)
    IsComplete     func(w donburi.World, events tutorialEvents) bool
}

type tutorialEvents struct {
    CameraMoved        *events.CameraMoved
    TileSelected       *events.TileSelected
    RoadSpawned        *events.RoadSpawned
    SettlementSpawned  *events.SettlementSpawned
    ResourcesUpdated   *events.ResourcesUpdated
    SettlementUpgraded *events.SettlementUpgraded
}
```

Two steps from `tutorialSteps` (same file, texts trimmed):

```go
{
    Text: `Click on the map to build a road.

You need to connect it to one of the roads.`,
    DialogPosition: math.Vec2{X: 96, Y: 50},
    OnShow:         func(w donburi.World) {},
    IsComplete: func(w donburi.World, events tutorialEvents) bool {
        return events.RoadSpawned != nil
    },
},
{
    Text: `Well done! ...`, // "Build a road to the mountains tile to collect stone."
    DialogPosition: math.Vec2{X: 96, Y: 50},
    OnShow:         func(w donburi.World) {},
    IsComplete: func(w donburi.World, events tutorialEvents) bool {
        return events.ResourcesUpdated != nil && events.ResourcesUpdated.Delta.Stone > 0
    },
},
```

A step completes off the event *and its payload* — the mountains step waits for a
resource tick that actually gained stone, not merely for any resource change.

Every subscription funnels into one advance path (same file):

```go
func (t *Tutorial) Init(w donburi.World) {
    t.ShowCurrentStep(w)
    events.RoadSpawnedEvent.Subscribe(w, func(w donburi.World, e events.RoadSpawned) {
        t.checkProgress(w, tutorialEvents{RoadSpawned: &e})
    })
    // ... one Subscribe per event type ...
}

func (t *Tutorial) checkProgress(w donburi.World, events tutorialEvents) {
    if t.currentStep >= len(tutorialSteps) {
        return
    }
    if tutorialSteps[t.currentStep].IsComplete(w, events) {
        t.currentStep++
        t.HideStep(w)
        t.ShowCurrentStep(w)
    }
}
```

`ShowCurrentStep` builds the dialog with `archetype.NewDialogWithButton`, tags it
`component.TutorialDialog`, and lifts it to `SpriteUILayerTutorial`; the final step
(whose `IsComplete` returns `false` forever) gets a Close button instead. `IsComplete`
can also inspect the world, not just the payload — the "click a settlement" step checks
`transform.FindChildWithComponent(events.TileSelected.Tile, component.Settlement)`.

## The tutorial map is a fixed seed

Authored content goes **through the generator** — there is no separate level format.
`assets/map/tutorial.yaml`:

```yaml
seed: 19
width: 100
height: 100
```

The seed pins the terrain, so step text like "build a road to the mountains tile" is
always true (see [procgen](procgen.md)). `scene/board_new_game.go` completes the
setup: a hardcoded start position (`42, 30`), fixed starting resources
(`Wood: 5, Stone: 10`), and the intro message skipped.

## Wiring: the same scene, one extra system

Tutorial mode is a flag into the board scene's creator, which appends the system
(`scene/board_new_game.go`; see [scenes](scenes.md)):

```go
func (c *newGameBoardCreator) ExtraSystems() []System {
    var systems []System
    if c.tutorial {
        systems = append(systems, system.NewTutorial())
    } else {
        systems = append(systems, system.NewActionsRecord())
    }
    return systems
}
```

The swap also disables [replay](replay.md) recording, and `scene/board.go` disables
saving (`canSave := saveSlot != nil && !tutorial`).

## Rules

- Steps are data; behavior lives in the two closures. Adding a step is editing the
  slice, nothing else.
- Complete steps off **events and their payloads** (`Delta.Stone > 0`), querying the
  world only when the payload isn't enough.
- The tutorial system never forks gameplay — the same systems run; it only watches and
  talks. Keep tutorial-only divergence in the scene creator (resources, seed, no save).
- Pure-instruction steps ("choose the road tool") use `IsComplete: return true` — they
  advance on the *next event of any kind*. Cheap and accepted here, but see below.

## Pitfalls

- `return true` steps advance on any event, not on the instructed action (tool
  selection fires no event) — an unrelated event can skip the step early. Emit a real
  event for anything a step must gate on.
- Dialogs are plain `NewDialogWithButton` panels; the character-portrait
  `archetype.ShowMessage` used elsewhere in the game is *not* part of the tutorial —
  pick one presentation and keep it.
