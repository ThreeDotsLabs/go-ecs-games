# Menus (ebitenui)

## How it works

ebitenui is for **screens**: menus, lobbies, forms, chat, modal dialogs — anything made of
containers, text inputs, and scrollareas. In-world UI (health bars, panels attached to
entities, dialog boxes in the scene) stays ECS — see [ui-ecs.md](ui-ecs.md).

Two integration shapes:

1. **A whole scene is UI** — `scene/menu.go` holds an `*ebitenui.UI` directly; `Update()` /
   `Draw()` just delegate to it. State changes (main ↔ online) rebuild the UI tree by
   reassigning `m.ui`. Widget callbacks set `m.nextScene`; the game loop polls `NextScene()`.
2. **UI overlays a running world** — `system/ui.go` wraps `*ebitenui.UI` in a normal ECS
   system registered in **both** slices (it updates and draws). World → UI flows through
   events subscribed in `Init(w)`; UI → world flows through callbacks handed to the
   constructor. ebitenui widgets never query or mutate the world directly.

## Canonical code

**The UI system, registered in both slices** (`system/ui.go`, `scene/game.go`):

```go
func (u *UISystem) Init(w donburi.World) {
    component.EventGameEnded.Subscribe(w, u.onGameEnded)
    component.EventWinnerPicked.Subscribe(w, u.onWinnerPicked)
}

func (u *UISystem) Update(w donburi.World) {
    if u.ui != nil {
        u.ui.Update()
    }
}

func (u *UISystem) Draw(w donburi.World, screen *ebiten.Image) {
    if u.ui != nil {
        u.ui.Draw(screen)
    }
}
```

```go
g.systems = []System{
    // ... gameplay systems ...
    system.NewDestroy(),
    g.uiSystem,
}

g.drawables = []Drawable{
    system.NewRender(),
    g.uiSystem, // after Render, so UI draws on top of the world
}
```

**Modal dialog** — build a `widget.NewWindow` with `Modal()`, add it, keep the close func
(`system/ui.go`, trimmed):

```go
func (u *UISystem) addMenuDialog(ui *ebitenui.UI) {
    u.closeCurrentDialog()

    overlayContainer := widget.NewContainer(
        widget.ContainerOpts.BackgroundImage(image.NewNineSliceColor(color.NRGBA{0, 0, 0, 100})),
        widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
    )

    dialogContent := widget.NewContainer( /* centered AnchorLayout, RowLayout content */ )
    dialogContent.AddChild(title)
    dialogContent.AddChild(returnBtn)
    dialogContent.AddChild(exitBtn)
    overlayContainer.AddChild(dialogContent)

    window := widget.NewWindow(
        widget.WindowOpts.Contents(overlayContainer),
        widget.WindowOpts.Modal(),
        widget.WindowOpts.Location(image2.Rect(0, 0, 1024, 900)),
    )

    u.closeCurrentDialogFunc = ui.AddWindow(window)
}

func (u *UISystem) closeCurrentDialog() {
    if u.closeCurrentDialogFunc != nil {
        u.closeCurrentDialogFunc()
        u.closeCurrentDialogFunc = nil
    }
}
```

`ui.AddWindow` returns the function that removes the window — that *is* the dialog handle.
Copy-pasting this construction per dialog (Connecting, Error, Menu, Game Over) ends in
`// TODO This pattern is a mess, refactor`: **extract a small
`showDialog(ui, title, content...) (close func())` helper before the second copy**.

**Stop clicks falling through to the world** — mirror ebitenui's package-level hover flag
into the game singleton, gate world input on it (`system/debug.go`,
`system/controls.go`):

```go
import "github.com/ebitenui/ebitenui/input"

game.Debug.UIHovered = input.UIHovered   // in the system that updates the ebitenui.UI
```

```go
if !game.Debug.Enabled || !game.Debug.UIHovered {
    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
        justClicked = true
        // ... handle world click
    }
}
```

Both games only gate while the (debug) UI is active; a permanent HUD needs the flag copied
and checked unconditionally.

## Rules

- ebitenui for screens; ECS entities for in-world UI ([ui-ecs.md](ui-ecs.md)). Don't mix.
- The boundary: widgets call scene methods / client methods / callbacks; world state
  reaches the UI only through events subscribed in `Init(w)`. No `donburi` calls inside
  widget handlers beyond publishing/reacting to events.
- One `currentDialog func()` slot; every `add*Dialog` starts with `closeCurrentDialog()`.
- Rebuilding the whole container tree on state change (`m.ui = m.createOnlineUI()`) is the
  normal move; update widgets in place (`text.Label = ...`, `container.RemoveChildren()`)
  only for high-frequency data like the rooms table and player panel.
- ebitenui wants `text.Face` (an interface): assign the `*text.GoTextFace`s to
  `text.Face` vars (`assets.NormalUIFont`) and passes pointers — reuse the same faces as
  world text ([fonts.md](fonts.md)).
- The UI system goes last in `g.systems` and after `Render` in `g.drawables`.

## Pitfalls

- **Dialog `Location` rects are hardcoded** to `image2.Rect(0, 0, 1024, 900)` (the window
  size) in every dialog. Derive from the game's screen dimensions singleton instead.
- **Version drift**: widget option APIs changed between ebitenui v0.6 and v0.7; pin one
  version per game (the newest) and don't mix examples written against older ones.
- Wire the `UIHovered` bridge on day one if the world takes clicks. Relying on dialogs
  being `Modal()` leaves non-modal HUD buttons falling through to world hit-testing.

## Variations

- **Debug overlay only**: `component.DebugUI{UI *ebitenui.UI}`
  on an entity, built in `scene/game_debug.go`, updated/drawn by the debug system —
  gameplay UI is pure ECS.
- **A menu scene with no world at all** — `Update()`/`Draw()` delegate
  straight to `m.ui`, `HandleMessage` mutates UI state from network messages
  ([networking.md](networking.md)).
- Hand-rolled menus as ECS buttons ([ui-ecs.md](ui-ecs.md)) are more code for less widget
  behavior — use ebitenui for menus.
