# Fonts & text

## How it works

The modern stack only: ebiten **text/v2** with `*text.GoTextFace` built from **one embedded
TTF** at three sizes. Faces are package vars in `assets/` and are **rebuilt** when
resolution changes — text is responsive, not baked to a design resolution. Entities render
text via `component.Text` ([ui-ecs.md](ui-ecs.md)); a `Text` system ticks streaming timers;
the renderer draws with `text.Draw`.

## Canonical code

**Loader — one TTF, three faces, rebuildable** (`assets/assets.go`):

```go
var (
    //go:embed fonts/UndeadPixelLight.ttf
    normalFontData []byte

    SmallFont  *text.GoTextFace
    NormalFont *text.GoTextFace
    LargeFont  *text.GoTextFace
)

func MustLoadFonts() {
    UpdateFonts(28)
}

func UpdateFonts(size int) {
    SmallFont = mustLoadFont(normalFontData, int(float64(size)*0.5))
    NormalFont = mustLoadFont(normalFontData, size)
    LargeFont = mustLoadFont(normalFontData, int(float64(size)*2.5))
}

func mustLoadFont(data []byte, size int) *text.GoTextFace {
    s, err := text.NewGoTextFaceSource(bytes.NewReader(data))
    if err != nil {
        panic(err)
    }
    return &text.GoTextFace{
        Source:    s,
        Direction: text.DirectionLeftToRight,
        Size:      float64(size),
        Language:  language.English,
    }
}
```

**Responsive rebuild** — the dimensions system recomputes the base size from screen height
on every layout change (`system/dimensions.go`; deriving it from a dialog row height
instead is the same mechanism):

```go
fontSize := int(float64(screenHeight) * 0.03)
assets.UpdateFonts(fontSize)
```

**The component + size enum** (`component/text.go`):

```go
type TextData struct {
    Text  string
    Color color.Color
    Size  TextSize        // TextSizeM / TextSizeS / TextSizeL

    Align text.Align

    Hidden bool

    Streaming      bool
    StreamingTimer *engine.Timer

    AlphaOverride *AlphaOverride
}
```

**Size → face mapping and measurement** (`archetype/ui.go`):

```go
func FontFromSize(size component.TextSize) *text.GoTextFace {
    font := assets.NormalFont
    switch size {
    case component.TextSizeL:
        font = assets.LargeFont
    case component.TextSizeS:
        font = assets.SmallFont
    }
    return font
}

func MeasureTextWidth(entry *donburi.Entry) float64 {
    txt := component.Text.Get(entry)
    font := FontFromSize(txt.Size)
    w, _ := text.Measure(txt.Text, font, font.Size)
    return w
}
```

`AdjustTextWidth(entry, width)` (same file) greedily word-wraps `TextData.Text` in place by
measuring candidate lines with `text.Measure` — call it once at creation, before measuring
height for layout.

**Typewriter streaming** — a `Text` system ticks the timer; the renderer draws a prefix
(`system/text.go`, `system/render.go`):

```go
runes := []rune(t.Text)
length := len(runes)
if t.Streaming {
    length = int(float64(length) * t.StreamingTimer.PercentDone())
}
textToDraw := string(runes[:length])
```

Slice the **runes**. Counting with `utf8.RuneCountInString(t.Text)` and then slicing
`t.Text[:length]` mixes a rune count with byte offsets: any non-ASCII text cuts mid-rune
and renders garbage (known pitfall #1 in [README.md](README.md)).

## Rules

- One embedded TTF; three sizes via multipliers (`0.5× / 1× / 2.5×` for a pixel-art UI,
  `0.6× / 1× / 1.4×` for a text-heavy one — pick per game, keep the enum at three).
- All text goes through `component.TextData` + `FontFromSize` — no ad-hoc faces in systems.
- Resolution changes call `assets.UpdateFonts`, never scale drawn text with `GeoM` alone.
- Wrap with `AdjustTextWidth` at creation; measure with `text.Measure` (text/v2), never
  guess line heights (`op.LineSpacing = font.Size` is the convention).
- ebitenui shares these faces via `text.Face` vars ([ui-ebitenui.md](ui-ebitenui.md)).

## Pitfalls

- Don't use the deprecated `font.Face` + `opentype.NewFace` + text **v1**
  (`text.Draw(dst, s, face, x, y, clr)`); use text/v2 throughout.
- `font.MeasureString` returns 26.6 fixed-point: dividing by a magic 60
  (`// TODO 60 picked arbitrarily here…`) is wrong; `.Round()` (÷64) is right, and
  text/v2 `text.Measure` makes the question moot.

## Variations

- **Baking static text into images**: draw dialog paragraphs straight into the
  frame's baked image at creation (`archetype/dialog.go NewDialogWithButton`) —
  the right optimization for text that never changes, since it costs one draw at build time
  and nothing per frame. Keep dynamic text as `component.Text` entities.
