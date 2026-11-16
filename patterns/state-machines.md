# State machines

## How it works

The house style: an **enum plus a switch, with all state on the component** — the state
field, per-phase [Timers](timers.md), target positions. The system holds only queries
and dispatch; one handler method per state; a transition is an assignment
(`customer.State = component.CustomerStateLeaving`) plus resetting the *next* phase's
timer at the moment of transition (logic co-location: reset where the state changes,
not in a loop that polls for it).

## Canonical code: enum + switch

Covered in depth in [ai](ai.md); the skeleton
(`component/customer.go`, `system/customer.go`):

```go
type CustomerState int

const (
    CustomerStateWandering CustomerState = iota
    CustomerStateInQueue
    CustomerStateThinking
    // ...
)

// in the system's Update, per entity:
switch customer.State {
case component.CustomerStateWandering:
    c.handleWandering(w, customerEntry, customer, velocity, customerTransform)
case component.CustomerStateThinking:
    c.handleThinking(w, customerEntry, customer, velocity)
// ... one case per state ...
}
```

Each `handleX` ticks that phase's timer, and only that one.

## The richest one: spell casting

State is encoded on `CasterData` as two flags
(`IsCasting`, `IsChannelling`) plus four per-spell timers
(`CastingTimer`, `ChannellingTimer`, `MaxChannellingTimer`, `CooldownTimer` — built in
`component.NewSpell`, which `Finish()`es the cooldown so spells start ready).

States and transitions (`system/casting.go`):

- **Idle/prepared**: every *non-prepared* spell's cooldown ticks unconditionally each
  frame; the prepared spell's cooldown ticks only while idle or waiting to start — it
  pauses during the wind-up itself. If `CastingTimer.IsStarted()` while `!IsCasting`,
  the button was released mid-cast: `Reset()` — interrupt.
- **Cast wind-up** (`IsCasting`, set by the input system): on the first tick
  (`!CastingTimer.IsStarted()`) require standing still unless
  `Template.CastWhileMoving`, require `CooldownTimer.IsReady()`, and pay the full
  `ManaCost` once. Then tick `CastingTimer`; moving mid-cast resets it (interrupt)
  unless `CastWhileMoving`.
- **Cast completes** (`CastingTimer.IsReady()`; trimmed of a debug-mode guard):

  ```go
  resolveSpellEffects(entry, entry, preparedSpell.Template.OnCastEffects)
  preparedSpell.CastingTimer.Reset()
  if preparedSpell.Template.IsChannel {
      preparedSpell.ChannellingTimer.Finish() // first channel tick fires immediately
      preparedSpell.MaxChannellingTimer.Reset()
      caster.IsChannelling = true
  }
  preparedSpell.CooldownTimer.Reset()
  ```

- **Channelling** (`IsChannelling`): moving interrupts unless
  `Template.ChannelWhileMoving`. Each time `ChannellingTimer` is ready, pay
  `ChannelTickManaCost` (fail → stop channelling), resolve `OnChannelTickEffects`,
  `Reset()`. `MaxChannellingTimer` ticks in parallel; ready → stop channelling.

The cast bar is pure timer readout — no extra state
(`archetype/player.go`):

```go
bar.Value = int(preparedSpell.CastingTimer.PercentDone() * 100)
```

Consider one explicit `CastingState` enum instead of the two bools — the flag pair
allows a fourth, meaningless combination the enum would rule out. Spell
numbers (`CastingTime`, `Cooldown`, effects) stay in data templates — see
[data-driven-rules](data-driven-rules.md).

## The scripted-sequence variant: a tutorial

`system/tutorial.go` is a state machine whose states are **data**: an ordered
`[]tutorialStep{Text, DialogPosition, OnShow, IsComplete(w, tutorialEvents) bool}` with
an index for a state field, advanced when the current step's `IsComplete` returns true
against the events that just fired. No enum, no switch — the slice order *is* the
transition table. Full treatment in [tutorial](tutorial.md).

## The counter-example: a dice game

Don't build a state machine when two ints and predicates express it. A Yatzy turn's
"phases" are derived (`scene/game.go`, `scene/table.go`):

```go
if g.rerolls < 2 { /* rolling phase: select and reroll dice */ }
if g.table.Full()  { /* game over */ }
if g.table.Ready() { /* score chosen: next round */ g.RollAllDice(); g.rerolls = 0 }
```

`Ready()`/`Full()` just scan the score boxes. There is no phase field to desync — the
"state" cannot disagree with the board because it *is* the board. Reach for the enum
only when phases have distinct per-tick behavior and timers, not for
turn-taking that data already implies.

## Rules

- Enum + switch + one `handleX` per state; state and timers on the component, dispatch
  in the system ([ecs-core](ecs-core.md)).
- Reset the next phase's timer at the transition that enters it.
- Interrupts are `Timer.Reset()` + a flag flip — never a second bookkeeping system.
- UI reads machine progress via `PercentDone()` (cast bars, customer patience bars);
  it never stores its own copy.
- Prefer derived predicates until a real multi-phase, timer-driven lifecycle appears;
  scripted sequences are data slices, not enums.

## Pitfalls

- Drop dead states: an enum case declared but never dispatched is a silent no-op state.
- A bool pair (`IsCasting`/`IsChannelling`) encoding one state is an enum waiting to
  happen.
