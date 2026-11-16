# Chunks & culling

An **opt-in module**: adopt it when the world dwarfs the viewport (the source map is
200×200 tiles); skip it for single-screen games.

## How it works

Chunks are plain entities inserted between the board root and its tiles, forming a
spatial index out of the [transform hierarchy](transform.md) itself. Each chunk knows
its world rect; every tile is a *child of its chunk* at a chunk-local position. Any
system that would otherwise touch every tile — rendering, cursor picking — first asks
"which chunks intersect the camera rect?" and only recurses into those. With
`ChunkSize` 50 and 32px tiles, a screen overlaps a handful of chunks, so per-frame
work tracks the viewport, not the world.

## Canonical code

**Chunk grid** — sized up front from the board dimensions, parented to the board
(`system/map.go`):

```go
const (
    TileSize  = 32
    ChunkSize = 50
)

func GenerateChunks(w donburi.World, parent *donburi.Entry, width int, height int) [][]*donburi.Entry {
    chunksWidth := int(stdmath.Ceil(float64(width) / float64(ChunkSize)))
    chunksHeight := int(stdmath.Ceil(float64(height) / float64(ChunkSize)))

    chunks := make([][]*donburi.Entry, chunksWidth)
    for i := 0; i < chunksWidth; i++ {
        chunks[i] = make([]*donburi.Entry, chunksHeight)
        for j := 0; j < chunksHeight; j++ {
            pos := math.Vec2{
                X: float64(i * ChunkSize * TileSize),
                Y: float64(j * ChunkSize * TileSize),
            }
            chunk := newChunk(w, pos)

            transform.AppendChild(parent, chunk, false)

            chunks[i][j] = chunk
        }
    }

    return chunks
}
```

Each chunk carries its own size (`ChunkData.Size` = `ChunkSize * TileSize` square), so
its world rect is `WorldPosition` + `Size`.

**Tiles live at chunk-local positions** — the modulo does the bookkeeping; the chunk's
transform supplies the rest (`system/map.go`, `newTile` / `CreateTiles`):

```go
tile := archetype.New(w).
    WithPosition(math.Vec2{
        X: float64(x % ChunkSize * TileSize),
        Y: float64(y % ChunkSize * TileSize),
    }).
    ...

chunkX := i / ChunkSize
chunkY := j / ChunkSize
chunk := chunks[chunkX][chunkY]

transform.AppendChild(chunk, tile, false)
```

**Render culling** — intersect each chunk's rect with the camera rect; only matching
chunks get their subtrees collected (`system/render.go`):

```go
r.chunksQuery.Each(w, func(entry *donburi.Entry) {
    chunk := component.Chunk.Get(entry)
    pos := transform.WorldPosition(entry)
    rect := engine.NewRect(pos.X, pos.Y, float64(chunk.Size.Width), float64(chunk.Size.Height))

    if !rect.Intersects(cameraRect) {
        return
    }

    chunks++

    for _, child := range findChildrenWithComponent(entry, component.Sprite, component.SpriteLayerInherit) {
        ...
    }
})
```

A second, per-entity cull inside the draw skips sprites that sit in a visible chunk
but off screen. The debug overlay prints `Rendered chunks: N` next to the entity
counts — keep that; it's how you see the culling work (see [debug.md](debug.md)).

**The same narrowing serves hit-testing** — cursor picking on a 200×200 board stays
cheap because only on-screen chunks are searched for tiles
(`system/controls.go`):

```go
c.chunksQuery.Each(w, func(entry *donburi.Entry) {
    if found {
        return
    }

    chunk := component.Chunk.Get(entry)
    pos := transform.WorldPosition(entry)
    rect := engine.NewRect(pos.X, pos.Y, float64(chunk.Size.Width), float64(chunk.Size.Height))

    if !rect.Intersects(cameraRect) {
        return
    }

    tiles := engine.FindChildrenWithComponent(entry, component.Tile)
    for _, tileEntry := range tiles {
        ...
```

## Rules

- Adopt chunks **only when the world is much larger than the screen**. A scene that
  roughly fits the viewport gains nothing but bookkeeping.
- Chunks are pure structure: no sprite, no game data — just `Chunk` + transform. Game
  logic keeps addressing tiles through the board's 2D slice; chunks matter only to
  systems that iterate spatially.
- Everything spatially culled must live under a chunk. An entity parented straight to
  the board skips culling silently.
- Static worlds only — tiles never move between chunks. If entities roam,
  a chunk hierarchy is the wrong tool (that's the spatial-hash staple in
  [README.md](README.md), design fresh).
- Show the rendered-chunk count in the debug overlay from day one.

## Pitfalls

- Put the culling in the renderer as a filter on the camera Root's traversal (chunk
  query → rect intersect → recurse; see [cameras.md](cameras.md),
  [layers-ysort.md](layers-ysort.md)), not in a separate render path.
- Build the camera rect once, as a `CameraData` method (`ViewportPosition` + viewport
  size / zoom); rebuilding it by hand in render and in controls drifts apart.
- `ChunkSize` 50 suits 32px tiles on a 200×200 board; retune per game — the target is
  "a few chunks visible at once", not any magic number.
