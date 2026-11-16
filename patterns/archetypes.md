# Archetypes & EntryBuilder

## How it works

`archetype/` is the only place entities are assembled. It has two layers:

1. **`archetype/entry.go`** — the fluent `EntryBuilder`. `New(w)` creates the entry and
   **always attaches `transform.Transform`**, so every entity lives in the hierarchy from
   birth. Chain `With*` methods, finish with `.Entry()`.
2. **One `NewX` factory per file** (`player.go`, `camera.go`, `oven.go`, …) that
   composes the builder plus `SetValue` calls into a finished entity. Don't add game
   entities to `entry.go` — it is only for extending the builder itself.

## Canonical code

**The builder core** (`archetype/entry.go`):

```go
type EntryBuilder struct {
    entry *donburi.Entry
}

// New creates a new entry with Transform.
func New(w donburi.World) EntryBuilder {
    return EntryBuilder{
        entry: w.Entry(w.Create(transform.Transform)),
    }
}

// NewNamed creates a new entry with Transform and sets its name.
// The name should be used for debugging only.
// If you need to identify the entry in the game logic, use a Tag component instead.
func NewNamed(w donburi.World, name string) EntryBuilder {
    e := New(w).WithName(name)
    return e
}
```

Every `With*` method returns the builder, and the component-adding ones are idempotent
(`With` checks `HasComponent` before `AddComponent`).

**Every builder method** (`archetype/entry.go`; the last row is an optional extra; sprite generators such as
`CreateScaledSprite` live in `assets/`, see [placeholder-art](placeholder-art.md)):

| Method | Effect |
|---|---|
| `With(c)` | Add component `c` if absent. Set data afterwards with `c.SetValue(e, …)`. |
| `WithData(c, data)` | Add + set in one call — but its parameter is `*donburi.ComponentType[any]`, so no typed component fits; unused in both games. Use `With` + `SetValue`. |
| `WithPosition(pos)` | Sets `Transform.LocalPosition`. |
| `WithScale(scale)` | Sets `Transform.LocalScale`. |
| `WithParent(parent)` | `transform.AppendChild(parent, entry, false)`. |
| `WithSprite(data)` | Ensures `Layer` exists, adds `Sprite`, sets its data. |
| `WithLayer(id)` | Adds `Layer`, sets it. |
| `WithLayerInherit()` | Parent's layer + 1 — **panics without a parent**, so `WithParent` must come first. |
| `WithText(data)` | Ensures `Layer`, adds `Text`, sets its data. |
| `WithName(name)` | Debug-only `Name` (see [tags](tags.md)). |
| `WithYSort()` | Adds `YSort`; derives `YOffset` from the sprite's pivot — call after `WithSprite`. |
| `WithBounds(size)` | Adds `Bounds` with an explicit size. |
| `WithSpriteBounds()` | `Bounds` from the sprite image × `transform.WorldScale` — after `WithSprite`/`WithScale`/`WithParent`. |
| `Entry()` | Unwraps the `*donburi.Entry`. |
| `WithBoundsAsCollider(layer)` | *(optional)* Adds `Collider` with a rect copied from `Bounds` — call after a bounds method. |

**A compact factory** — builder chain, then `SetValue` for the data-carrying components
(`archetype/player.go`, trimmed):

```go
func NewPlayer(w donburi.World, parent *donburi.Entry, pos math.Vec2) *donburi.Entry {
    board := component.MustFindBoard(w)
    scale := board.CellSize / assets.TileSize

    player := New(w).
        WithName("Player").
        WithParent(parent).
        WithPosition(pos).
        WithScale(math.Vec2{X: scale, Y: scale}).
        WithLayer(component.LayerCharacters).
        WithSprite(component.SpriteData{Image: assets.Farmer}).
        WithYSort().
        With(component.Player).
        With(component.Solid).
        With(component.Movable).
        With(component.Animator).
        Entry()

    component.Player.SetValue(player, component.PlayerData{
        Facing:    component.Cell{X: 0, Y: 1}, TurnTimer: turnTimer,
    })
    return player
}
```

**One dispatcher for data-driven creation** — when objects come from level data or a build
menu, every creation path funnels through a single `SpawnObject`, keyed by a domain enum;
`StringToObjectType` translates level-file strings at the edge
(`archetype/spawn.go`, trimmed):

```go
func SpawnObject(world donburi.World, objectType domain.ObjectType, gridPos component.GridPosition, level *donburi.Entry) (*donburi.Entry, error) {
    switch objectType {
    case domain.ObjectTypeCashRegister:
        return NewCashRegister(world, gridPos, level), nil
    case domain.ObjectTypeOven:
        return NewOven(world, gridPos, level), nil
    // ... one case per ObjectType ...
    default:
        return nil, fmt.Errorf("unknown object type: %s", objectType)
    }
}
```

**Sprite separation** — a multi-part object is a *logic parent with no sprite*, sprite
children, and collider children (`archetype/oven.go`, trimmed):

```go
obj := New(world).
    WithPosition(worldPos).
    WithLayer(domain.SpriteLayerObjects).
    With(component.Oven).
    With(component.Collider).
    With(component.ItemHolder).
    WithYSort(). // Enable Y-sort so child items sort properly with player
    WithParent(level).
    Entry()

// State-dependent middle sprite: a child, found later by its tag.
New(world).
    WithPosition(math.Vec2{X: 0, Y: -6 * domain.SpriteScale}).
    WithSprite(component.SpriteData{
        Image: assets.Assets.Sprites.Objects.OvenStandby,
        Pivot: component.SpritePivotBottomCenter,
    }).
    With(component.OvenMiddle).
    WithParent(obj).
    WithLayer(domain.SpriteLayerObjects).
    WithYSort().
    Entry()

// Interaction collider: another child, on its own collision layer.
interactable := New(world).
    WithParent(obj).
    WithLayerInherit().
    With(component.Collider).
    Entry()
```

Why: sprites can rotate or swap (the oven middle changes with state, found via
`transform.FindChildWithComponent(obj, component.OvenMiddle)`) without touching the logic
entity; attached UI (progress bars) stays upright; and Y-sort treats the whole subtree
as one unit (see [layers-ysort](layers-ysort.md)). Apply this to every buildable object.

## Rules

- **Every world entity goes through `archetype.New`**, so it always has a `Transform`.
  The only bare `w.Create` calls are the transform-less singletons in `createWorld()`
  (`Game`, `Cursor`, `Board` — see [singletons](singletons.md)).
- One `NewX` factory per file. `entry.go` changes only when extending the builder.
- `With` adds, `SetValue` fills: **add the component before setting its value** or donburi
  panics. The builder's `With` is safe to repeat.
- Order-sensitive chains: `WithParent` before `WithLayerInherit`; `WithSprite` before
  `WithYSort` / `WithSpriteBounds`; scale and parent before `WithSpriteBounds`.
- Names (`NewNamed`, `WithName`) are debug-only. Logic finds entities by
  [tag](tags.md), never by name string.
- Data-driven object sets get one `SpawnObject` dispatcher; string→enum translation
  happens once, at the data boundary.

## Pitfalls

- Drop or fix `WithData` — its `ComponentType[any]` signature can't accept a typed
  component, which is why no call site exists.
- Keep `BoundsFromImage(entry, image)` as the one shared helper behind
  `WithSpriteBounds`; inlining it per method drifts.
- Raw `w.Entry(w.Create(…))` calls outside the builder skip its defaults — use the builder.
