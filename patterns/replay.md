# Replay

Undo — revising an earlier choice within a running turn — is a different tool built on
the same determinism: replay the turn's commands on an isolated fork of the domain, and
refuse revisions that cross a commitment boundary (a die already rolled, information
already revealed). This doc covers recorded-game playback.

## How it works

A replay records **player intent, not state**: every command event the player can issue,
tagged with the tick it happened on, plus a snapshot of the starting board. Playback
seeds a fresh world from that snapshot and **re-publishes the exact same events** at the
matching ticks — the regular game systems do the rest, exactly as they did live. Nothing
in the game knows it is being replayed except the input layer (disabled) and the
transport panel (added).

This works because of two existing invariants: the command-vs-fact event split (a
replay re-publishes commands like `NewRoadRequested`, and the fact events they cause
follow naturally — see [events.md](events.md)), and deterministic frame-count timers
(the same tick always means the same amount of elapsed game — see
[timers.md](timers.md)).

## Canonical code

**Schema** — an `Action` is a type + tick + oneof payload over the five command events
(`save/actions.proto`):

```proto
message GameReplay {
  Header header = 1;
  Board board = 2;
  Player player = 3;
  repeated Action actions = 4;
  int64 last_tick = 5;
}

message Action {
  ActionType type = 1;
  int64 tick = 2;
  ActionPayload payload  = 3;
}

message ActionPayload {
  oneof payload {
    NewRoadRequested new_road_requested = 1;
    NewSettlementRequested new_settlement_requested = 2;
    SettlementUpgradeRequested settlement_upgrade_requested = 3;
    CameraMoved camera_moved = 4;
    ResearchRequested research_requested = 5;
  };
}
```

**Recording** — a system subscribes to the command events and appends `{tick, payload}`;
the tick advances once per frame unless paused (`system/actions_record.go`):

```go
func (a *ActionsRecord) Init(w donburi.World) {
    a.actionsRecorder = engine.MustFindComponent[component.ActionsRecorderData](w, component.ActionsRecorder)

    events.NewRoadRequestedEvent.Subscribe(w, func(w donburi.World, event events.NewRoadRequested) {
        tile := component.Tile.Get(event.Tile)
        a.record(
            save.ActionType_ActionNewRoadRequested,
            &save.ActionPayload{
                Payload: &save.ActionPayload_NewRoadRequested{
                    NewRoadRequested: &save.NewRoadRequested{
                        Position: mapPositionToSave(tile.Position),
                    },
                },
            },
        )
    })
    // same for NewSettlementRequested, SettlementUpgradeRequested,
    // CameraMoved, ResearchRequested
}

func (a *ActionsRecord) Update(w donburi.World) {
    if a.game.Paused {
        return
    }
    a.actionsRecorder.CurrentTick++
}
```

The replay file's board snapshot saves deposits **full** — `newBoardForSave(w, true)`
writes `Amount = MaxAmount` (`system/save.go`) — because playback starts from
tick 0 and the recorded actions re-deplete them.

**Playback** — advance the tick (scaled by speed), pop due actions, re-publish the same
events (`system/actions_play.go`):

```go
func (a *ActionsPlay) Update(w donburi.World) {
    if a.game.Paused || a.actionsPlayer.Paused {
        return
    }

    a.actionsPlayer.CurrentTick += 1 * a.actionsPlayer.Speed

    for _, action := range a.actionsPlayer.GetActionsForCurrentTick() {
        a.replayAction(w, action)
    }
}

func (a *ActionsPlay) replayAction(w donburi.World, action component.Action) {
    board := engine.MustFindComponent[component.BoardData](w, component.Board)

    switch action.ActionType {
    case save.ActionType_ActionNewRoadRequested:
        payload := action.Payload.GetNewRoadRequested()
        tile := board.MustTileByPosition(MapPositionFromSave(payload.Position))
        events.NewRoadRequestedEvent.Publish(w, events.NewRoadRequested{Tile: tile})
    // ... one case per ActionType, each just re-publishing the command event
    }
}
```

**Transport state** lives on a singleton component: `ActionsPlayerData` holds
`CurrentTick float64`, `Speed`, `Paused`, `Finished` with `Play`/`Pause`/`Finish`/
`SetSpeed` helpers; `GetActionsForCurrentTick` consumes the sorted action list and calls
`Finish()` when empty (`component/actions.go`).

**Wiring** — the replay scene disables input (`game.ActionsDisabled = true`), registers
`system.NewActionsPlay()` instead of `NewActionsRecord()`, loads actions into
`ActionsPlayerData`, and builds the transport UI: rewind / pause / play / 10x buttons
plus a progress marker whose animation maps `CurrentTick / maxTick` onto the bar
(`scene/board_replay_game.go`, `archetype/replay_panel.go`).

## Rules

- Record commands (requests), never facts or state deltas. The five recorded events are
  exactly the player-intent events; `CameraMoved` is included so the replay watches
  what the player watched.
- One tick counter, incremented once per un-paused frame, owned by a component — not by
  the system (`ActionsRecorderData.CurrentTick`).
- Recording is always on: the normal game scene registers `NewActionsRecord` and writes
  the replay alongside every save (`Save.save` calls both — see
  [save-load.md](save-load.md)); a loaded game re-loads the recorded actions and keeps
  appending (`ActionsRecorderData.Load(lastTick, actions)`).
- Playback re-publishes events through the same publish calls gameplay uses. If a
  replayed reference can't be resolved (settlement ID not found), panic — the replay is
  corrupt or determinism broke.
- **Determinism is the invariant.** Any new nondeterminism in a system — unseeded
  `rand`, wall-clock time, map-iteration-order-dependent logic that touches game state —
  silently breaks every existing replay. Keep `rand` confined to world generation
  (`scene/generator/`), which replays bypass via the board snapshot. Seed
  anything random a system does, and treat "replays diverge" as a determinism bug.

## Variations

- **Full event sourcing** gives replayability for free (`Board.History()` — see
  [networking.md](networking.md)); the action log above is the cheaper pattern when
  there is no domain event stream to fold.
