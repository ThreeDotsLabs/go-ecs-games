# Spritesheet tile catalog

## How it works

One packed spritesheet is embedded and decoded **once**; every named tile is a `SubImage`
view into that single texture — one GPU upload, zero per-tile decodes, and all draws from
the sheet batch on one texture. On top of the raw slicer sits a **semantic catalog**: small
structs grouping related tiles by meaning (a crop's whole lifecycle, a soil strip's caps,
the barn's wall courses), each built from sheet coordinates in exactly one place
(`assets/tiles.go`). Game code grabs tiles by name — `assets.Carrot.Ripe`,
`assets.Barn.DoorBig` — never by `(col, row)` index.

## Canonical code

**Sheet + slicer** — embed once, decode once, slice on demand (`assets/tiles.go`):

```go
// The Kenney "Tiny Farm" pack, packed into a single sheet: a 12x11 grid of
// 16x16 tiles with no spacing. Every named tile below is a SubImage view into
// this one texture, so there is a single GPU upload and no per-tile decoding.
//
//go:embed kenney_tiny-farm/Tilemap/tilemap_packed.png
var tilemapData []byte

var tilemap = mustLoadImage(tilemapData)

const TileSize = 16

func tile(col, row int) *ebiten.Image {
    x, y := col*TileSize, row*TileSize
    return tilemap.SubImage(image.Rect(x, y, x+TileSize, y+TileSize)).(*ebiten.Image)
}
```

**Semantic groups** — a struct per shape, a builder that encodes the sheet's layout, named
vars as the only public surface (`assets/tiles.go`):

```go
// Crop holds the sprites for one crop across its whole lifecycle, from planting
// to the goods it yields.
type Crop struct {
    Seedling *ebiten.Image // just sprouted
    Growing  *ebiten.Image // half grown
    Ripe     *ebiten.Image // ready to harvest
    Withered *ebiten.Image // dried up, missed harvest
    Produce  *ebiten.Image // the harvested item
    SeedBag  *ebiten.Image // large sack of seeds
    SeedPack *ebiten.Image // small packet of seeds
    Crate    *ebiten.Image // crate of harvested produce
}

// crop builds a Crop from its row in the sheet. Every crop occupies the same
// eight columns (4..11), so only the row differs.
func crop(row int) Crop {
    return Crop{
        Seedling: tile(4, row),
        Growing:  tile(5, row),
        Ripe:     tile(6, row),
        Withered: tile(7, row),
        Produce:  tile(8, row),
        SeedBag:  tile(9, row),
        SeedPack: tile(10, row),
        Crate:    tile(11, row),
    }
}

var (
    Carrot = crop(0)
    Wheat  = crop(5)
    // … Radish, Corn, Tomato, Cabbage
)

var (
    PineTree = TwoTile{Top: tile(3, 0), Bottom: tile(3, 1)} // large evergreen
    Bush     = tile(3, 3)
    Cow      = tile(1, 10)
)
```

The other shapes follow the same pattern: `SoilStrip` (`Single/First/Middle/Last` caps for
tilled-plot runs), `TwoTile` (`Top/Bottom` for tall trees), `Trough` (empty/full × left/
right), `WallRow` (`Left/Middle/Right`), and `Barn` — a single anonymous-struct var naming
every course of the barn's front. A new *shape* of grouping = a new small struct + builder
next to `Crop`; no new mechanism.

**Gotchas**: `SubImage` returns
`image.Image`, so the `.(*ebiten.Image)` assertion is required; and `tile()` doesn't
bounds-check — an out-of-grid `(col, row)` silently yields a garbage (empty/clipped) view,
not an error. Find coordinates in the pack's `Tilesheet.txt` before adding a var.

## Rules

- **Grab tiles by name, never by index.** `tile(col, row)` is package-private to `assets/`;
  sheet coordinates appear only in `assets/tiles.go`.
- One decode per sheet. Tiles are `SubImage` views — never copy a tile into a fresh
  `ebiten.NewImage` (that forfeits batching and costs memory).
- Adding a tile = one named var; adding a family = one struct + builder. Callers never
  learn the sheet layout.
- Doc-comment what a tile *is* (`// large, leafless`) — the name plus comment is the
  catalog's documentation; there is no other index.
- The catalog exposes only `*ebiten.Image` and tiny grouping structs, so sprites slot
  straight into `SpriteData` like any other art (see [assets.md](assets.md),
  [placeholder-art.md](placeholder-art.md)).

## Pitfalls

- Never decode the tileset PNG **inside the tile loop** (`fs.ReadFile` + decode per
  iteration) — decode once above the loop and cache the `*ebiten.Image` per tileset.
- Consider a bounds panic in `tile()` — the silent-garbage failure mode has no error path.

## Variations

**Tiles addressed by Tiled tileset class.** Instead of Go-side coordinates,
each tile is given a **class name in the Tiled tileset editor** (`.tsx`), and the loader
resolves class → `SubImage` rect at load, correcting for the sheet's 1px spacing. Art
naming lives in Tiled, not in Go consts — re-cutting the sheet never touches Go
(`assets/assets.go`, trimmed):

```go
func (l *levelLoader) MustFindTile(tilesetClass string, tileClass string) *ebiten.Image {
    ts, ok := l.Tilesets[tilesetClass] // tilesets collected while loading TMX levels
    // …
    for _, t := range ts.Tiles {
        f, err := fs.ReadFile(assetsFS, filepath.Join("levels", ts.Image.Source))
        // err check trimmed
        tilesetImage := mustNewEbitenImage(f) // wart: re-decoded every iteration

        if t.Class == tileClass {
            width := ts.TileWidth
            height := ts.TileHeight

            col := int(t.ID) % ts.Columns
            row := int(t.ID) / ts.Columns

            // Plus one because of 1px margin
            if col > 0 {
                width += 1
            }
            if row > 0 {
                height += 1
            }

            sx := col * width
            sy := row * height

            return tilesetImage.SubImage(
                image.Rect(sx, sy, sx+ts.TileWidth, sy+ts.TileHeight),
            ).(*ebiten.Image)
        }
    }
    panic(fmt.Sprintf("tile not found: %s", tileClass))
}

// usage:
TankBase = loader.MustFindTile(TilesetClassTiles, "tank-base")
```

Use this flavor when tilesets are already maintained in Tiled for TMX levels
([tiled.md](tiled.md)); use the coordinate catalog above when the sheet is a standalone
art pack.

**Paper-doll character assembly.** The same class-addressed idea, one level
richer: the character tileset's tiles carry a **class** (`body`, `hair`, `facial_hair`,
`armor`, `weapon`) and armor/weapons add a **`slot` custom property** (`head`, `chest`,
`legs`, `feet`, `main_hand`, `off_hand`). `MustLoadAssets` decodes the sheet once, slices
each tile with the same 1px-margin math, and buckets the parts into lists
(`Bodies`, `HeadArmors`, `MainHandWeapons`, …) of `BodyPart{ID, Image}`
(`assets/assets.go`). Character assembly is then "pick one part per slot and
stack the sprites" — the catalog's structure mirrors the doll's slots, and new art appears
in the fitting room by tagging tiles in Tiled.
