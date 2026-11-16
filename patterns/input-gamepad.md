# Input: gamepad

## How it works

Gamepad input rides in the same input system as the keyboard (see
[input-keyboard.md](input-keyboard.md)) and writes into the same `Velocity` / action
handlers. Sticks are read as raw axis values with a deadzone; buttons through ebiten's
`StandardGamepadButton*` layout so mappings survive across controllers. For local
multiplayer, the *device* a player joined with is data on their `Input` component — a
`GamepadID *ebiten.GamepadID` where `nil` means keyboard — chosen once in a join screen
and read by the controls system every tick.

## Canonical code

**Modern polling** — the canonical excerpt (`system/player_input.go`; keyboard has
already set `moveX`/`moveY`, gamepad overrides it):

```go
gamepadIDs := ebiten.AppendGamepadIDs(nil)
if len(gamepadIDs) > 0 {
    gamepadID := gamepadIDs[0]
    leftStickX := ebiten.GamepadAxisValue(gamepadID, 0) // Left stick X
    leftStickY := ebiten.GamepadAxisValue(gamepadID, 1) // Left stick Y

    // Apply deadzone
    const deadzone = 0.2
    if math.Abs(leftStickX) > deadzone {
        moveX = leftStickX
    }
    if math.Abs(leftStickY) > deadzone {
        moveY = leftStickY
    }
}
```

Buttons in the same system. Use the standard layout, not raw indices
(`inpututil.IsGamepadButtonJustPressed(gamepadID, 0)` means a different button on every
controller):

```go
// system/controls.go
caster.IsCasting = ebiten.IsKeyPressed(input.CastKey) ||
    input.GamepadID != nil &&
        ebiten.IsStandardGamepadButtonPressed(*input.GamepadID, ebiten.StandardGamepadButtonFrontBottomRight)
```

**Twin-stick movement + aim** (`system/controls.go`):

```go
if input.GamepadID != nil {
    lx, ly := ebiten.GamepadAxisValue(*input.GamepadID, 0), ebiten.GamepadAxisValue(*input.GamepadID, 1)
    if stdmath.Abs(lx) > 0.2 || stdmath.Abs(ly) > 0.2 {
        delta.X += lx * moveSpeed
        delta.Y += ly * moveSpeed
    }

    rx, ry := ebiten.GamepadAxisValue(*input.GamepadID, 2), ebiten.GamepadAxisValue(*input.GamepadID, 5)
    if stdmath.Abs(rx) > 0.2 || stdmath.Abs(ry) > 0.2 {
        component.Direction.Get(entry).Direction = math.Vec2{X: rx, Y: ry}
    }
}
```

Left stick (axes 0/1) moves; right stick (axes 2/5) writes an aim direction into a
`Direction` component that the casting/crosshair systems consume — input never aims
directly. After the keyboard/gamepad branches, one shared clamp keeps diagonal keyboard
movement from exceeding `moveSpeed` (`delta` is rescaled when both components are set).

**The device on the component** (`component/input.go`):

```go
type InputData struct {
    Disabled  bool
    GamepadID *ebiten.GamepadID // nil = keyboard player
    // ...key bindings, see input-keyboard.md...
}
```

## The multi-device join flow

A character-select scene (`scene/character_select.go`) owns local-multiplayer device
handling:

- Four `PlayerSelect` slots, each `{Joined, Ready bool; Keyboard bool; GamePadID
  *ebiten.GamepadID; ClassIndex; Demo *donburi.Entry}`. The keyboard and every connected
  gamepad are independent candidate devices.
- Each scene `Update` loops `ebiten.AppendGamepadIDs(nil)` and checks **per-pad**
  just-pressed standard buttons: A/Cross joins (or readies-up if already joined),
  B/Circle unreadies → leaves → backs out to the main menu when nobody is joined, D-pad
  left/right cycles classes. Enter/Escape/arrows do the same for the keyboard.
- A device is matched to its slot by comparing values: `keyboard && p.Keyboard` or
  `!keyboard && p.GamePadID != nil && *p.GamePadID == *gamepadID`. First free slot wins
  on join. The `!keyboard` guard matters: `gamepadID` is nil for keyboard events, and
  without it a keyboard Escape with a gamepad player joined dereferences nil.
- When every joined player is ready, the scene hands `[]JoinedPlayer{Keyboard, GamePadID,
  Class}` to the battle scene, which stamps each player's `InputData.GamepadID` at spawn
  (`archetype/player.go`). After that, gameplay code never asks "which
  devices exist" — it reads the component.

```go
// scene/character_select.go — the per-pad poll
for _, id := range ebiten.AppendGamepadIDs(nil) {
    id := id
    if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom) {
        g.onPlayerJoin(false, &id)
    } else if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightRight) {
        g.onPlayerLeave(false, &id)
    }
    // ...D-pad left/right → onChangeClass(false, &id, ∓1)...
}
```

## Rules

- **Always deadzone** — 0.2 on every axis read. Raw axes drift.
- **Prefer `StandardGamepadButton*` / `IsStandardGamepadButtonJustPressed`** over raw
  button indices — the standard layout is controller-agnostic. Raw *axes* 0/1 (left
  stick) are fine.
- **Detect gamepad presence per-frame** (`ebiten.AppendGamepadIDs(nil)` in `Update`),
  never cache the ID list across ticks. Binding a chosen `GamepadID` on `InputData` at
  join time is the exception — that's persistent player data, not presence.
- Keyboard and gamepad live in one input system writing the same outputs; gamepad wins
  when it's past the deadzone, or when the entity has a bound `GamepadID`.
- Normalize/clamp diagonal speed once, after all device branches.

## Pitfalls

- Handle disconnects: a bound `GamepadID` whose pad unplugs just goes silent.
  Detect absence per-frame (rule above) and pause or re-open the join screen.
- Aim the mouse fallback through the camera's screen→world conversion, not the
  entity's `LocalPosition` (see [cameras.md](cameras.md)).
