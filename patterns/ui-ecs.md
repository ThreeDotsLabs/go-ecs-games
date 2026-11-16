# ECS-native game UI

## How it works

In-game UI is **plain entities**: a frame sprite baked once into an image, a `Collider`
sized to it, buttons and text as transform children. Every UI entity lives under a
dedicated UI camera's root (`archetype.NewCamera(w, pos, size, 1, ui)` over a UI root
entity — `scene/game.go`), children layered with
`WithLayerInherit` — see [cameras.md](cameras.md) and [layers-ysort.md](layers-ysort.md).
Whole subtrees toggle via `component.Active`; menus and forms are *not* this pattern
([ui-ebitenui.md](ui-ebitenui.md)).

## Canonical code

**Frame — draw once, attach Sprite+Collider** (`archetype/dialog.go`, trimmed):

```go
func NewFrameWithBorder(w donburi.World, pos math.Vec2, width int, height int) *donburi.Entry {
    totalWidth := width + frameBorderWidth*2
    totalHeight := height + frameBorderWidth*2

    image := ebiten.NewImage(totalWidth, totalHeight)
    image.Fill(assets.BorderColor)
    vector.FillRect(image,
        float32(frameBorderWidth), float32(frameBorderWidth),
        float32(totalWidth-frameBorderWidth*2), float32(totalHeight-frameBorderWidth*2),
        assets.PanelColor, true)

    frame := New(w).  // newFrame, inlined
        WithPosition(pos).
        WithLayer(component.SpriteUILayerDialog).
        WithSprite(component.SpriteData{Image: image}).
        With(component.Collider).
        With(component.UI).
        Entry()
    component.Collider.SetValue(frame, component.ColliderData{
        Width: float64(totalWidth), Height: float64(totalHeight),
    })
    return frame
}
```

**Buttons** — `ButtonData{OnClick, Sound, KeepSelection}`; every `OnClick` wrapper first
publishes `ButtonClicked` so audio hooks it centrally ([audio.md](audio.md)) (`archetype/ui.go`):

```go
component.Button.SetValue(button, component.ButtonData{
    OnClick: func(w donburi.World, e *donburi.Entry) {
        events.ButtonClickedEvent.Publish(w, events.ButtonClicked{
            Button: component.Button.Get(e),
        })
        onClick(w, e)
    },
    Sound: component.ButtonSoundNone,
})
```

**Click dispatch** — topmost panel under the cursor wins, then the hit button child fires
(`system/controls.go`; `uiPanelsQuery` is `filter.Or(Contains(UIPanel), Contains(Dialog))`):

```go
func (c *Controls) getUIPanelUnderCursor(w donburi.World, x int, y int) *donburi.Entry {
    uiPanelsByLayer := make(map[int][]*donburi.Entry)
    c.uiPanelsQuery.Each(w, func(entry *donburi.Entry) {
        l := int(component.Layer.Get(entry).Layer)
        uiPanelsByLayer[l] = append(uiPanelsByLayer[l], entry)
    })
    cursor := math.Vec2{X: float64(x), Y: float64(y)}
    // skipped: collect layer keys into layers, sort.Sort(sort.Reverse(...)) — top first
    for _, i := range layers {
        for _, entry := range uiPanelsByLayer[i] {
            // skipped: inactive panels (component.Active)
            collider := component.Collider.Get(entry)
            if collider.Rect(entry).Contains(cursor) {
                return entry
            }
        }
    }
    return nil
}
```

```go
if uiPanelUnderCursor != nil && clicked {
    buttons := engine.FindChildrenWithComponent(uiPanelUnderCursor, component.Button)
    for _, entry := range buttons {
        // skipped: inactive buttons; then:
        if component.Collider.Get(entry).Rect(entry).Contains(cursor) {
            button := component.Button.Get(entry)
            button.OnClick(w, entry)
            if !button.KeepSelection {
                c.unselect(w)
            }
            break
        }
    }
}
```

A panel under the cursor **swallows** the click even when no button is hit — world
selection runs only when `uiPanelUnderCursor == nil`.

**Panels** — Show = create the entity tree, Hide = destroy it; live values update through a
`component.Script` closure capturing what it needs (`archetype/resource_panel.go`):

```go
updateFunc := func(e *donburi.Entry) {
    component.Text.Get(amountText).Text = fmt.Sprintf("Amount: %v", tile.Resource.Amount)
}
component.Script.Get(frame).Update = updateFunc
```

Callers poke it on relevant events: `component.Script.Get(panel).Update(panel)`
(`system/ui.go`). `NewDialogWithButton` measures the text, sizes the frame, and
bakes the paragraphs into the frame image ([fonts.md](fonts.md)); `NewTable` bakes a grid
and exposes `Rows [][]*donburi.Entry` cell anchors; `ShowMessage` = portrait + streaming
`TextData` + `{player}` substitution (`archetype/message.go`).

**Scrolling log panel** — the dialog is an entity with a **nested camera** over
a `StackedView{CurrentY}` log; new paragraphs append at `CurrentY`, then the log camera
tweens down; a baked alpha-gradient mask fades both edges
(`archetype/dialog.go`, `system/render.go`):

```go
logCam := component.Camera.Get(logCamera)
logCam.Mask = CreateScrollMask(w, dialogWidth, logCameraHeight)

// in the renderer, after drawing the camera's children:
if camera.Mask != nil {
    op := &ebiten.DrawImageOptions{}
    op.Blend = ebiten.BlendDestinationIn
    camera.Viewport.DrawImage(camera.Mask, op)
}
```

Options are sprite+collider entities; the highlight is a single `ActiveOptionIndicator`
entity re-parented onto the active option with
`transform.ChangeParent(indicator, entry, false)`
(`system/controls.go`). Long text wraps with
`AdjustTextWidth(entry, width)` before measuring (`archetype/ui.go`).

**Hiding subtrees** — `component.Active` is ancestor-aware in the renderer: one flag on the
panel root hides the whole tree (`system/render.go` `isActive` walks
`transform.GetParent` to the root).

## Rules

- Bake static chrome (frames, borders, tables) into one image at creation; only dynamic
  text/values get their own entities.
- Every clickable UI entity: `Collider` + `Button` (or `UIPanel`/`Dialog` on the
  container). The dispatch is data-driven — no per-panel input code.
- All `OnClick`s publish `ButtonClicked`; gameplay reactions go through events too
  ([events.md](events.md)).
- Toggle visibility with `component.Active` on the subtree root, never by destroying
  sprites or flipping `Hidden` on each child.
- UI roots are found by **tag/component** (`component.Dialog`, `component.BuildPanel`),
  never by name string.

## Pitfalls

- **Update panels in place via their `Script` closure** instead of closing and reopening
  them — destroy+recreate (`// TODO Probably a hack - close and open the panel instead
  of updating`) resets scroll and hover state and churns entities.
- Don't find the UI root by name (`name.Name == "UI"`); give it a tag component.
- Build queries once per system, not ad hoc inside panel helpers (`HideBuildPanel`) —
  [ecs-core.md](ecs-core.md).
- Measure text with text/v2, not the deprecated `font.MeasureString` and a magic `/60`
  ([fonts.md](fonts.md)).

## Variations

- A `component.UI` tag with a separate offscreen sorted by `SpriteUILayer*` also works,
  but duplicates what the camera root gives for free — keep the camera root.
- Pull-closure widget primitives ([ui-widgets.md](ui-widgets.md)) cover bars and
  labels; dialog content can come from Twine ([dialogue.md](dialogue.md)).
