# Sprite tint & flip

## How it works

`SpriteData` carries three optional override pointers — `nil` means "leave the sprite
alone", so the zero value costs nothing. The [renderer](cameras.md) applies whichever
are set on every draw: `ColorOverride` adds to the RGB channels (additive tint),
`AlphaOverride` scales alpha, `ColorBlendOverride` lerps between grayscale and full
color. Effects are therefore *data*: a system animates a float on a component and the
sprite changes — no draw code involved. `FlipX` mirrors the sprite and `Pivot` decides
which point of the image sits at the entity's position.

## Canonical code

**The overrides** (`component/sprite.go`) — `ColorOverride{R, G, B}`,
`AlphaOverride{A}`, and `ColorBlendOverride{Value}` where the value's range is
documented on the type:

```go
type SpriteData struct {
    Image *ebiten.Image
    Pivot SpritePivot
    ...
    FlipX bool // mirror around the vertical axis (Scale(-1, 1))

    ColorOverride      *ColorOverride
    AlphaOverride      *AlphaOverride
    ColorBlendOverride *ColorBlendOverride
}

type ColorBlendOverride struct {
    // Value is in range [0, 1]
    // 0 is monochrome, 1 is the original color
    Value float64
}
```

**Application order** in the renderer (`system/render.go`, `renderSprite`). The
overrides compose into one matrix from the `ebiten/v2/colorm` package (the old
`DrawImageOptions.ColorM` field is deprecated):

```go
var cm colorm.ColorM
if sprite.ColorOverride != nil {
    cm.Translate(sprite.ColorOverride.R, sprite.ColorOverride.G, sprite.ColorOverride.B, 0)
}
if sprite.ColorBlendOverride != nil {
    cm.Concat(monochromeBlend(sprite.ColorBlendOverride.Value))
}
if sprite.AlphaOverride != nil {
    cm.Scale(1, 1, 1, sprite.AlphaOverride.A)
}
colorm.DrawImage(camera.Viewport, sprite.Image, cm, &colorm.DrawImageOptions{GeoM: op.GeoM})
```

A sprite with no color overrides can skip the matrix and use `ebiten.DrawImage` with
`op.ColorScale` — alpha alone is `op.ColorScale.ScaleAlpha(float32(a))`.

**The monochrome blend** — a luminance matrix lerping grayscale↔color
(`system/render.go`):

```go
// monochromeBlend lerps grayscale (blend 0) to the original color (blend 1).
func monochromeBlend(blend float64) colorm.ColorM {
    blend = min(max(blend, 0), 1)
    // Standard luminance coefficients
    lum := [3]float64{0.299, 0.587, 0.114}

    var cm colorm.ColorM
    for out := 0; out < 3; out++ {
        for in := 0; in < 3; in++ {
            v := lum[in] * (1 - blend)
            if in == out {
                v += blend // color preservation
            }
            cm.SetElement(out, in, v)
        }
    }
    return cm
}
```

`Concat` matters: assigning the blend matrix over the one that already holds the
`ColorOverride` translate silently drops the tint.

**Showcase: proximity colorization.** Points of interest sit desaturated and bloom into
color as the player approaches — spawn the sprite with the pointer already set
(`archetype/poi.go` uses `ColorBlendOverride: &component.ColorBlendOverride{Value: 0}`),
then drive the value each tick (`system/detectpoi.go`):

```go
value := 0.0
if archetype.CanInteractWithPOI(engine.MustGetParent(poi)) {
    value = distanceToBlendValue(distance)
}

component.Sprite.Get(poi).ColorBlendOverride.Value = value
```

**Showcase: selection highlight** — not a tint on the target but a white overlay
entity parented to it, sized from the target's sprite, at low alpha
(`archetype/object.go`):

```go
highlight := New(world).
    WithParent(spriteEntity).            // Parent to the actual sprite entity for proper positioning
    WithSprite(component.SpriteData{
        Image:         highlightSprite,
        Pivot:         targetSprite.Pivot,               // Use same pivot as target sprite
        AlphaOverride: &component.AlphaOverride{A: 0.3}, // Subtle transparency
    }).
```

**Flip drives facing.** The player mirrors when walking left, keeps facing on vertical
steps (`system/player.go`):

```go
// Flip to face left or right; keep the current facing when turning vertically.
if dir.X < 0 {
    component.Sprite.Get(entry).FlipX = true
} else if dir.X > 0 {
    component.Sprite.Get(entry).FlipX = false
}
```

A held item tracks the carrier with the *inverse* flip, because the tool art faces the
opposite way (`system/interact.go`: `component.Sprite.Get(e).FlipX = !facingLeft`).

**Pivots.** `SpritePivotTopLeft` (default), `SpritePivotCenter`,
`SpritePivotBottomCenter`. Bottom-center is the convention for world objects — the
entity position is the feet, which is what the grid and [Y-sort](layers-ysort.md)
both assume. Center is for rotating things; top-left for UI.

## Rules

- Overrides are pointers and `nil` by default. If a system will write the value every
  frame, **create the pointer at spawn** (the `Value: 0` above) — don't make
  hot loops nil-check-and-allocate.
- Tint via the overrides, not by baking recolored images, when the value animates.
  Bake a copy when the effect is constant (see Variations below).
- One entity, one look: compound effects (highlight + tint) are extra child entities,
  not stacked hacks on one sprite.
- World objects get `SpritePivotBottomCenter`; don't mix pivots within one multi-part
  object.

## Pitfalls

- **Name flags after what they do.** A flag called `FlipY` that mirrors horizontally
  (known pitfall #3 in [README.md](README.md)) gets every new call site wrong.
- Don't use the deprecated `DrawImageOptions.ColorM`; use `colorm` for matrices and
  `op.ColorScale` for plain tint and alpha (text: `op.ColorScale.ScaleWithColor(col)`).
- Don't carry override machinery nothing sets; add `ColorBlendOverride` when a game uses
  it.

## Variations

- **Pre-baked tinted copies** instead of per-frame overrides: gray+alpha shadows (a
  `colorm.ColorM` with `Scale(0, 0, 0, 0.4)` + `Translate(0.5, 0.5, 0.5, 0)`,
  `archetype/shadow.go`), a pre-baked white sprite copy flashed for 100ms on hit
  (`archetype/enemy.go newDamageIndicator`), and white silhouettes for an evolution
  effect (`system/evolution.go`). Right choice when the effect is constant per frame —
  one bake, zero per-draw cost.
