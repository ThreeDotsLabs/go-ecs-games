# Color palette

## How it works

Every color the game draws is a named `color.RGBA` var in `assets/colors.go`, grouped
by purpose. Systems and archetypes refer to `assets.ProgressBarFill`, never to a
literal — so a palette change is one file, a color's meaning is greppable, and two
UI elements can't drift apart by each inlining "roughly the same gray".

## Canonical code

**Named vars grouped by purpose** (`assets/colors.go` — trimmed; the real file
runs ~90 vars over groups: text, UI, progress bars, day/night, and per-object
palettes like bed, coffee, dough, and pie colors):

```go
// Progress bar colors
ProgressBarBackground  = color.RGBA{60, 60, 60, 255}
ProgressBarFill        = color.RGBA{0, 255, 0, 255}
ProgressBarBorder      = color.RGBA{0, 0, 0, 255}
ProgressBarFillWorking = color.RGBA{100, 200, 100, 255}
ProgressBarBorderGray  = color.RGBA{80, 80, 80, 255}
...
// Scene colors
SceneBackground = color.RGBA{70, 140, 70, 255}
```

**Palette provenance** — commit the GIMP palette you draw from right next to
`colors.go`, so the source of every value survives (`assets/poisson-23.gpl`):

```
GIMP Palette
#Palette Name: Poisson 23
#Description: Based on images from the First Colour Publication on Fish (1754)
#Colors: 23
187	176	148	bbb094
128	118	101	807665
...
```

Every var in `assets/colors.go` is one of those 23 values under a purpose
name — `PanelColor` is `a1563c`, `TextColor` is `f7efc7` — including a semantic group
for the seasons (condensed here; the file writes one field per line):

```go
SpringColor = color.RGBA{R: 139, G: 180, B: 141, A: 255}
SummerColor = color.RGBA{R: 112, G: 147, B: 149, A: 255}
FallColor   = color.RGBA{R: 195, G: 130, B: 82, A: 255}
WinterColor = color.RGBA{R: 161, G: 205, B: 176, A: 255}
```

**Minimal is fine — but comment the why.** A whole palette can be two vars, as long as
the non-obvious one explains itself (`assets/colors.go`):

```go
// SceneBackground is the color the renderer clears the screen to each frame.
// It matches the grass baked into the Kenney soil tiles so tilled plots blend
// seamlessly into the field.
SceneBackground = color.RGBA{R: 0x84, G: 0xc6, B: 0x69, A: 0xff} // grass green
```

## Rules

- **No color literals in `system/`, `archetype/`, or `scene/`** — every color is a
  named var in `assets/colors.go`. Treat it as part of a no-magic-numbers rule and
  follow it from the first commit; retrofitting is drudgery.
- Name by **purpose**, not appearance, whenever a color has a role: `TextDarkColor`,
  `ProgressBarFillWorking`, `WarningColor`. Bare `White`/`Gray` vars are acceptable
  only as shared primitives.
- Group with comment headers by purpose (text / UI / per-object), so the file reads
  as a design document.
- Comment any value with a non-obvious reason (the grass match above).
- `golang.org/x/image/colornames` is for **debug draws only** — the renderer's
  gizmos use `colornames.Red`/`Magenta`/`Lime` (`system/render.go`), and
  that's where it stops. Game visuals never reference it.
- When working from an external palette, commit the palette file next to `colors.go`
  (a `.gpl`) and pick vars from it.

## Pitfalls

- Literal-suffixed names (`DoughGoldenBrown`, `ProofedSlightlyDarkerYellow`) show the
  naming strain of ~90 one-off object colors — when a game needs per-object
  palettes that large, prefer structured groups (a `Dough` struct of stages) over
  ever-longer adjective chains.
- Declare vars as concrete `color.RGBA`, not `color.Color`, so values stay comparable
  and printable.
