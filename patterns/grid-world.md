# Tile-world grid

## How it works

Every cell of the world is an entity carrying `component.Tile`. A `component.Board`
singleton indexes them: a `[][]*donburi.Entry` for O(1) lookup by position, plus map
dimensions. Tiles are spawned once at map build (generated or loaded from save), parented
into the render hierarchy, and never destroyed; everything that happens to a cell —
terrain, deposits, resources, roads, sight, build permissions — is state on its `TileData`,
mutated in place. Neighbor pointers are wired once after all tiles exist, so adjacency
logic never does index arithmetic.

Use this when cells *have state*. When the grid only answers "who stands where" for a few
movers, the entity-per-cell is the expensive tool — use [grid-occupancy.md](grid-occupancy.md).

## Canonical code

**The board indexes tile entities** (`component/board.go`), with bounds-checked
lookups — `Tiles` is indexed `[x][y]`:

```go
type BoardData struct {
	Size     Size
	TileSize Size

	Tiles [][]*donburi.Entry

	StartPosition Position
}

func (b *BoardData) TileByPosition(pos Position) (*donburi.Entry, bool) {
	if pos.X < 0 || pos.X >= b.Size.Width || pos.Y < 0 || pos.Y >= b.Size.Height {
		return nil, false
	}

	return b.Tiles[pos.X][pos.Y], true
}

func (b *BoardData) MustTileByPosition(pos Position) *donburi.Entry {
	tile, ok := b.TileByPosition(pos)
	if !ok {
		panic("tile not found")
	}
	return tile
}
```

**Per-tile state and neighbor pointers** (`component/tile.go`) — note the derived booleans:

```go
type Position struct {
	X, Y int
}

type NeighborTiles struct {
	N, E, S, W *donburi.Entry
}

type TileData struct {
	Position      Position
	Terrain       domain.Terrain
	Deposit       *domain.Deposit
	HiddenDeposit *domain.Deposit
	Resource      TileResource

	NeighborTiles NeighborTiles

	CanBeHarvested  bool
	InControlRange  bool
	InSightRange    bool
	HasRoad         bool
	HasBuilding     bool
	HasNeighborRoad bool

	CanBuildRoad      bool
	CanBuildBuildings bool
}
```

`NeighborTiles.All()` returns the non-nil neighbors each paired with its `Direction` —
edge tiles simply have fewer neighbors, and call sites never bounds-check.

**Neighbors are wired ONCE**, after every tile entity exists (`system/map.go`):

```go
func markNeighbors(tiles [][]*donburi.Entry) {
	for i := 0; i < len(tiles); i++ {
		for j := 0; j < len(tiles[i]); j++ {
			tile := component.Tile.Get(tiles[i][j])

			if i > 0 {
				tile.NeighborTiles.W = tiles[i-1][j]
			}
			if i < len(tiles)-1 {
				tile.NeighborTiles.E = tiles[i+1][j]
			}
			if j > 0 {
				tile.NeighborTiles.N = tiles[i][j-1]
			}
			if j < len(tiles[i])-1 {
				tile.NeighborTiles.S = tiles[i][j+1]
			}
		}
	}
}
```

`CreateTiles` builds the `[][]*donburi.Entry`, parents each tile into a chunk of the
board's transform subtree (board → chunks → tiles; see [chunks.md](chunks.md)), then runs
`markNeighbors`, `markBorders` and `UpdateAllTilesBuildPermissions`; `LoadTiles` does the
same from a save. Things *on* a tile — roads, borders, flags, labels — are its child entities.

**Range iteration** is a Manhattan diamond, bounds-checked in one place, callback-based
(`system/buildings.go`):

```go
func forTilesInRange(w donburi.World, startX int, startY int, totalRange int, callback func(tile *donburi.Entry)) {
	board := engine.MustFindComponent[component.BoardData](w, component.Board)

	for i := -totalRange; i <= totalRange; i++ {
		effectiveRange := totalRange - engine.Abs(i)

		for j := -effectiveRange; j <= effectiveRange; j++ {
			x := startX + i
			y := startY + j

			if x >= 0 && x < board.Size.Width && y >= 0 && y < board.Size.Height {
				callback(board.Tiles[x][y])
			}
		}
	}
}
```

Settlements mark control and sight ranges through it; roads mark sight ranges.

**Derived state is recomputed on change, not evaluated per query.** "Can I build here" is
a plain boolean on `TileData`; the events that change the world recompute it — targeted
when the change is local, full-board when it is global (`system/buildings.go`):

```go
func UpdateAllTilesBuildPermissions(w donburi.World) {
	donburi.NewQuery(filter.Contains(component.Tile)).Each(w, func(entry *donburi.Entry) {
		updateTileBuildPermissions(w, entry)
	})
}
```

`SpawnRoad` calls `updateTileBuildPermissions` for just the new road's tile and its
neighbors; map creation, loading, the origin settlement and deposit discovery run the
full-board version. Readers — UI, input handlers, AI — only check `tile.CanBuildRoad`.

## Rules

- One entity per cell, created at map build, never destroyed. Cell state mutates in place;
  visuals on a cell are child entities added/removed under the tile.
- All position lookups go through `TileByPosition` (returns ok) or `MustTileByPosition`
  (panics) — never index `Tiles` raw outside the board/map code.
- Wire `NeighborTiles` exactly once, after all tiles exist; from then on adjacency follows
  pointers (`tile.NeighborTiles.All()`), never recomputed indices.
- Derived per-tile state (permissions, ranges) is recomputed eagerly on the events that
  change it — tile + neighbors when local, everything when global.
- Range queries share one `forTilesInRange`; don't hand-roll diamond loops at call sites.
- At 200×200 (40k tile entities) this only works with chunked rendering
  and culling — pair with [chunks.md](chunks.md).

## Pitfalls

- Build queries in system constructors, not inline inside these helpers
  ([ecs-core.md](ecs-core.md)).
- Don't give `TileData` methods that reach into `assets` for sprites (`RoadImage()` and
  friends) — a `component/` → `assets/` dependency the layout
  discourages. Keep sprite *selection* pure (return an ID) and let a system map it to an
  image; never let `domain/` hold `*ebiten.Image` ([project-structure.md](project-structure.md)).
- `Position` on `TileData` belongs in its own component as soon as non-tile entities
  need grid positions.
- `StartPosition` on `BoardData` is generator output ([procgen.md](procgen.md)) riding on
  the board singleton — fine, but keep such fields few; the board is an index, not a grab-bag.
