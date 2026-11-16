# Input: keyboard

## How it works

Bindings are **data on an `Input` component**, populated at entity creation. Systems poll
`ebiten.IsKeyPressed` / `inpututil.IsKeyJustPressed` over the bound key slices every tick —
no action-mapping layer, no input buffering. `Disabled` on the same component is the one
modal-UI mechanism: whoever opens a modal flips it, whoever closes it flips it back. Input
writes into `Velocity` (or starts a grid step) — never the transform directly
(see [movement.md](movement.md)).

## Canonical code

**The component** — key *slices*, so one action can have several bindings
(`component/input.go`):

```go
type InputData struct {
    Disabled bool

    MoveUpKeys    []ebiten.Key
    MoveRightKeys []ebiten.Key
    MoveLeftKeys  []ebiten.Key
    MoveDownKeys  []ebiten.Key
    PickUpKeys    []ebiten.Key
    InteractKeys  []ebiten.Key

    MoveSpeed float64
}
var Input = donburi.NewComponentType[InputData]()
```

**Populated at entity creation**, never inside a system (`scene/game.go`):

```go
localComponent.Input.SetValue(player, localComponent.InputData{
    MoveUpKeys:    []ebiten.Key{ebiten.KeyW, ebiten.KeyUp},
    MoveRightKeys: []ebiten.Key{ebiten.KeyD, ebiten.KeyRight},
    // ...MoveLeftKeys, MoveDownKeys likewise...
    PickUpKeys:    []ebiten.Key{ebiten.KeySpace},
    InteractKeys:  []ebiten.Key{ebiten.KeyE},
    MoveSpeed:     settings.Player.MoveSpeed,
})
```

**The polling system** — reset velocity, poll the slices, normalize diagonals
(`system/player_input.go`, trimmed of gamepad handling — see
[input-gamepad.md](input-gamepad.md) — and a network-test key buffer):

```go
func (p *PlayerInput) Update(w donburi.World) {
    p.query.Each(w, func(entry *donburi.Entry) {
        input := component.Input.Get(entry)
        velocity := component.Velocity.Get(entry)

        if input.Disabled {
            return
        }

        velocity.Velocity = donburimath.Vec2{X: 0, Y: 0}

        var moveX, moveY float64
        if p.isAnyKeyPressed(input.MoveUpKeys) {
            moveY = -1
        }
        if p.isAnyKeyPressed(input.MoveDownKeys) {
            moveY = 1
        }
        // ...MoveLeftKeys / MoveRightKeys set moveX the same way...

        if moveX != 0 || moveY != 0 {
            // Normalize diagonal movement
            length := math.Sqrt(moveX*moveX + moveY*moveY)
            if length > 1 {
                moveX /= length
                moveY /= length
            }
            velocity.Velocity = donburimath.Vec2{
                X: moveX * input.MoveSpeed,
                Y: moveY * input.MoveSpeed,
            }
        }
        // ... action keys: isAnyKeyJustPressed(input.PickUpKeys) etc.
    })
}

func (p *PlayerInput) isAnyKeyPressed(keys []ebiten.Key) bool {
    for _, key := range keys {
        if ebiten.IsKeyPressed(key) {
            return true
        }
    }
    return false
}
```

**`Disabled` is the modal-UI mechanism.** A computer-terminal UI sets it on open, clears it
on close, and zeroes velocity so the player doesn't slide (`archetype/computer_ui.go`):

```go
input := component.Input.Get(player)
input.Disabled = true
velocity := component.Velocity.Get(player)
velocity.Velocity = math.Vec2{X: 0, Y: 0}
```

Lock input the same way during level transitions — the transition animation's
`OnStart` sets `Disabled`, the fade-in's end clears it (`archetype/level.go`);
its dialogs do the same.

## Tap-to-turn, hold-to-walk (grid games)

Grid games disambiguate "select the tile I'm facing" from "walk there": a *new* direction
only turns in place and resets a grace timer; a step happens once the same direction is
still held when the timer is ready. The trick is starting the timer **already finished**,
so the first press in the spawn facing steps immediately (`archetype/player.go`):

```go
// Roughly the turn-in-place grace of the old Pokémon games: shorter and
// tapping feels like it eats inputs, longer and walking feels laggy.
turnTimer := engine.NewTimer(150 * time.Millisecond)
// Start ready, so the first press in the spawn facing steps immediately.
turnTimer.Finish()
```
The turn branch (`system/player.go`):

```go
if facing != player.Facing {
    player.Facing = facing // (sprite flip trimmed)
    player.TurnTimer.Reset()
    return
}
player.TurnTimer.Update()
if !player.TurnTimer.IsReady() {
    return
}
// ...claim the target cell and start the glide...
```

## Per-player bindings (local multiplayer)

Key a bindings map by player number; `NewPlayer` copies the entry onto that player's
`InputData` (`archetype/player.go`):

```go
var PlayerBindings = map[int]component.InputData{
    1: {
        MoveUpKeys:    []ebiten.Key{ebiten.KeyW},
        MoveRightKeys: []ebiten.Key{ebiten.KeyD},
        MoveDownKeys:  []ebiten.Key{ebiten.KeyS},
        MoveLeftKeys:  []ebiten.Key{ebiten.KeyA},
        InteractKeys:  []ebiten.Key{ebiten.KeySpace},
    },
    2: {
        MoveUpKeys:    []ebiten.Key{ebiten.KeyUp},
        MoveRightKeys: []ebiten.Key{ebiten.KeyRight},
        MoveDownKeys:  []ebiten.Key{ebiten.KeyDown},
        MoveLeftKeys:  []ebiten.Key{ebiten.KeyLeft},
        InteractKeys:  []ebiten.Key{ebiten.KeyEnter},
    },
}
```

One input system serves every player — the query finds each `Input` entity and reads
its own keys.

## Rules

- **Every binding lives on `InputData`** — never hardcode a key inside a system. The one
  sanctioned exception is global dev shortcuts (debug toggle `/`, restart `R`).
- Key fields are slices; check them with an `isAnyKeyPressed`-style loop. Held state via
  `ebiten.IsKeyPressed`, one-shots via `inpututil.IsKeyJustPressed` — no custom edge
  detection.
- `Disabled` gates the whole entity's input; modal UI toggles it and zeroes velocity.
- Normalize diagonal movement before applying `MoveSpeed`.
- Register the input system before the systems that consume its output —
  `PlayerInput` → `Velocity` → `Collision` (`scene/game.go`).

## Pitfalls

- **There is no rebinding UI and no action-abstraction layer** (see the staples in
  [README.md](README.md)) — the data-driven component is deliberately the whole
  abstraction. If a settings screen
  is ever needed, it edits `InputData` and nothing else.
- A player system that hardcodes WASD instead of reading `InputData` is the usual
  regression when a game is trimmed down; keep the component even with one player.
