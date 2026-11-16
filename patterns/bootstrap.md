# Game bootstrap

## How it works

`main.go` stays thin: parse flags, size the window from the monitor, `RunGame`.
Everything else lives in `game/game.go` — a `Game` struct implementing `ebiten.Game`
that owns async asset loading with a text progress screen, the current scene, global
keys (F = fullscreen), optional panic-safety, and the layout policy ([scenes](scenes.md)).

## Canonical code

**Thin main** — window sized as a percentage of the monitor, never absolute pixels
(`main.go`):

```go
func main() {
	quickFlag := flag.Bool("quick", false, "quick mode")
	flag.Parse()

	config := game.Config{
		Quick: *quickFlag,
	}

	ebiten.SetVsyncEnabled(true)
	w, h := ebiten.Monitor().Size()
	ebiten.SetWindowSize(engine.IntPercent(w, 0.8), engine.IntPercent(h, 0.8))

	err := ebiten.RunGame(game.NewGame(config))
	if err != nil {
		log.Fatal(err)
	}
}
```

`-quick` skips the title straight into gameplay ([dev-shortcuts](dev-shortcuts.md)).

**Async asset loading** — fonts load synchronously (the progress screen needs them),
everything else in a goroutine reporting over channels (`game/game.go`):

```go
	assets.MustLoadFonts()
	audio.NewContext(44100)

	progressChan := make(chan string)
	errorChan := make(chan error)

	go assets.LoadAssets(progressChan, errorChan)

	go func() {
		// (defer close of both channels)
		err := <-errorChan
		if err != nil {
			g.loadingLines = append(g.loadingLines, fmt.Sprintf("ERROR: %v", err))
			return
		}

		if config.Quick {
			g.switchToGame()
		} else {
			g.switchToTitle()
		}
	}()
```

A third goroutine drains `progressChan` into `g.loadingLines`; `Draw` renders the
accumulated lines until a scene exists (`game/game.go`):

```go
	if g.scene == nil {
		op := &text.DrawOptions{}
		op.LineSpacing = assets.NormalFont.Size
		op.GeoM.Translate(10, 10)
		lines := strings.Join(g.loadingLines, "\n")
		text.Draw(screen, lines, assets.NormalFont, op)
		return
	}
	g.scene.Draw(screen)
```

**Config.Safe** — render the panic instead of crashing; same block guards `Draw`
(`game/game.go`):

```go
func (g *Game) Update() error {
	if g.config.Safe {
		defer func() {
			if r := recover(); r != nil {
				g.loadingLines = append(g.loadingLines, fmt.Sprintf("PANIC (Update): %v", r))
				g.scene = nil
			}
		}()
	}
	// ...
}
```

Nulling the scene drops back to the loading-lines screen, which now shows the panic.
Mobile builds pass `Safe: true` (`mobile/main.go`); desktop dev
leaves it off so panics crash loudly with a stack trace.

**Global keys** — fullscreen lives here, not in scenes (`game/game.go`):

```go
	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
```

**Layout — pick one of three strategies.**

*Native-pixel* — logical size follows the window at full device
resolution; scenes are told when it changes (`game/game.go`):

```go
func (g *Game) Layout(width, height int) (int, int) {
	if g.rawScreenWidth != width || g.rawScreenHeight != height {
		g.rawScreenWidth = width
		g.rawScreenHeight = height

		scale := ebiten.Monitor().DeviceScaleFactor()

		g.screenWidth = int(float64(width) * scale)
		g.screenHeight = int(float64(height) * scale)

		if g.scene != nil {
			g.scene.OnLayoutChange(g.screenWidth, g.screenHeight)
		}
	}

	return g.screenWidth, g.screenHeight
}
```

`OnLayoutChange` sets `Dimensions.Updated` on the `Game` singleton; the `Dimensions`
system recomputes UI metrics and re-rasters fonts at the new size
(`system/dimensions.go` ends in `assets.UpdateFonts(fontSize)` —
see [fonts](fonts.md)). Crisp at any DPI; costs a resize pipeline.

*Fixed logical resolution* — `Layout` returns constants (1300×600,
`game/game.go`); ebiten scales the frame. Zero resize handling; blurry on hi-DPI.

*Identity passthrough* — `return outsideWidth, outsideHeight`
(`main.go`). Simplest; fine while the window is a fixed size.

## Rules

- Window size = monitor size × percent (`engine.IntPercent(w, 0.8)`), never absolute.
- Screen size lives in **one** place. Hardcoding 1024×900 in `main.go`, the `Game`
  component, the camera, and menu dialogs ends in `// TODO Keep screen size in one place`.
- Gate `Update`/`Draw` until the first real layout (`if g.rawScreenWidth == 0 { return }`)
  and until a scene exists.
- The loading screen is plain text — nothing loaded beyond fonts ([assets](assets.md)).
- A quick-start setting is standard equipment from day one ([dev-shortcuts](dev-shortcuts.md)).

## Pitfalls

- **Per-frame screenshot copies** — `g.lastFrame = ebiten.NewImageFromImage(screen)` at
  the end of `Draw` allocates a full screen copy every frame. Capture a screenshot only
  when one is asked for ([debug](debug.md)).
- **First-layout busy-wait** — an asset goroutine spinning
  `for { if g.screenWidth > 0 { break }; time.Sleep(time.Millisecond * 100) }` because
  scenes need real dimensions at construction. Let scenes accept 0×0 and get
  `OnLayoutChange` later, or defer the switch into `Update` ([scenes](scenes.md)). Never
  sleep-poll.

## Variations

- **Synchronous loading** (`assets.MustLoadAssets()` in `NewGame`), with a loading
  screen only for deferred scene builds — fine with few assets.
- **No loading step at all**: `assets.MustLoadFonts()`, then straight into the first
  scene — fine for a single embedded sheet.
- **An HTTP control server** behind a setting, for agent-driven testing: endpoints to
  send input and fetch a screenshot.
- **Mobile** builds construct the same `game.Game` via `mobile.SetGame(...)` with
  `Quick: true, Safe: true` ([mobile](mobile.md)).
