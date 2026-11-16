# Data-driven rules & abilities

## How it works

Game content — recipes, spells, scoring — is **declarative data in a pure package**
(`domain/`, or a dedicated one like `spell/`), and systems *interpret* it. The rules
package imports no ECS and no rendering beyond leaf value types (`spell/` touches only
`*ebiten.Image` handles and `math.Vec2`). Adding content means
adding a table entry — no new system, no new switch case. When a rule needs behavior that
varies per entry (a projectile spread, a scoring formula), the entry stores a **function**.

## Canonical code

**Lookup tables** — a slice of rule structs plus a `CanX(input) (Rule, bool)` query
(`domain/items.go`; `domain/baking.go` has the same shape for `BakingRule`):

```go
type WorkingRule struct {
    Input       IngredientType
    Output      IngredientType
    OutputCount int // Number of output items produced
    WorkingTime time.Duration
}

var kneadingRules = []WorkingRule{
    {Input: IngredientRawDough, Output: IngredientDough, WorkingTime: 2 * time.Second},
    {Input: IngredientRawSourdough, Output: IngredientSourdough, WorkingTime: 2 * time.Second},
    {Input: IngredientRolledDough, Output: IngredientDough, WorkingTime: 2 * time.Second},
    // ...
}

func CanKnead(ingredient IngredientType) (WorkingRule, bool) {
    for _, rule := range kneadingRules {
        if rule.Input == ingredient {
            return rule, true
        }
    }
    return WorkingRule{}, false
}
```

The same file is the item catalog: `var Items = map[ItemID]Item` (kind, display name,
price) plus `combinationRules`/`CanCombine(a, b)` for "flour + water = raw dough". The
kneading-counter system calls `domain.CanKnead(...)` and acts on the returned rule — it
knows nothing about dough.

**Abilities as effect lists** — a spell is costs/timings plus `[]Effect` per lifecycle
moment; an effect is a type tag plus a typed payload (`spell/spells.go`,
`spell/effect.go`):

```go
type Spell struct {
    Name                  string
    IsChannel             bool
    ChannelTickDuration   time.Duration
    CastWhileMoving       bool
    ManaCost              int
    CastingTime           time.Duration
    Cooldown              time.Duration
    OnCastEffects         []Effect
    OnChannelTickEffects  []Effect
    OnCastFinishedEffects []Effect
}

type Effect struct {
    Type EffectType // SpawnProjectiles | ApplyAura | SpawnEntity
    Data any        // SpawnProjectilesData | ApplyAuraData | SpawnEntityData
}
```

**Behavior stored in data — the star.** `SpawnProjectilesData.Directions` is a
`func(direction math.Vec2) []math.Vec2`: novas, fans and volleys are table entries, and
**one spawner handles all of them** (`spell/spells.go`; all 15 spells live in
`LoadSpells()`):

```go
FrostNova = Spell{
    Name: "Frost Nova", ManaCost: 30,
    CastingTime: 750 * time.Millisecond, Cooldown: 8 * time.Second,
    OnCastEffects: []Effect{{
        Type: EffectTypeSpawnProjectiles,
        Data: SpawnProjectilesData{
            Image: assets.FrostProjectile, Speed: 6, Damage: 3, Duration: 1 * time.Second,
            Directions: func(direction math.Vec2) []math.Vec2 { // 8-way ring
                var directions []math.Vec2
                for i := 0; i < 8; i++ {
                    angle := float64(i) * 45 * stdmath.Pi / 180.0
                    directions = append(directions, math.Vec2{X: stdmath.Cos(angle), Y: stdmath.Sin(angle)})
                }
                return directions
            },
            OnHitEffects: []Effect{{ // nested: every hit applies a 50% slow aura
                Type: EffectTypeApplyAura,
                Data: ApplyAuraData{AuraEffect: AuraEffect{
                    ID: "frost-slow", OnApply: AuraEffectTypeSlowMovement,
                    Duration: 3 * time.Second, Amount: 0.5,
                }},
            }},
        },
    }},
}
```

`ArcaneVolley` is the same entry with a ±15° fan (`angles := []float64{15, 10, 0, -10, -15}`
rotated around the aim direction); `Spark` omits `Directions` entirely and the spawner
falls back to the caster's facing (`archetype/projectile.go`). **Nested
`OnHitEffects` make composition pure data**: "fireball that applies poison" is a list
inside a list. The spawner copies them onto the projectile
(`component.SpellEffect{Caster, Effects}`), and the collision system resolves them on
impact against the victim (`system/collision_damage.go`).

**Resolver-chain dispatch** — systems interpret effects through an extensible chain; each
resolver declines types it doesn't own, and an unhandled type panics loudly
(`system/casting.go`):

```go
var spellEffectResolvers = []SpellEffectResolver{
    ResolveSpellEffectNone,
    ResolveSpellEffectSpawnProjectile,
    ResolveSpellEffectApplyAura,
    ResolveSpellEffectSpawnEntity,
}

type SpellEffectResolver func(caster *donburi.Entry, target *donburi.Entry, effect spell.Effect) bool
func resolveSpellEffects(caster *donburi.Entry, target *donburi.Entry, effects []spell.Effect) {
    for _, effect := range effects {
        resolved := false
        for _, resolver := range spellEffectResolvers {
            if resolver(caster, target, effect) {
                resolved = true
                break
            }
        }
        if !resolved {
            panic(fmt.Sprintf("unknown spell effect: %v", effect.Type))
        }
    }
}
```

Adding an effect type = new payload struct + new resolver appended to the slice. The same
`resolveSpellEffects` runs on-cast, on-channel-tick, and on-hit — one interpreter, every
entry point.

## Rules

- Rules are declarative data in `domain/` or a dedicated pure package; systems interpret.
  Adding content = adding a table entry.
- Per-entry behavior goes in function fields (`Directions`, a dice game's `Scoring`), not in
  `switch` statements inside systems.
- Effects compose by nesting (`OnHitEffects` inside `SpawnProjectilesData`).
- Definitions that embed loaded assets must be built by a function called after asset
  loading — `LoadSpells()` runs right after `assets.MustLoadAssets()` in
  `main.go`, which is why the spells aren't package-level literals.
- Keep the catalog as the single source of truth; components reference entries by ID
  (see [domain.md](domain.md)).

## Pitfalls

- **`Effect.Data any` + unchecked assertions** (`effect.Data.(spell.SpawnProjectilesData)`)
  is the design's weakness: a wrong payload compiles and panics at cast time. Prefer a
  sealed interface (`type EffectData interface{ isEffect() }` + a `switch` the compiler
  can check) or typed generics — keep the resolver chain, type the payloads.

## Variations

- **The minimal form**: `Box{Name string; Scoring scoringFunc}` — a dice game's scoring
  rule is a func field over plain dice data (`scene/table.go`, `scoring.go`), fully
  table-tested (see [testing.md](testing.md)).
- **Physics**: `collisionEffects` is a
  layer-pair-to-closure table consulted by one collision system (see
  [collisions.md](collisions.md)).
- **A board game** drives its whole rulebook from an action table
  (`spaceActions map[SpaceType][]Action`) — see [domain.md](domain.md).
