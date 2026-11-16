# Occupancy grid

## How it works

One `component.Board` singleton holds the logical tile grid: dimensions, cell size, and an
`Occupied` map from cell to the `Solid`-tagged entity standing on it. There are no
per-cell entities and no physics: walkability is one map lookup
(`IsFree`), and a move is "check the target cell, transfer occupancy, start the glide"
(the glide itself is [movement.md](movement.md)). Only movers and blockers ever appear in
`Occupied`; when every cell needs state of its own, use [grid-world.md](grid-world.md).

## Canonical code

**The board component** (`component/board.go`) — this file *is* the pattern:

```go
// Cell addresses a single tile of the board grid.
type Cell struct {
	X int
	Y int
}

// Add returns the cell offset by d.
func (c Cell) Add(d Cell) Cell {
	return Cell{X: c.X + d.X, Y: c.Y + d.Y}
}

// BoardData is the logical tile grid the world plays out on: its size in
// cells plus which solid entity currently occupies each cell (see the Solid
// tag). Non-solid entities never appear in Occupied.
type BoardData struct {
	Width    int     // in cells
	Height   int     // in cells
	CellSize float64 // in world pixels

	Occupied map[Cell]donburi.Entity
}

var Board = donburi.NewComponentType[BoardData]()

func MustFindBoard(w donburi.World) *BoardData {
	return engine.MustFindComponent[BoardData](w, Board)
}

// WorldToCell returns the cell containing pos.
func (b *BoardData) WorldToCell(pos math.Vec2) Cell {
	return Cell{
		X: int(stdmath.Floor(pos.X / b.CellSize)),
		Y: int(stdmath.Floor(pos.Y / b.CellSize)),
	}
}

// CellToWorld returns the world position of the cell's top-left corner.
func (b *BoardData) CellToWorld(c Cell) math.Vec2 {
	return math.Vec2{X: float64(c.X) * b.CellSize, Y: float64(c.Y) * b.CellSize}
}

// InBounds reports whether c lies on the board.
func (b *BoardData) InBounds(c Cell) bool {
	return c.X >= 0 && c.Y >= 0 && c.X < b.Width && c.Y < b.Height
}

// IsFree reports whether a solid entity may enter c.
func (b *BoardData) IsFree(c Cell) bool {
	if !b.InBounds(c) {
		return false
	}
	_, occupied := b.Occupied[c]
	return !occupied
}

// Claim marks c as occupied by e. Claiming an already-taken cell panics —
// it means the map spawns two solids on the same tile, which would otherwise
// be an invisible authoring error.
func (b *BoardData) Claim(c Cell, e donburi.Entity) {
	if other, taken := b.Occupied[c]; taken && other != e {
		panic(fmt.Sprintf("cell %v claimed twice", c))
	}
	b.Occupied[c] = e
}

// Move transfers e's occupancy between cells. The vacated cell frees up as
// soon as the move starts, so another solid may enter it while e is still
// gliding toward its target.
func (b *BoardData) Move(from, to Cell, e donburi.Entity) {
	delete(b.Occupied, from)
	b.Occupied[to] = e
}
```

**The Solid tag** carries the invariant as its doc comment (`component/solid.go`):

```go
// Solid marks an entity that occupies its cell on the Board: at most one
// solid entity can be on a cell at a time. Whatever spawns or moves a solid
// entity is responsible for keeping BoardData.Occupied in sync.
var Solid = donburi.NewTag()
```

**Claiming at spawn** — whoever creates a solid entity claims its cell (`scene/game.go`):

```go
// solid places a tile that blocks movement and claims its board cell.
func solid(w donburi.World, root *donburi.Entry, board *component.BoardData, img *ebiten.Image, col, row int) *donburi.Entry {
	e := tile(w, root, img, col, row).
		With(component.Solid).
		Entry()
	board.Claim(component.Cell{X: col, Y: row}, e.Entity())
	return e
}
```

**Movement** — a step is claim-then-glide (`system/player.go`):

```go
		t := transform.GetTransform(entry)
		from := board.WorldToCell(t.LocalPosition)
		to := from.Add(player.Facing)
		if !board.IsFree(to) {
			return
		}

		board.Move(from, to, entry.Entity())
		movable.TargetPosition = board.CellToWorld(to)
		movable.Moving = true
```

The vacated cell frees the moment the move **starts** — another solid may enter it while
the sprite is still gliding; `system.Movable` never consults the board mid-glide.

**Logical position while gliding** — logic must never see an entity "between" cells, so cell lookups return the *claimed* cell (`system/movable.go`):

```go
// entityCell returns the cell a movable entity logically occupies — the cell
// it has claimed on the board, which is the glide target while moving.
func entityCell(board *component.BoardData, entry *donburi.Entry) component.Cell {
	if entry.HasComponent(component.Movable) {
		movable := component.Movable.Get(entry)
		if movable.Moving {
			return board.WorldToCell(movable.TargetPosition)
		}
	}
	return board.WorldToCell(transform.GetTransform(entry).LocalPosition)
}
```

`system/interact.go` and `system/selectedtile.go` both target through it — `entityCell(board, player).Add(component.Player.Get(player).Facing)`.

## Rules

- The board is a required singleton in grid games — created in `createWorld()` with
  `Occupied` initialized (a nil map panics on the first `Claim`), fetched via
  `component.MustFindBoard(w)` ([singletons.md](singletons.md)).
- `Occupied` tracks solids only; walkable decorations never touch it.
- Whatever spawns or moves a solid keeps `Occupied` in sync — spawn helpers `Claim`,
  movement code calls `Move`. Nothing ever scans the world to rebuild the map.
- `Claim` panics on double-claim on purpose — keep it; invisible authoring errors become loud.
- All conversion goes through `WorldToCell` (floor-divide) / `CellToWorld` (top-left
  corner); no system does its own division. Multi-cell objects claim every cell they
  block (two-tile trees, so the player can't stand "inside" the canopy).
- Any "what cell is this entity on" question goes through `entityCell`.

## Pitfalls

- The code above never despawns a solid at runtime. The moment you do (chopping a tree), the
  despawn path must free the cell — add a `Release` counterpart to `Claim` on `BoardData`
  rather than poking `Occupied` from systems.
- `entityCell` is unexported in `system/movable.go`; promote it to a `BoardData` method when needed elsewhere.

## Not this: the layout grid

`component/grid.go` shares the word "grid" and nothing else: `GridData{MaxCols}`
+ the `GridCell` tag + `UpdateGridPositions` lay out a container's children in rows
(`col := i % MaxCols`, `row := i / MaxCols`), routing position changes through `Movable`
so reflow animates — a **UI row/column layout** knowing nothing of world cells or
occupancy. It suits card hands and score boards. Keep the names apart: *board*
(occupancy) vs *grid* (layout).
