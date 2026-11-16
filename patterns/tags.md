# Tags

## How it works

A tag is a data-less component created with `donburi.NewTag()`. It marks an entity for
queries and `HasComponent` checks — "this entity is solid", "this is the selected-tile
highlight" — where a data component would carry an empty struct for nothing.

Tags are the **only** way game logic identifies kinds of entities. The `component.Name`
string (set via `archetype.NewNamed` / `WithName`) exists solely for the debug overlay —
the builder's doc comment is the contract (`archetype/entry.go`):

```go
// NewNamed creates a new entry with Transform and sets its name.
// The name should be used for debugging only.
// If you need to identify the entry in the game logic, use a Tag component instead.
```

## Canonical code

Each tag lives **next to the concept it marks** — its component file, or its own small
file — with a doc comment stating the invariant the rest of the game may rely on.

`component/solid.go`:

```go
// Solid marks an entity that occupies its cell on the Board: at most one
// solid entity can be on a cell at a time. Whatever spawns or moves a solid
// entity is responsible for keeping BoardData.Occupied in sync.
var Solid = donburi.NewTag()
```

`component/rustle.go`:

```go
// Rustle marks walkable foliage that shivers when the player walks through
// its tile (see system.Rustle).
var Rustle = donburi.NewTag()
```

`component/selectedtile.go`:

```go
// SelectedTile marks the highlight entity that sits on the tile the player is
// facing — the tile Space interacts with.
var SelectedTile = donburi.NewTag()
```

Using a tag is the same as any component, minus the data:

```go
archetype.New(w).With(component.Solid)               // attach
donburi.NewQuery(filter.Contains(component.Rustle))  // query
entry.HasComponent(component.Solid)                  // check
```

Tags also pick children out of a hierarchy: a three-sprite oven finds its state-dependent
middle sprite with `transform.FindChildWithComponent(obj, component.OvenMiddle)` — never
by child index or debug name (see [archetypes](archetypes.md) sprite separation, and
[transform](transform.md) for the find helpers).

## Rules

- **Tag for logic, `Name` for debugging.** No system may branch on a name string.
- `donburi.NewTag()` when there is no data; promote to a `donburi.NewComponentType[XData]()`
  the moment the concept grows a field. Don't pre-create empty `XData` structs "for later".
- Declare the tag next to its concept — `Solid` sits with the board-occupancy contract,
  not in a grab-bag file.
- Every tag gets a doc comment stating its **invariant**: what being tagged promises, and
  who is responsible for upholding it (see `Solid` above).
- A tag is API: when a system starts depending on one, the doc comment should name that
  system (`Rustle` → `system.Rustle`).

## Variations

- **A central `component/tags.go`** (`component/tags.go`):

  ```go
  var (
      ItemHands           = donburi.NewTag() // Close hands for holding items
      InteractionDetector = donburi.NewTag() // Farther detector for detecting interactables
      OvenMiddle          = donburi.NewTag() // Middle sprite of the oven (state-dependent)
      // ...
  )
  ```

  Acceptable at ~60 components (one registry file stays findable), but one-line comments
  are thinner than invariant doc comments. Prefer tags next to their concept; fall back
  to a central file only if the component package grows so large that stray tags get lost.
