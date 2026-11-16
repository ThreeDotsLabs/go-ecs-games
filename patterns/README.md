# Pattern catalog

Patterns for 2D games in Go with [Ebitengine](https://ebitengine.org/) and the
[donburi](https://github.com/yohamta/donburi) ECS library, distilled from a series of shipped and prototype games. 

Each pattern gets one doc: how it works, the canonical code, the rules
to follow, and the pitfalls the source games hit. Start with
[project-structure.md](project-structure.md) for the package layout the docs assume.

**The docs are prescriptive.** Snippets show the corrected form: where a source game had
a bug or used a deprecated API, the doc says so and gives the fix. File paths in snippet
captions (`system/render.go`) name where that code lives in a game laid out per
[project-structure.md](project-structure.md).

Staples the catalog doesn't cover are listed at the bottom.

## Core architecture

| Pattern | Covers |
|---|---|
| [Project structure](project-structure.md) | packages, dependency rules, file conventions |
| [ECS core](ecs-core.md) | components, systems, queries, ordering |
| [Archetypes & EntryBuilder](archetypes.md) | entity factories over a fluent builder |
| [Tags](tags.md) | marker components with documented invariants |
| [Transform & hierarchy](transform.md) | parent/child, ownership, rotation-driven movement |
| [Entity lifecycle](entity-lifecycle.md) | mark-and-sweep destruction, despawn drivers |
| [Scenes & switching](scenes.md) | one world per scene, pull-based switching |
| [Game bootstrap](bootstrap.md) | main, async asset loading, layout |
| [engine/ helpers](engine-helpers.md) | Timer, Rect, lerp, random, find helpers |
| [Events](events.md) | donburi events as the default decoupling |
| [Singletons](singletons.md) | one-per-world components and `MustFind` |
| [Background work over game snapshots](background-work.md) | off-thread searches, snapshot-owned caches, stale-answer rejection |

## Rendering & camera

| Pattern | Covers |
|---|---|
| [Layers & Y-sort](layers-ysort.md) | draw order by layer, then by world Y |
| [Cameras](cameras.md) | world/UI cameras, follow, fades (screen→world helper still missing) |
| [Sprite tint & flip](sprite-effects.md) | color overrides, flips, blend modes |
| [Chunks & culling](chunks.md) | worlds much larger than the screen (opt-in module) |
| [Color palette](colors.md) | named colors in one file |
| [Procedural VFX](vfx.md) | tween choreography, compound effect entities |

## World, assets & content

| Pattern | Covers |
|---|---|
| [Asset loading & validation](assets.md) | struct-tag manifest, reflection loader, progress |
| [Spritesheet tile catalog](tile-catalog.md) | semantic names over a packed sheet |
| [TMX / Tiled maps](tiled.md) | typed object classes, tile layers into archetypes |
| [Occupancy grid](grid-occupancy.md) | claim-then-glide cell movement |
| [Tile-world grid](grid-world.md) | a board of tiles with terrain and buildings |
| [Procedural generation](procgen.md) | data-driven map generators |
| [Placeholder art](placeholder-art.md) | sprites generated in code |

## Gameplay

| Pattern | Covers |
|---|---|
| [Input: keyboard](input-keyboard.md) | data-driven bindings, tap-to-turn |
| [Input: gamepad](input-gamepad.md) | twin-stick, multi-device join |
| [Input: touch](input-touch.md) | gesture state machine |
| [Input: cursor abstraction](input-cursor.md) | one cursor singleton for mouse and touch |
| [Collisions](collisions.md) | layer matrix, enter/stay/exit (O(n²); colliders ignore rotation) |
| [Movement](movement.md) | grid glide, free velocity |
| [Animations & tweens](animations.md) | the Animator component and choreography |
| [Timers](timers.md) | frame-count timers |
| [Enemy AI](ai.md) | FSM NPCs, steering, sense/act |
| [Data-driven rules & abilities](data-driven-rules.md) | rule tables, composable abilities |
| [Interactions & item storage](interactions.md) | interactables, containers, carry |
| [Domain separation](domain.md) | pure rules engine beside the ECS |
| [State machines](state-machines.md) | enum + switch, scripted sequences |
| [Tutorial](tutorial.md) | data-driven tutorial steps |
| [Dialogue (Twine)](dialogue.md) | Twine stories as game data |

## UI, text, audio

| Pattern | Covers |
|---|---|
| [Menus (ebitenui)](ui-ebitenui.md) | menus, lobbies, HUD with ebitenui |
| [ECS-native game UI](ui-ecs.md) | panels and dialogs as entities |
| [Widget primitives](ui-widgets.md) | bars, buttons, labels |
| [Fonts & text](fonts.md) | one TTF, sized faces, wrapping |
| [Audio](audio.md) | event-driven SFX, music cross-fade |

## Persistence, network, platform

| Pattern | Covers |
|---|---|
| [Save / load](save-load.md) | schema, projection, native + wasm storage |
| [Replay](replay.md) | record, playback, transport UI |
| [Networking](networking.md) | WebSocket transport, authoritative server |
| [WASM & web deploy](wasm.md) | browser shell, Pages deploy |
| [Mobile](mobile.md) | iOS and Android via ebitenmobile |
| [Versioning](versioning.md) | build version embedded in the binary |

## Dev experience

| Pattern | Covers |
|---|---|
| [Debug tooling](debug.md) | overlay, gizmos, hierarchy dump, time scale |
| [Dev shortcuts](dev-shortcuts.md) | one settings table for every dev hook |
| [Profiling](profiling.md) | per-system timings, headless perf report |
| [Testing](testing.md) | rule tables, golden files, full-game contracts, headless arena |

## Staples not covered

None of the source games implement these; build them fresh when first needed:

- **Platformer physics** — gravity, jumping, ground detection, coyote time.
- **Sprite-sheet frame animation as data** (`frames + fps + loop mode`) — walk cycles are
  hand-rolled Animator closures in [animations](animations.md).
- **Game feel / juice** — screen shake, hit-stop, smooth camera follow (deadzone/lookahead).
- **General pathfinding** (A* / nav grid) and **collision broadphase** (spatial hash).
- **Particles** and **Kage shaders**.
- **2D lighting** — light sources, falloff, occlusion.
- **Input-action abstraction** — bindings are polled inline.
- **9-slice panels**, **audio variation** (pitch/volume randomization), **settings UI**
  (volume, rebinding), **localization**.