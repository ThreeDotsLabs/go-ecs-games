# Widget primitives

## How it works

Three tiny widgets — `ProgressBar`, `Text`, `List` — that **pull** state through closures.
No system knows about health, mana, or cooldowns; each widget entity owns an `Update`
closure that reads whatever it wants and writes the widget's own data. The widget system
then dirty-checks and re-renders only on change. HUDs are just transform subtrees of these
([ui-ecs.md](ui-ecs.md), [transform.md](transform.md)).

## Canonical code

**Pull-closure progress bar** (design: `component/progressbar.go` +
`system/progressbar.go`, modernized):

```go
type ProgressBarData struct {
    MaxValue int
    Value    int

    Width  int
    Height int

    BackgroundColor color.Color

    Update func(bar *ProgressBarData)
}

var ProgressBar = donburi.NewComponentType[ProgressBarData]()
```

Each bar owns its `ebiten.Image` (created `Width`×`Height` in the archetype). The system
calls the closure, then compares before/after — **only a changed bar re-renders**:

```go
func (p *ProgressBar) Update(w donburi.World) {
    p.query.Each(w, func(entry *donburi.Entry) {
        bar := component.ProgressBar.Get(entry)
        if bar.Update == nil {
            return
        }
        value, maxValue, bgColor := bar.Value, bar.MaxValue, bar.BackgroundColor

        bar.Update(bar)

        if value == bar.Value && maxValue == bar.MaxValue && bgColor == bar.BackgroundColor {
            return // nothing changed — keep the baked image
        }

        sprite := component.Sprite.Get(entry)
        sprite.Image.Clear()
        width := float64(bar.Width) * (float64(bar.Value) / float64(bar.MaxValue))
        vector.FillRect(sprite.Image, 0, 0,
            float32(width), float32(bar.Height), bar.BackgroundColor, true)
    })
}
```

The closure may also **recolor** the bar — spell-cooldown bars flip
`bar.BackgroundColor` between ready/prepared/cooling-down inside their own `Update`
(`archetype/player.go`). `component.Text{..., Update func(*TextData)}` is the
same idea for strings; `component.List{Items, Offset}` re-lays-out children on
`Append`/`Remove` (the aura icon strip, `component/list.go`).

**Composition — the HUD is one subtree** (design: `archetype/player.go`,
modernized): every widget hangs off one `uiParent` positioned per player corner, so moving
the HUD = moving one entity, and a 4-player screen is the same code four times.

```go
uiParent := New(w).
    WithPosition(PlayerUIBasePosition(playerID, screenW, screenH)).
    Entry()

healthBar := NewProgressBar(w, math.Vec2{X: 10, Y: 10}, 100, 10, colornames.Red,
    func(bar *component.ProgressBarData) {
        h := component.Health.Get(player)
        bar.Value = h.Health
        bar.MaxValue = h.MaxHealth
    },
)
transform.AppendChild(uiParent, healthBar, false)
```

**Floating damage text** — a one-liner because primitives compose: `Text` + `TimeToLive` +
upward `Velocity` (design: `archetype/ui.go NewDamageText`, modernized):

```go
func NewDamageText(w donburi.World, damage int, pos math.Vec2) *donburi.Entry {
    t := New(w).
        WithPosition(pos).
        WithText(component.TextData{
            Size:  component.TextSizeS,
            Text:  fmt.Sprint(damage),
            Color: colornames.Red,
        }).
        With(component.TimeToLive).
        With(component.Velocity).
        Entry()

    component.TimeToLive.SetValue(t, component.TimeToLiveData{
        Timer: engine.NewTimer(time.Second),
    })
    component.Velocity.SetValue(t, component.VelocityData{
        Velocity: math.Vec2{X: 0, Y: -1},
    })
    return t
}
```

## Rules

- **Widgets pull state via closures** — the widget system never imports gameplay
  components; the closure is written where the widget is created, next to the state it
  reads ([ui-ecs.md](ui-ecs.md) `Script` is the panel-scale version of the same idea).
- **Dirty-check before re-rendering** an owned image; `Clear()`+redraw every frame is the
  bug, not the baseline.
- Compose HUDs under one parent entity; position the parent, not the widgets.
- Ephemeral UI (damage numbers, pickup toasts) = widget + `TimeToLive` + `Velocity`,
  destroyed by the standard lifecycle ([entity-lifecycle.md](entity-lifecycle.md)).
- Nil-guard the closure (as above) in every widget system — a progress-bar system
  without the guard panics on a closure-less bar.

## Gaps

Richer looks worth building on these primitives: a 3-slice progress bar
(`Left/Mid/Right` sprites, the middle tiled to length) and pressed/released button
sprite swapping.
9-slice panels remain a write-fresh staple with no source
([README](README.md)).
