# Singletons

## How it works

A singleton is a component that exists on **exactly one** entity in the world:
`component.Game` (screen dimensions + debug flags) and `component.Cursor` (pointer
state) always; `component.Board` in grid games. They are created **up front** in
`createWorld()` — before any entity that might depend on them — and looked up with
`MustFind*` accessors that panic when the component is missing. The panic is
intentional: a missing singleton is an authoring error in scene setup, not a runtime
condition to handle.

## Canonical code

**Creation, first thing in the scene** (`scene/game.go`, trimmed):

```go
func (g *Game) createWorld() {
    w := donburi.NewWorld()

    game := w.Entry(w.Create(component.Game))
    component.Game.SetValue(game, component.GameData{
        Dimensions: component.Dimensions{
            ScreenWidth:  screenWidth,
            ScreenHeight: screenHeight,
        },
        Debug: component.Debug{Enabled: false},
    })

    w.Create(component.Cursor)

    boardEntry := w.Entry(w.Create(component.Board))
    component.Board.SetValue(boardEntry, component.BoardData{ /* ... */ })
    // ... everything else comes after ...
}
```

**The generic accessor** — a `Component[T]` constraint marries the untyped
`IComponentType` to its data type (`engine/find.go`):

```go
type Component[T any] interface {
    donburi.IComponentType
    Get(entry *donburi.Entry) *T
}

func MustFindComponent[T any](w donburi.World, c Component[T]) *T {
    entry, ok := donburi.NewQuery(filter.Contains(c)).First(w)
    if !ok {
        panic("MustFindComponent: entry not found")
    }
    return c.Get(entry)
}
```

**Per-component wrappers** live next to the component, so call sites read as intent
(`component/game.go`; `MustFindBoard` is identical, and `MustFindCursor` is the
equivalent two-step `engine.MustFindWithComponent` + `Get` — normalize to this form):

```go
func MustFindGame(w donburi.World) *GameData {
    return engine.MustFindComponent[GameData](w, Game)
}
```

Usage anywhere: `board := component.MustFindBoard(w)` — no plumbing, no nil checks.

## Rules

- Exactly one `Game` and one `Cursor` per world (plus `Board` in grid games), created
  first in `createWorld()`. Systems panic without them — preserve this when rewriting
  a scene.
- **Only true singletons get `MustFind*`.** Anything that can legitimately be absent,
  or plural, goes through queries ([ecs-core](ecs-core.md)).
- Panic-on-missing stays — don't soften it into an error return; it fires on the first
  frame and points straight at broken scene setup.
- Add a new singleton = component file + `MustFindX` wrapper beside it + creation in
  `createWorld()`, all three together.
- Calling `MustFind*` every `Update` is the norm (it's a query `First`). A system may
  instead cache the pointer in `Init(w)` — `system/render.go` reads
  `MustFindGame` in `Init` to size its offscreen — but only cache per-world state in a
  system when it's derived setup like that; the data itself stays on the component
  (see [engine-helpers](engine-helpers.md) for the rest of `engine/find.go`).
