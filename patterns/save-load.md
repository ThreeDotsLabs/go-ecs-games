# Save / load

## How it works

Saves are **protobuf on disk**, schema in `save/*.proto`, generated `.pb.go` committed
next to it. Two top-level messages, two files on disk: `Global` (meta-progression that
survives across runs — diamonds — in `global.save`) and `Game` (one run's state, in
`games/<slot>.save`).

What gets saved is a **hand-written projection, not the ECS world**. A `Save` system
queries components and maps them into proto messages through explicit `mapXToSave` /
`MapXFromSave` switch pairs that panic on unknown values. Loading rebuilds a fresh world
through the **same spawn functions used in gameplay**, with their side effects disabled
by a flag — so a load never re-charges costs, re-validates placement, or re-publishes
events.

## Canonical code

**Schema** — `Global` vs `Game`, every field explicit, enums for every domain enum
(`save/schema.proto`):

```proto
message Global {
    Header header = 1;  // Header{int64 version = 1}
    int64 diamonds = 2;
};

message Game {
    Header header = 1;
    Board board = 2;
    Resources resources = 3;
    Camera camera = 4;
    repeated Building buildings = 5;
    repeated Road roads = 6;
    Research research = 7;
    Player player = 8;
    Progress progress = 9;
}
```

**Projection, not serialization** — the save system reads components and builds the
proto by hand (`system/save.go`):

```go
func (s *Save) saveGame(w donburi.World) {
    game := &save.Game{}
    game.Header = &save.Header{Version: saveVersion}

    resources := engine.MustFindComponent[component.ResourcesData](w, component.Resources)
    game.Resources = &save.Resources{
        Wood:  int64(resources.Wood),
        Stone: int64(resources.Stone),
        // ...
    }

    var roads []*save.Road
    donburi.NewQuery(filter.Contains(component.Road)).Each(w, func(entry *donburi.Entry) {
        parent, _ := transform.GetParent(entry)
        tile := component.Tile.Get(parent)
        roads = append(roads, &save.Road{Position: mapPositionToSave(tile.Position)})
    })
    game.Roads = roads
    // ... board, buildings, research, player, progress; diamonds go to
    // s.globalSave.Diamonds (meta-progression outlives the slot); then:
    s.saveGameFunc(s.globalSave, game)
}
```

**Mapping pairs** — one `switch` per direction per enum, panicking on unknown values so
a schema/domain drift fails loudly (`system/save.go`):

```go
func mapTerrainToSave(terrain domain.Terrain) save.TerrainType {
    switch terrain {
    case domain.TerrainPlains:
        return save.TerrainType_TerrainPlains
    // ...
    default:
        panic(fmt.Sprintf("unknown terrain: %v", terrain))
    }
}
```

**When to save** — the system is event-driven, no polling (`system/save.go`):
`Init` subscribes to `QuitGameRequestedEvent` (save, then back to menu) and to the
`RoadSpawned`/`SettlementSpawned`/`SettlementUpgraded` fact events, which advance the
day; when the day rolls the season over, it saves. A `saveEnabled` flag (false for
tutorial and replay scenes) gates both paths.

**Load through spawn functions with side effects off** — the flag is the whole trick
(`system/buildings.go`):

```go
// SpawnRoad returns true if the road was spawned or already exists
func (b *Buildings) SpawnRoad(w donburi.World, tileEntry *donburi.Entry, requestedByPlayer bool) bool {
    // requestedByPlayer guards: idempotency check, canBuildRoad validation,
    // adjacency requirement, stone cost, and the RoadSpawnedEvent publish.
    // With false, the entity + sprite + occupancy bookkeeping still happen.
```

The loader calls it with `false` (`scene/board_load_game.go`):

```go
for _, u := range c.game.Roads {
    tile := tiles[u.Position.X][u.Position.Y]
    c.buildings.SpawnRoad(w, tile, false)
}
```

**Storage** — lenient proto options, slot files, missing file = zero value
(`storage/storage.go`):

```go
var marshalOptions = proto.MarshalOptions{AllowPartial: true}
var unmarshalOptions = proto.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}
```

Slots are files named `games/<slot>.save`; `OccupiedSlots` scans the directory and
parses `.save` filenames; `DeleteSlot` removes both the save and its replay (see
[replay.md](replay.md)); `load` returns the untouched zero-value message when the file
is missing. The save directory is per-platform via build tags: `storage_dir.go`
(`!ios && !android`) uses `configdir.LocalConfig("mygame")`; `storage_dir_ios.go`
bridges via cgo/Objective-C to `ApplicationSupportDirectory` (see
[mobile.md](mobile.md)); `storage_dir_android.go` resolves the app's files directory.

**Codegen** — `task proto` runs protoc in Docker (`dev/protoc/Dockerfile`
pins `protoc` 25.2 + `protoc-gen-go` v1.32.0) into `save/`; the `.pb.go` is committed.

## Rules

- Save a projection. The proto schema is its own model — never serialize components,
  and keep the mapping switches exhaustive-with-panic so drift is caught immediately.
- Load through the same spawn/archetype functions gameplay uses, with a
  `requestedByPlayer`-style flag disabling validation, costs, and event publishing.
  This is the cleanest solve for "loading must not re-trigger effects".
- Meta-progression (`Global`) is a separate message and a separate file from run state
  (`Game`); `SaveSlot` writes both.
- Save on defined moments (quit, season change), not every frame. `AllowPartial` +
  `DiscardUnknown` on unmarshal keep old saves loading after schema additions.
- Commit the generated `.pb.go`; pin codegen tool versions in the Dockerfile.

## Pitfalls

- **Storage needs a js/wasm backend.** A `storage/storage.go` that is `//go:build !js`
  while `game/game.go` calls `storage.NewStorage()` unconditionally doesn't merely lose
  saving on the web — the game **fails to compile** for `GOOS=js`
  (`undefined: storage.Storage`), and the web deploy breaks with it. Pair it with a
  byte-slice `saveFile`/`loadFile`: the native half writes under `os.UserConfigDir()`
  (`client/storage_native.go`, `//go:build !wasm`); the wasm half is localStorage
  (`client/storage_wasm.go`):

  ```go
  //go:build wasm
  func saveFile(file string, data []byte) error {
      js.Global().Get("localStorage").Call("setItem", file, string(data))
      return nil
  }

  func loadFile(file string) ([]byte, error) {
      val := js.Global().Get("localStorage").Call("getItem", file)
      if val.IsNull() {
          return nil, nil
      }
      return []byte(val.String()), nil
  }
  ```

- **Check `Header.Version` on load.** Writing `saveVersion = 1` and never reading it
  back is a migration bug waiting for the first schema change; check it before mapping
  fields, and migrate or reject on mismatch.
- Pick one tag spelling — `!js` or `!wasm`; either matches the js/wasm target, but
  don't mix both in one game.

## Variations

- **JSON** for a tiny blob (a network session token) instead of proto — fine at that
  size; proto earns its keep at `Game`-schema size.
- Replays reuse this storage layer (`replays/<slot>.replay`, same marshal options;
  [replay.md](replay.md)).
