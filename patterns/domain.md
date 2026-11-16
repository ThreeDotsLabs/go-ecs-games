# Domain separation

## How it works

`domain/` holds the game's *rules* with **zero ebiten and zero donburi** — a healthy
`domain/` passes `grep -rn "ebiten\|donburi"` clean. The ECS is presentation: systems call
into the domain, and the domain answers with **events** that systems drain and animate.
Components store domain **IDs**, never copies of domain data. This buys three things
seen nowhere else in the collection: the rules are unit-testable, the same rules run
hotseat / vs-AI / online with identical rendering, and the server can execute them
headless (see [networking.md](networking.md)).

## Canonical code

**Narrow public surface** — the entire mutating/query API; every other method is
unexported, and while the state fields (`Tiles`, `Players`, `Spaces`) are exported, the
ECS never reads them — it consumes events only (`domain/board.go`):

```go
func NewBoard(player1ID string, player2ID string) *Board
func (b *Board) MoveDisc(discID string, spaceID int)   // the only move
func (b *Board) PopEvents() []Event                    // drain since last call
func (b *Board) History() []Event                      // full log (replay/debug)
func (b *Board) Winner() *Player

func (b *Board) PopEvents() []Event {
    events := b.events
    b.events = []Event{}
    return events
}
```

Every state change appends events (`EventDiscMoved`, `EventTileMovedToCourt`,
`EventWaitForPlayerInput`…); the caller learns what happened only by popping them.

**Rules as data** — space behavior is an action table executed by one `resolveAction`
interpreter (`domain/space.go`; see
[data-driven-rules.md](data-driven-rules.md)):

```go
var spaceActions = map[SpaceType][]Action{
    SpaceTypeDraw: {
        {ActionType: ActionTypeDrawTile, TargetType: TargetTypeSelf},
    },
    SpaceTypeGold: {
        {ActionType: ActionTypePlaceOnCourt, TargetType: TargetTypeSelf, TileColor: ColorGold},
        {ActionType: ActionTypeScoreTiles, TargetType: TargetTypeSelf, TileColor: ColorGold},
    },
    // ...
}
```

**The seam** — the ECS never holds `*domain.Board`; it holds a 2-method interface
(`component/board.go`). This is the single best architectural idea in the
collection:

```go
type BoardAdapter interface {
    PopEvents() []domain.Event
    MoveDisc(discID string, spaceID int)
}
```

Three swappable implementations, chosen in `scene/game.go` — rendering and input are
100% shared across all three:

- **Practice/hotseat**: `*domain.Board` itself (it already satisfies the interface).
- **vs AI** (`scene.AIBoard`): wraps the board; in `PopEvents` it intercepts
  `EventWaitForPlayerInput` addressed to the AI, asks `ai.ChooseMove(...)`, plays it
  immediately, and filters out `EventTileRevealed` for the AI's hidden tiles. The UI
  never learns the AI's hand.
- **Online** (`scene.ServerSideBoard`): `MoveDisc` forwards to the websocket client; the
  message handler appends server-sent events to a slice that `PopEvents` drains. The
  domain runs on the server; the client is a dumb renderer of the same event stream.

**Event-handler queue** — domain events arrive in a burst (one `MoveDisc` can emit ten),
but animations take time. Each event becomes a queued `EventHandler func() (done bool)`;
exactly one runs per tick until it reports done (`component/board.go`,
`system/game_logic.go`):

```go
type EventHandler func() (done bool)
type EventHandlerConstructor func() EventHandler

case *domain.EventDiscMoved:
    boardData.AddEventHandler(func() component.EventHandler {
        disc := getDiscByID(w, e.DiscID)
        space := getSpaceByID(w, e.SpaceID)

        moveDiscToSpace(disc, space)

        return waitUntilNothingMoves(w)
    })

// tail of GameLogic.Update — the queue pump:
if boardData.CurrentEventHandler == nil {
    handler := boardData.NextEventHandler()
    if handler != nil {
        boardData.CurrentEventHandler = handler
    }
}

currentHandler := boardData.CurrentEventHandler
if currentHandler != nil {
    done := currentHandler()
    if done {
        boardData.CurrentEventHandler = nil
    }
}
```

`waitUntilNothingMoves` returns a closure that reports done once no `Movable` is moving —
animations serialize against the event stream without the domain knowing animations
exist. The constructor indirection matters: world lookups run when the handler *starts*,
not when the event arrived.

**Components store IDs, not copies** — the item component is one field plus a
delegating getter; prices, names and kinds stay in the domain catalog
(`component/item.go`):

```go
type ItemData struct {
    ItemID domain.ItemID
}

func (d ItemData) GetItem() domain.Item {
    return domain.GetItemByID(d.ItemID)
}
```

## Rules

- `domain/` imports no ebiten and no donburi. Rules, constants, balance tables, event
  types — nothing else (see [project-structure.md](project-structure.md)).
- Export the minimum: constructors, the move methods, `PopEvents`. Internal helpers and
  fields stay unexported so invariants hold.
- State changes communicate via events pulled with `PopEvents()` — the domain never calls
  back into the caller.
- The ECS depends on a small adapter interface, not the concrete domain type; every
  alternate mode (AI, network, replay) is a new adapter, not a fork of the scene.
- Convert events to queued handlers; run one at a time gated on
  `waitUntilNothingMoves`-style predicates.
- Components store domain IDs and delegate lookups. Duplicating a domain field into a
  component is a bug factory.

## Pitfalls

- **Domain panics are a server crash vector.** `Board.MoveDisc` panics on invalid input —
  acceptable when the caller is your own UI, fatal when the domain runs server-side
  behind a websocket. Validate at the transport boundary or return errors from the
  domain when a server executes it (see [networking.md](networking.md)).
- **Keep art out of the domain.** `*ebiten.Image` fields in domain structs
  (`TerrainSprites` on a tile) couple the rules to asset load order. Store sprite
  *names/IDs* in domain and resolve them in `assets/`/`archetype/`, or inject.
- **Named acceptable leak:** a `domain.Story` that holds a `donburi.World` solely to
  publish events (`domain/story.go`), or donburi event types declared in
  `domain/events.go`. Both keep the *rules* pure but tie the package to donburi.
  Prefer the pull model (`PopEvents`); if you keep the leak, keep it this shallow —
  world reference for publishing only, never queries or entity access.

## Variations

- **The minimal form**: `scoring.go` as pure functions over plain dice data with
  exhaustive table tests (`domain/scoring_test.go`) — the habit to keep from the first
  rule (see [testing.md](testing.md)).
- **A read model** (`domain/readmodel/`): a snapshot type that folds the domain event
  stream (`Update(events []domain.Event)`) into a serializable board view — the
  server-side answer for late joins and reconnects.
- **The table-catalog end of the spectrum**: no engine object, just typed
  constants, rule slices and `CanX` lookups (see
  [data-driven-rules.md](data-driven-rules.md)).
