# Placeholder art

## How it works

Ship gameplay before art. Every "sprite" a game draws can be **generated at init in
`assets/`** — `ebiten.NewImage` + `Fill` and `vector` primitives, exposed as package-level
`*ebiten.Image` vars named for what they represent. The rest of the game cannot tell:
`component.SpriteData` holds a `*ebiten.Image` and does not care where it came from, so
real art later replaces a generator without touching a single system or archetype
(see [assets.md](assets.md) for the loaded-art pipeline this slots into).

## Canonical code

**Generators + named vars** — one tiny constructor per shape, colors from
`golang.org/x/image/colornames` (`assets/assets.go`, trimmed):

```go
var (
    GoldTileImage  = newTileImage(colornames.Gold)
    BlueTileImage  = newTileImage(colornames.Blue)
    DiscSlotImage  = newDiscImage(colornames.Darkgrey)
    WhiteDiscImage = newDiscImage(colornames.White)
    ButtonImage    = newButtonImage(colornames.White)
    // …
)

func newDiscImage(color color.Color) *ebiten.Image {
    img := ebiten.NewImage(40, 40)
    vector.FillCircle(img, 20, 20, 18, color, true)
    return img
}

func newTileImage(color color.Color) *ebiten.Image {
    img := ebiten.NewImage(40, 40)
    img.Fill(color)
    return img
}
```

**Tintable outline** — a tile cursor built from `SubImage` fills and kept
plain white so scenes recolor it via sprite overrides
(`assets/ui.go`):

```go
var TileCursor = makeTileCursor()

func makeTileCursor() *ebiten.Image {
    img := ebiten.NewImage(TileSize, TileSize)
    const b = 1 // border thickness in source pixels; scales up with the tile
    for _, r := range []image.Rectangle{
        image.Rect(0, 0, TileSize, b),                   // top
        image.Rect(0, TileSize-b, TileSize, TileSize),   // bottom
        image.Rect(0, b, b, TileSize-b),                 // left
        image.Rect(TileSize-b, b, TileSize, TileSize-b), // right
    } {
        img.SubImage(r).(*ebiten.Image).Fill(color.White)
    }
    return img
}
```

**Draw-then-upscale** — in a pixel-art game, code-drawn placeholders must match the art's
scale factor: draw at base pixel size and upscale by `domain.SpriteScale` with
nearest-neighbor, exactly like the asset loader does
(`assets/generated.go`):

```go
// CreateScaledSprite draws a base-size sprite and scales it by SpriteScale for
// pixel-perfect rendering.
func CreateScaledSprite(width, height int, drawFunc func(*ebiten.Image)) *ebiten.Image {
    baseImage := ebiten.NewImage(width, height)
    drawFunc(baseImage)

    scaledImage := ebiten.NewImage(width*domain.SpriteScale, height*domain.SpriteScale)
    opts := &ebiten.DrawImageOptions{}
    opts.GeoM.Scale(float64(domain.SpriteScale), float64(domain.SpriteScale))
    opts.Filter = ebiten.FilterNearest
    scaledImage.DrawImage(baseImage, opts)

    return scaledImage
}
```

**The one-liner** — eight spell projectiles shipped (and a quicksand area) as
filled rectangles: `SparkProjectile = ebiten.NewImage(5, 2)` +
`SparkProjectile.Fill(colornames.Lightyellow)` (`assets/assets.go`).

## Rules

- Placeholder generators live in **`assets/`**, return `*ebiten.Image`, and are named for
  the thing, not the shape (`WhiteDiscImage`, not `Circle40`): swapping in real art means
  changing one var's initializer, nothing else.
- Systems and archetypes must never know an image is a placeholder — the `SpriteData`
  contract is the same for generated and loaded art.
- Generate white/neutral where scenes tint (the `TileCursor`); pick distinct
  `colornames` per game concept so gameplay reads at a glance ([colors.md](colors.md)).
- Generate at the size gameplay needs — final dimensions, or base pixel size upscaled by
  `SpriteScale` — so colliders and layout don't shift when real art lands.
- Don't gate features on art: block out the entity with a generator, tune the gameplay,
  replace the image last ([tile-catalog.md](tile-catalog.md) is often the cheap upgrade).

## Pitfalls

- `CreateScaledSprite` is an asset helper: keep it in `assets/` beside the other
  generators, not in `archetype/entry.go`.

## Variations

- **Everything generated** (discs, tiles, ribbons, buttons, slots, overlays) — a whole
  playable board game with no image files at all.
- **Mixed**: real tiles from a packed sheet, generated UI (`TileCursor`); or drawn
  fireball/lightning sprites beside `Fill()` rectangles for eight other spells —
  placeholders and finals coexisting in one `assets.go` is fine.
