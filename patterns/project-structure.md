# Project structure

The canonical layout for a game built on these patterns: Go, Ebitengine and donburi,
one package per role.

## Layout

```
my-game/
├── main.go            # thin ebiten.Game: window config, delegates to the current scene
├── scene/             # Scene interface + one file per scene; each scene owns a donburi.World
│   ├── scene.go       #   Scene / System / Drawable / Initializer interfaces live HERE
│   ├── menu.go        #   a pre-game scene (menu, lobby): pure ebitenui, no world
│   └── game.go        #   the gameplay scene: ordered systems + drawables + createWorld()
├── component/         # pure data: one component per file (XData struct + var X)
├── system/            # behavior: one system per file (struct + NewX() + Update(w))
├── archetype/         # entity factories: EntryBuilder (entry.go) + NewPlayer, NewCamera, …
├── domain/            # (optional) pure game rules & tables: no donburi, no ebiten draw calls
├── engine/            # game-agnostic helpers: Timer, Rect, lerp, random, find helpers
├── assets/            # //go:embed + loaders + tile catalog + colors.go palette
├── client/ server/    # (optional) network transport + authoritative server (see networking doc)
└── web/               # WASM shell (index.html, main.html); game.wasm is gitignored
```

## Dependency rules (enforced by review, violations are bugs)

- `engine/` imports **only** donburi + stdlib + ebiten math. Never `component/`,
  `assets/`, or `domain/`. If a helper needs a component type, it belongs in that
  component's file or a system.
- `component/` holds **data only**. Receiver methods are fine when they are pure
  helpers over that data (`BoardData.IsFree`, `SpriteData.Hide`) — no queries, no
  world mutation, no drawing.
- `system/` holds behavior. A system struct owns **cached queries built once in its
  constructor** and as little state as possible; game state lives on components.
- `domain/` (when present) has **zero ECS and zero rendering** — rules, constants,
  balance tables, event types. Components store domain *IDs/references*, not copies
  (`component.ItemData{ItemID domain.ItemID}`). Never put `*ebiten.Image` in domain
  structs: it couples the rules to asset load order.
- `archetype/` composes components into entities. It may read `assets/` and `domain/`.
  Keep UI *controller* logic (panel show/hide) out of it; that is a system's job.
- `assets/` loads and validates at startup; no game logic. A loader that headless
  tools need too (say, `assets/maps` for Tiled maps) lives in a subpackage that never
  imports ebiten, whose init needs a display; `assets` forwards to it for the game.

## File conventions

- One component per file, uniform shape:

  ```go
  type MovableData struct {
      Moving         bool
      Speed          float64
      TargetPosition math.Vec2
  }

  var Movable = donburi.NewComponentType[MovableData]()
  ```

- Tags are `donburi.NewTag()` vars declared **next to the concept they mark**
  (`Solid` in `component/solid.go`), each with a doc comment stating
  its invariant. `component.Name` strings are for the debug overlay only — game logic
  identifies entities by tag, never by name string.
- One system per file: struct + `NewX()` constructor + `Update(w donburi.World)`;
  add `Draw(w, screen)` only when it renders, `Init(w)` only for one-time setup or
  event subscriptions.
- Scene interfaces (`Scene`, `System`, `Drawable`, `Initializer`) live in
  `scene/scene.go` — systems implement them structurally, no imports needed.

## Wiring invariants

- **Each scene owns its own world.** Scene switch = build the new scene (fresh world),
  drop the old. Restart = re-run `createWorld()`.
- **System order is the ordered slice in the scene constructor** — nothing else orders
  systems. Comment every non-obvious ordering dependency inline (see
  [ecs-core](ecs-core.md)). `system.NewDestroy()` is always last.
- **An unregistered system silently never runs.** Registering in `g.systems` /
  `g.drawables` is part of adding a system.
- **Required singletons**: exactly one `component.Game` and one `component.Cursor`
  (plus `component.Board` in grid games), created up front in `createWorld()`.
  `engine.MustFindComponent` / `component.MustFind*` panic when they are missing.
- **Every visible entity lives under a camera's `Root`**, and that root carries a
  `Layer` component — the renderer panics otherwise.

## Naming

- Module `github.com/<you>/<game>`. Packages singular: `component`, `system`,
  `archetype`, `scene`. Files lowercase, no underscores except build tags
  (`storage_wasm.go`) and `_test.go`.
- Constructors `NewX`; must-succeed lookups `MustX`; component data `XData`,
  component type var `X`.
