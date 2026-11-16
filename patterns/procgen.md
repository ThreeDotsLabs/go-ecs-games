# Procedural generation

## How it works

World generation is three independent normalized simplex-noise layers — altitude,
temperature, humidity (`ojrac/opensimplex-go`) — each with its own derived seed and scale,
bucketed into discrete `LayerID`s by configured `{min,max}` ranges. Biome is then a pure
3-axis table lookup, Whittaker-style: `TerrainsMap[Altitude][Temperature][Humidity]`.
Deposits (trees, rocks, deer…) scatter by a blue-noise trick — keep only local maxima of a
high-frequency simplex field within radius r, a cheap Poisson-disk approximation. All tuning
lives in YAML; a second binary (`cmd/sandbox`) exists solely to tune it.

The generator is pure and headless: config in, plain slices out (`Terrains [][]domain.Terrain`,
`Deposits [][]*domain.Deposit`). Turning those into tile entities is the map system's job
([grid-world.md](grid-world.md)); sprite selection (which tile image fits its neighbors) comes after that.

## Canonical code

**One master seed, arithmetically derived per layer** (`scene/generator/generator.go`).
`NewWorldGenerator` replaces `Seed == 0` with a random one — 0 means "random" in every config:

```go
func (w *WorldGenerator) Generate() World {
	gw := World{
		Altitude:    w.generateLayers(w.config.Seed*19+19, w.config.Altitude.Scale, w.config.Altitude.Layers),
		Temperature: w.generateLayers(w.config.Seed*191+191, w.config.Temperature.Scale, w.config.Temperature.Layers),
		Humidity:    w.generateLayers(w.config.Seed*1919+1919, w.config.Humidity.Scale, w.config.Humidity.Layers),
	}

	gw.Terrains = w.generateTerrains(gw)
	gw.Deposits = w.generateDeposits(gw)

	return gw
}
```

**Sampling and bucketing one layer** — coordinates normalized to `[-0.5, 0.5)` so `Scale`
is frequency, the noise value bucketed by the configured ranges:

```go
	simplex := opensimplex.NewNormalized(seed)
	// … per cell:
			noise := simplex.Eval2(normalizedX*scale, normalizedY*scale)

			for layerID, r := range ranges {
				if noise >= r.Min && noise < r.Max {
					data[x][y] = Layer{
						LayerID: layerID,
						Value:   noise,
					}
					break
				}
			}
```

**Biome is a table, not code** — `data[x][y] = TerrainsMap[altitude][temperature][humidity]`.
Low altitude is always water, low-medium shore, high mountains; only medium branches on climate:

```go
	AltitudeMedium: {
		// … TemperatureCold: all plains …
		TemperatureMild: {
			HumidityDry:    domain.TerrainPlains,
			HumidityNormal: domain.TerrainPlains,
			HumidityWet:    domain.TerrainForest,
		},
		TemperatureHot: {
			HumidityDry:    domain.TerrainDesert,
			HumidityNormal: domain.TerrainPlains,
			HumidityWet:    domain.TerrainPlains,
		},
	},
```

**Deposits: blue noise** (`blueNoise(seed, r)`). A high-frequency field (`Eval2(normalizedX*50,
normalizedY*50)`); a cell survives only if it is the maximum within radius r — larger r, sparser:

```go
	for xc := 0; xc < w.config.Width; xc++ {
		for yc := 0; yc < w.config.Height; yc++ {
			max := 0.0

			for dx := -r; dx <= r; dx++ {
				for dy := -r; dy <= r; dy++ {
					// … max = highest in-bounds blueNoise[xc+dx][yc+dy] …
				}
			}

			if blueNoise[xc][yc] == max {
				result[xc][yc] = true
			}
		}
	}
```

The radius comes per (terrain, deposit) from the `DepositOnTerrainR` table ("the lower
the R, the more points" — forest: trees r=1, deer r=4), and each deposit type gets its
own derived seed (`w.config.Seed*factor+factor`, `factor` stepping by 33 per deposit).

**Fully data-driven** (`assets/map/world.yaml` — 200×200, seed 0 = random):

```yaml
seed: 0
width: 200
height: 200

altitude:
  scale: 8
  layers:
    0:
      min: 0.0
      max: 0.3
```

`tutorial.yaml` pins `seed: 19` on a 100×100 map — a fixed seed turns the generator into
authored content; the tutorial hard-codes its start position at (42, 30) on that known map.
`assets` embeds and parses these (`mustLoadWorldGeneratorConfig`); `generator.ConfigFromAssets`
adapts `assets.GeneratorConfig` → `WorldGeneratorConfig` so the generator never imports `assets`.

**Placement by retry, reroll on failure** (`scene/board_new_game.go`):

```go
	for i := 0; i < 50; i++ {
		settlementX := engine.RandomIntRange(mapWidth/3, mapWidth-mapWidth/3)
		settlementY := engine.RandomIntRange(mapHeight/3, mapHeight-mapHeight/3)

		tile := component.Tile.Get(tiles[settlementX][settlementY])
		if tile.Terrain == domain.TerrainPlains {
			return settlementX, settlementY, true
		}
	}

	return 0, 0, false
```

On `!ok` the caller switches into a fresh new-game scene — no valid start tile means reroll
the whole world, never crash. The small-but-typical companion to any generator.

## The sandbox binary

Tuning tools are **separate `cmd/` binaries reusing the real render pipeline**.
`cmd/sandbox/main.go` boots `scene/sandbox.go`: the real camera, chunks, `CreateTiles` and
renderer — but config comes from `sandbox.yaml` in the **working directory** (`os.ReadFile`,
not embedded, so it is live-editable), a Reload button rebuilds the world (`s.world =
s.createWorld()` — edit YAML, click, see the map), toggle buttons overlay per-tile debug
tints for altitude/temperature/humidity, and a board toggle hides terrain to view raw layers.
Noise ranges are untunable blind; this ~300-line scene made `world.yaml`'s numbers findable.

## Rules

- Generator stays pure and headless; the `ConfigFromAssets` adapter keeps the dependency
  arrow pointing at the generator, never out of it.
- One master seed, per-layer and per-deposit seeds derived arithmetically — one number
  reproduces the terrain. Seed 0 = random; a pinned seed = authored content.
- Meaning comes from data: bucket ranges, the terrain table and deposit radii are config
  and tables, not branching code — tuning never requires recompiling logic.
- Separate generation from instantiation from sprite resolution — three passes.

## Pitfalls

- `generateDeposits` iterates `for deposit := range domain.Deposits` — **Go map order** —
  so per-deposit seed assignment (`factor` increments per iteration) and which deposit
  wins a contested cell are nondeterministic even with a pinned seed; resource *amounts*
  also come from world-seed-independent `engine.RandomIntRange`. Terrain is fully
  reproducible, deposits are not — iterate a sorted slice and seed the amount RNG before
  relying on fixed-seed maps.
- Bucketing has no fallback: a noise value outside every configured range silently leaves
  the zero `Layer` (LayerID 0). Validate at load that the ranges cover `[0,1]`.

## Variations

- **Characters, not maps** — paper-doll composition. Parts are picked
  randomly from loaded tables (`engine.RandomFrom(component.AllBodyParts.Bodies)`,
  `RandomFromOrEmpty` for optional hair/armor/weapons — `archetype/player.go`),
  then `CharacterData.Image()` bakes them once into a single sprite, drawing body → feet →
  legs → chest → hair → facial hair → head → weapons in fixed z-order
  (`component/character.go`). Same shape: pick from data tables, bake once.
