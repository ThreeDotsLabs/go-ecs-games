# Networking

## How it works

**Authoritative server, dumb clients.** The server is a second `package main` in the
same repo (`server/`) that literally runs `domain.Board`; clients never simulate rules
online — they send intent messages and fold the server's domain events into their
world. Shared packages: `client/` (transport + message catalog, imported by both
binaries) and `domain/` (pure rules — see [domain.md](domain.md)). The protocol is JSON
over WebSocket (`github.com/coder/websocket`); state sync is **event sourcing** — the
domain appends events, the server flushes them per player, and a client rebuilds any
board (late join and reconnect included) by folding the event history.

## Canonical code

**Protocol & dispatch** — every message embeds `MessageHeader{Type}`; the receiver
unmarshals twice: once into the header to learn the type, then into the concrete struct
from a constructor map (`client/messages.go`, `client/websocket.go`):

```go
var MessageFromType = map[string]func() Message{
    "connect":     func() Message { return &Connect{} },
    "board_event": func() Message { return &BoardEvent{} },
    // ... ~20 types
}

type Message interface{ MessageType() string }

type MessageHeader struct {
    Type string `json:"type"`
}

func (w *WebSocketConn) ReadMessage(ctx context.Context) (Message, error) {
    message, err := w.conn.Read(ctx)
    // ...
    var msg MessageHeader
    err = json.Unmarshal(message, &msg)      // pass 1: learn the type
    // ...
    constructor, ok := MessageFromType[msg.Type]
    if !ok {
        return MessageHeader{}, fmt.Errorf("unknown message type: %s", msg.Type)
    }
    msgType := constructor()
    err = json.Unmarshal(message, msgType)   // pass 2: fill the concrete struct
    return msgType, nil
}
```

Domain events ride inside one message type with the same trick one level down:
`BoardEvent{EventName, EventData json.RawMessage}` dispatched through
`domain.EventConstructors[EventName]` (`domain/events.go`).

**Transport portability** — one interface, two build-tagged files: THE trick for one
binary that networks both native and in the browser (`client/websocket.go`):

```go
type WebsocketConn interface {
    Read(ctx context.Context) ([]byte, error)
    Write(ctx context.Context, data []byte) error
    Close(status websocket.StatusCode, reason string) error
}
```

`connection_native.go` (`//go:build !wasm`) wraps `coder/websocket` `Dial`/`Accept`
(the server uses `Accept` from the same file). `connection_wasm.go` drives the raw
browser WebSocket via `syscall/js`, feeding callbacks into a channel `Read` blocks on:

```go
//go:build wasm

func dial(ctx context.Context, url string) (WebsocketConn, error) {
    ws := js.Global().Get("WebSocket").New(url)
    conn := &wasmConn{ws: ws, recvChan: make(chan []byte, 10), errChan: make(chan error, 1)}

    ws.Call("addEventListener", "message", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
        conn.recvChan <- []byte(args[0].Get("data").String())
        return nil
    }))
    // "error" → errChan; "close" → close(recvChan); block on "open" before returning
    return conn, nil
}

func (c *wasmConn) Read(ctx context.Context) ([]byte, error) {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case data := <-c.recvChan:
        return data, nil
    case err := <-c.errChan:
        return nil, err
    }
}
```

**Server concurrency** — one mutex over rooms/sessions; each session owns a buffered
channel (`Messages chan client.Message, 16`) drained by a per-connection writer
goroutine, so handlers never block on I/O — `SendToPlayer` just pushes to the channel
(`server/server.go`, `server/domain.go`). **Client concurrency** mirrors it:
a reader goroutine fills a buffered channel, and the game drains **one message per
ebiten tick**, so all world mutation stays on the game loop's goroutine
(`main.go`):

```go
func (g *Game) Update() error {
    // Processing a single message per update should be good enough
    select {
    case msg := <-g.client.Messages():
        g.currentScene.HandleMessage(msg)
    default:
    }

    g.currentScene.Update()
    // ...
}
```

**Sessions & reconnect** — on `connect` the server issues uuid `PlayerID` +
`SessionToken` in `connected`; the client persists them via the `saveFile`/`loadFile`
storage pair (see [save-load.md](save-load.md)) and next launch sends
`connect_with_session` instead. An `error` with slug `invalid_session` makes the client
close and wipe the stored session (`client/client.go`). Server-side, a drop only marks
the room player `Connected=false` — rejoinable — and all-disconnected rooms are GC'd
(`server/domain.go` `EndSession`/`checkEmptyRooms`).

**State sync = event sourcing** — after every mutating command the server pops the new
domain events and flushes them per player (`server/domain.go`):

```go
func (s *Server) flushEvents(roomID string, playerID string, events []domain.Event) error {
    for _, event := range events {
        forPlayer, ok := event.(domain.EventForPlayer)
        var eventForPlayerID string
        if ok {
            eventForPlayerID = forPlayer.EventForPlayerID()
        }
        bytes, err := json.Marshal(event)
        // ...
        if eventForPlayerID != "" && playerID != eventForPlayerID {
            continue  // hidden info: another player's event is silently skipped
        }
        s.SendToPlayer(playerID, client.BoardEvent{
            MessageHeader: client.MessageHeader{Type: "board_event"},
            RoomID:    roomID,
            EventName: event.EventName(),
            EventData: bytes,
        })
    }
    return nil
}
```

`FlushEventsToAllPlayers` → `Board.PopEvents()` → `flushEvents` per player. The board
also keeps everything in `Board.History()`; `get_room` replays the full history, which
makes late join and reconnect free — the client scene is a pure event fold, so
replaying history rebuilds the exact view (`domain/board.go`).

**Client-side seams** (`scene/game.go`):

- `scene.HandleMessage(msg)` on the `Scene` interface routes inbound messages to the
  current scene (see [scenes.md](scenes.md)): `room_details`, `board_event`, chat.
- **`BoardAdapter`** makes online, local, and AI play interchangeable: the board
  component holds `BoardAdapter{PopEvents() []domain.Event; MoveDisc(discID, spaceID)}`
  (`component/board.go`). Local plugs in `domain.Board`; AI wraps it in `AIBoard`
  (answers wait-for-input events itself); online plugs in `ServerSideBoard`, whose
  `MoveDisc` sends the message and whose `PopEvents` returns what `HandleMessage` queued.
- **Placeholder-ID rewrite**: the scene is built before the server answers, on
  placeholder player IDs; when `room_details` arrives, `archetype.UpdatePlayerIDs(w,
  id1, id2)` swaps them for real IDs in every player-owning component.
- **One error path**: `sendLocalErrorMessage` injects local failures (dial timeout, no
  connection) into the same inbound channel as `ErrorMessage{Slug: "client_error"}`, so
  scenes handle local and server errors identically (`client/client.go`).

## Message catalog

| Direction | Types |
|---|---|
| client → server | `connect`, `connect_with_session`, `get_rooms`, `host_room`, `join_room`, `leave_room`, `get_room`, `player_ready`, `move_disc`, `send_chat_message` |
| server → client | `connected`, `available_rooms`, `room_joined`, `player_joined`, `player_left`, `room_details`, `board_event`, `chat_message_sent`, `error` |

Lobby flow: `host_room` → broadcast `available_rooms`; `join_room` → `room_joined` +
`room_details` to peers; `player_ready` on all → `domain.NewBoard(...)` + first flush.

## Rules

- The server owns the rules. A client-side `domain.Board` exists only for offline
  modes; online, the client's board adapter is transport.
- One dispatch mechanism everywhere: header type + constructor map, at both the message
  layer and the nested event layer.
- Handlers never write to sockets — they push to per-session channels; one writer
  goroutine per connection.
- The game loop is the only goroutine that touches the world; everything async funnels
  through the one buffered channel drained once per tick.
- Hidden information is the event's concern (`EventForPlayer`), enforced at flush time
  on the server — never filtered client-side.

## Pitfalls

- **Validate ownership on the server** before calling the domain: a `move_disc` handler
  that doesn't check the mover owns the disc lets any client move anything.
- **Domain panics are a server crash vector**: a domain that panics on invalid moves is
  fine client-side, fatal when a hostile message reaches it server-side. Return errors
  at the boundary (or recover per-message) when hosting.
- An in-memory `StoredSessions` map means a server restart invalidates every session.
- **Reconnect from the game loop.** Without a reconnect loop a dropped client stays
  dropped until relaunch. Redial from the client adapter's own `Tick`, so the game loop
  stays the only goroutine that touches its state: a lost link starts a resume, each
  attempt is one goroutine whose result comes back on a channel the tick drains, the
  backoff is counted in ticks, and the session token from `welcome` goes out in the
  resume hello. The ECS never sees the connection; it reads the link state for a banner.
  Keep the token per server address (`client/storage.go`).
- **`coder/websocket`'s `Close(code, reason)` is a close handshake** that waits up to
  5 s for the read lock, which the reader goroutine holds while blocked. A server that
  closes links under a room mutex must call `CloseNow()` there, or one dead client
  stalls everyone's tick. The wasm `message` callback must not block either: a hidden
  tab stops the game loop that drains it, so a full buffer has to drop the link, not
  the event loop (`client/connection_wasm.go`).

## The game-agnostic boundary

The transport splits cleanly from the game. These files carry over to a new networked
game unchanged apart from module path and app-name strings: `client/websocket.go`,
`connection_native.go`, `connection_wasm.go`, `storage.go`, `storage_wasm.go` (and
`storage_native.go` modulo the config-dir name), `server/main.go`, `server/server.go`.
Rewritten per game: the command messages in `client/messages.go`/`client.go`
(`move_disc`, `board_event`) and the `HandleMessage` game-start/move cases in
`server/domain.go` (~100 lines).

## Variations: scaling up

- **Binary protocol.** Keep the two-pass JSON decode for logs and tests, and speak
  protobuf on the wire: every message inside an `Envelope` whose `oneof` is the type
  header, sent as binary frames. Keep the Go structs as the source of truth — the domain
  never imports the generated package; a hand-written mapping carries each struct onto
  its protobuf both ways, and a test holds the mapping complete field for field. Worth
  it once snapshots stream at 20 Hz.
- **Rooms.** A headless server can be a hub of rooms rather than one room: a hello with
  no room code opens a room (a fresh server on its own goroutine) and the welcome
  carries its four-digit code; a hello with a code joins that room; a resume goes to the
  room that knows the token. A room with nobody connected pauses its tick loop and the
  hub closes it after ten idle minutes.
- **An embedded server** for single player: the same server runs in-process over an
  in-memory pipe, so solo and online play share one code path.
