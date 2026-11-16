# TMX / Tiled maps

## How it works

When a game has authored levels, **TMX is the level format** — levels are edited in Tiled,
not built in Go. The library is `github.com/lafriks/go-tiled`, always loaded with
`tiled.WithFileSystem` over the embedded FS so `.tsx` tilesets and images resolve inside
the binary. The loader parses each map into pure `domain` structs at startup (inside the
asset pipeline, [assets.md](assets.md)); `archetype/` then turns those structs into
entities through one dispatcher ([archetypes.md](archetypes.md)). go-tiled types never
leave the loader. Everything in a map is identified by **class** — Tiled's class field on
layers, objects, and tiles — plus **typed custom properties**; unknown classes are load
errors, never silent skips.

## Canonical code

**Loading** — glob-discover, parse over the embed FS
(`assets/loader/loader.go`):

```go
levelPaths, err := fs.Glob(assetsFS, "game/levels/*.tmx")
// …
levelMap, err := tiled.LoadFile(levelPath, tiled.WithFileSystem(assetsFS))
```

Adding a level = adding a file: glob `"levels/level*.tmx"` the same way — drop
in `level03.tmx` and it loads. Relative tileset refs (`../objects.tsx`) are resolved by
go-tiled itself before reaching the FS (noted in `assets/loader/tmx.go`).

**Typed object classes** — an adventure game reads six (`object`, `poi`, `entrypoint`, `trigger`,
`limits`, `fadepoint`) plus map-level properties (`cameraZoom`, `outdoor`); each class
reads its own custom properties (`assets/loader/loader.go`, trimmed):

```go
if obj.Class == "entrypoint" {
    pos := domain.CharacterPosition{LocalPosition: math.Vec2{X: obj.X}}
    if obj.Properties.GetBool("flipX") {
        pos.FlipX = true
    }
    entrypoint := domain.Entrypoint{
        Index:             obj.Properties.GetInt("index"),
        CharacterPosition: pos,
    }
    // …
}
```

POIs read `passage`, `level`, `edge`, `touchTrigger`, `parentObject` the same way, and the
loader validates hard: exactly one of passage/level per POI, sequential entrypoint
indices, at most one `limits`/`fadepoint`. A `trigger` object overrides its POI's
interaction rect **by object reference** (same file):

```go
if obj.Class == "trigger" {
    rect := engine.NewRect(obj.X, obj.Y, obj.Width, obj.Height)
    poiID := obj.Properties.GetString("poi")
    // find the POI by ID, replace its TriggerRect; an unknown ID is a load error
}
```

The character's collider is authored in Tiled too — read from the Character tileset's
per-tile `ObjectGroups` (an object with class `collider`), not hardcoded in Go.

**Commit the Tiled project** — commit `game.tiled-project` and
`propertytypes.json` beside the maps, so Tiled itself presents each class as a typed form
and validates enums at authoring time. Prescribed for every TMX game
(`assets/game/propertytypes.json`, trimmed):

```json
{ "name": "entrypoint", "type": "class",
  "members": [ { "name": "flipX", "type": "bool", "value": false },
               { "name": "index", "type": "int",  "value": 0 } ] },
{ "name": "edge", "type": "enum", "storageType": "string",
  "values": [ "left", "right" ] }
```

**Tile layers dispatched by class; objects become domain data** — switch on
`layer.Class` and flatten objects into `domain.LevelObject`
(`assets/loader/tmx.go`, trimmed):

```go
for _, layer := range m.Layers {
    switch layer.Class {
    case "floor":
        // per-cell tile IDs -> level.FloorTiles
    case "wall":
        // cells -> domain.WallObject{Position, TileID, Sprite}
    case "":
        // Object layers are handled through ObjectGroups, not layers
    default:
        return nil, fmt.Errorf("unknown layer class: %s", layer.Class)
    }
}

levelObj := domain.LevelObject{
    Position:   domain.GridPosition{X: gridX, Y: gridY - 1}, // Adjust Y for Tiled's coordinate system
    Class:      obj.Class,
    Properties: props, // flattened to map[string]string
    TileID:     int(obj.GID),
}
```

`player_spawn` and `room` are special-cased in the loader; everything else flows through
as a `LevelObject`. **One archetype dispatcher** (`createGameObject`) then turns them into
entities (`archetype/level.go`, trimmed — see [archetypes.md](archetypes.md)):

```go
switch obj.Class {
case "furniture", "dispenser":
    // … StringToObjectType(obj.Class, obj.Properties["type"]) + SpawnObject
case "light_switch":
    NewLightSwitch(world, gridPos, obj.Properties["room_name"], level)
    return nil
// … door, back_door, entrance
default:
    return fmt.Errorf("unknown object class: %s", obj.Class)
}
```

**Gameplay data in the map** — enemy paths are Tiled polylines (one-shot) or
polygons (looping); enemies point at them via object-typed properties
(`assets/assets.go`, trimmed):

```go
if len(o.PolyLines) > 0 {
    paths[o.ID] = Path{Loops: false, Points: points} // points = o.X/Y + each vertex
}
if len(o.Polygons) > 0 {
    paths[o.ID] = Path{Loops: true, Points: points}
}

// on an enemy object, the "path" and "spawn" properties hold Tiled object IDs:
pathID, err := strconv.Atoi(p.Value) // for p.Name == "path"; missing path panics
enemy.Path = paths[uint32(pathID)]
```

Level backgrounds are the tile layer **pre-rendered to one image at load**, so the runtime
never draws individual map tiles (same file; alternatively author one webp background
per map as a Tiled image layer, decoded lazily — [assets.md](assets.md)):

```go
renderer, err := render.NewRendererWithFileSystem(levelMap, assetsFS)
// …
err = renderer.RenderLayer(0)
level.Background = ebiten.NewImageFromImage(renderer.Result)
```

## Rules

- TMX whenever levels are authored; hand-built `createWorld()` content is for demos and
  procedural games ([procgen.md](procgen.md), [grid-world.md](grid-world.md)).
- Always load with `tiled.WithFileSystem(embedFS)`; maps, tilesets, and images ship embedded.
- The map is **data**: loader → pure domain structs → one archetype dispatcher → entities.
- Dispatch on class + typed custom properties, with `default: fmt.Errorf("unknown …")` in
  every switch.
- Commit the `.tiled-project`, `propertytypes.json`, and `.tsx` files — the editor then
  validates exactly what the loader expects.
- Validate invariants at load; cross-reference targets at startup ([assets.md](assets.md)).

## Pitfalls

- Keep loader state (`levelNames`, `characterCollider`, `tilesetCache`) in a loader
  struct, not package globals.
- Gather spawn-point special-casing (`player_spawn`, `customer_spawn`) in one place;
  skimming one off in an archetype before the dispatcher hides it.
- Return loader errors into the async loading pipeline instead of `MustLoad*` panics.
- Decode each tileset image once, not per tile lookup ([tile-catalog.md](tile-catalog.md)).
- go-tiled resolves `../` tileset paths itself; a relative-FS shim around the embed FS
  is dead weight.

## Variations

- Parse a `.tsx` directly with `encoding/xml` to slice a character sheet — a tileset
  without a map ([tile-catalog.md](tile-catalog.md)).
- Tie POIs to Twine passages by name — the passage side is [dialogue.md](dialogue.md).
