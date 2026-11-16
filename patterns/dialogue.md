# Dialogue (Twine)

## How it works

Dialogue is **authored externally and parsed at load**, never written in Go:

1. Write in the **Twine** editor using the Chapbook format plus custom directives.
2. Export/commit as `.twee` text — embedded at `assets/game/game.twee`.
3. `assets/twine/parser.go` (452 lines, imports only stdlib + `domain` types, backed by a
   380-line test in `parser_test.go`) parses it into `domain.RawStory` at startup.
4. `domain.NewStory(w, raw)` links the `Passage`/`Link` graph and owns runtime state:
   facts, inventory, money.

Rendering is a separate concern (`archetype/dialog.go` builds the dialog UI from a
passage — see [ui-ecs.md](ui-ecs.md)); the story publishes domain events (music changes,
inventory updates) consumed by the audio and movement systems (see
[events.md](events.md)). Levels and POIs reference passages **by title**: a passage
tagged `[Level]` auto-plays on entering its level, and a Tiled POI's `passage` property
opens one on interaction (see [tiled.md](tiled.md)).

## The format

Real passages (`assets/game/game.twee`) — front-matter macros and
conditions above `--`, paragraphs with inline `[directives]`, `> [[links]]` with
`{tag}` prefixes:

```
::"train-station,0" [Level]
playMusic: station_ambience
--
[h1,effect fade-in]
Millbrook Station
[continue]

[after 1s]
The station clock has stopped at exactly midnight.

::Hotel Clerk
[if fact learnedAboutWatchmaker]
"The watchmaker? Top floor. Though time moves... strangely up there."
[else]
"Welcome to Gilman House. How many nights? Or days? Hard to tell anymore."
[continue]

> {exit} [[Leave->Hotel Clerk]]

::Try to wind it
setFact: woundClock
--
The mechanism groans to life, but instead of ticking forward, shadows in the
station begin moving in reverse.

> {level:bus-station,0} [[Follow the shadows]]
```

The vocabulary, straight from the parser's switch statements:

- **Paragraph directives**: `[h1]` `[center]` `[hint]` `[fear]` `[after 2s]`
  `[effect typing 2s]` / `[effect fade-in]` `[if …]` `[unless …]` `[else]` `[continue]`
  (ends the directive's scope). Directives combine: `[h1,effect fade-in]`.
- **Conditions** compile to `[]domain.Condition`: `fact x`, `hasItem x`, `hasMoney 100`,
  combined with `&&`, negated with `!` — `[if hasItem key && !hasMoney 100]`.
- **Macros** (front matter) compile to `[]domain.Macro`: `addItem` `takeItem` `setFact`
  `addMoney` `takeMoney` `playMusic` `changeCharacterSpeed` — **the writer drives audio
  and pacing**; `Passage.Visit` publishes `MusicChangedEvent` /
  `CharacterSpeedChangedEvent` that `system/audio.go` and `system/controls.go` consume.
- **Link tags**: `{exit}` closes the dialog, `{back}` marks a return link (excluded from
  exhaustion, below), `{level:bedroom,0}` jumps to a level entrypoint.
- **Passage tags**: `[once]` makes a passage playable a single time; `[Level]` binds it
  to a level name.

The parser emits pure structures the rest of the game consumes
(`domain/story.go`) — `Try to wind it` above becomes:

```go
domain.RawPassage{
    Title:      "Try to wind it",
    Macros:     []domain.Macro{{Type: domain.MacroTypeSetFact, Value: "woundClock"}},
    Paragraphs: []domain.Paragraph{{Text: "The mechanism groans to life, ..."}},
    Links: []domain.RawLink{{
        Text: "Follow the shadows", Target: "Follow the shadows",
        Level: &domain.TargetLevel{Name: "bus-station", Entrypoint: &zero},
    }},
}
```

(verified by running the parser; `parser_test.go` pins the full mapping, conditions and
`[else]` inversion included).

## The domain side

`domain.NewStory` links `RawLink.Target` strings into `*Passage` pointers and tracks
state. The pieces to keep:

- **`Passage.Visit()`** marks visited and executes macros (items, money, facts, events).
- **`Passage.Links()`** filters `AllLinks` by conditions and one-time visits — the UI
  only ever sees currently-valid choices.
- **`Link.AllVisited()`** walks the whole subtree behind a link (`deepChildLinks`,
  cycle-safe via a visited set, skipping `{back}` links and links returning to the
  source) and reports whether everything below was seen. The dialog renders exhausted
  branches in a dimmed color (`archetype/dialog.go`):

```go
textColor := assets.TextBlueColor
if link.AllVisited() {
    textColor = assets.TextDarkColor
}
```

This is the UX detail worth keeping: players see at a glance which conversation branches
still hold something new.

## Rules

- Dialogue content lives in the `.twee` file; Go code contains zero dialogue text.
  Writers iterate in Twine without touching code.
- The parser depends only on stdlib + `domain` types — no ebiten, no donburi — so it
  stays unit-testable; every format feature gets a case in `parser_test.go`.
- Parse **and validate** at load: `assets/loader/loader.go` has `validateAssets`
  cross-checking passages referenced by levels/POIs exist, `{level:…}` targets exist,
  `playMusic` tracks exist, and every `fact`/`hasItem` *required* by a condition is *set*
  by some macro. Cross-reference validation belongs in the asset validator (see
  [assets.md](assets.md)).
- Parser panics on malformed input are fine — it runs once at load, and failing fast on
  a typo'd macro beats a silently dead branch.

## Pitfalls

- **A disabled validator is worse than none**: an early `return nil` left in
  `validateAssets` turns every check after it into dead code while the game still looks
  validated. Keep the validator covered by a test that feeds it a broken story.
- **Brace groups accumulate**: when parsing `{level:…}` and `{exit}` link prefixes,
  append each group to `link.Tags`; re-assigning after the level branch drops all but the
  last group.
- **Quoted titles**: Twine allows `::"train-station,0"`. Strip the quotes in
  `parsePassage`, or lookups by `train-station,0` silently miss and the level intro never
  plays.
- `domain.Story` holds a `donburi.World` purely to publish events — the named acceptable
  leak; see [domain.md](domain.md) before copying it.
