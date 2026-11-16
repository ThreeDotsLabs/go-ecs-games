# Scenes & switching

## How it works

A scene is the unit of game state. Each scene owns its own `donburi.World` plus the
hand-ordered system/drawable slices (see [ecs-core](ecs-core.md)); `main.go` is a thin
`ebiten.Game` that holds only the current scene and delegates `Update`/`Draw`.
Switching is **pull-based**: main polls `NextScene()` every tick and swaps on non-nil.
A switch constructs the new scene (fresh world) and drops the old one — nothing survives
except what is passed through constructors.

## Canonical code

**The interface** — declared once in `scene/scene.go`. The base form
(`scene/scene.go`):

```go
type Scene interface {
	Update()
	Draw(screen *ebiten.Image)
	NextScene() Scene
}
```

A networked game adds `HandleMessage(msg client.Message)`: main
pumps one server message per tick into the scene before updating it
(`main.go`). Only add it when there is a network ([networking](networking.md)).

**The switch loop** — main polls, the scene decides (`main.go`):

```go
func (g *Game) Update() error {
	if g.currentScene != nil {
		g.currentScene.Update()

		nextScene := g.currentScene.NextScene()
		if nextScene != nil {
			g.currentScene = nextScene
		}
	}

	return nil
}
```

A scene triggers the switch by setting a field, usually from a button callback —
`m.nextScene = NewGame(m.client, nil, nil)` — and `NextScene()` just returns that
field (`scene/menu.go`).

**Menu scenes may have no world.** A `Menu` that is pure ebitenui — a widget tree,
zero donburi ([ui-ebitenui](ui-ebitenui.md)). Don't force ECS onto a screen of widgets.

**Restart is a world rebuild** — `createWorld()` is re-callable (`scene/game.go`):

```go
	// Press R to rebuild the world from scratch — handy while iterating.
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.createWorld()
		g.init()
	}
```

**Scene context** — when scenes need shared services, pass one value, not globals
(`scene/scene.go`):

```go
type Context struct {
	Version string

	ScreenWidth  int
	ScreenHeight int

	Storage       Storage
	SceneSwitcher Switcher

	SetMusic func(bool)
}

type Switcher interface {
	SwitchToMainMenu()
	SwitchToNewGame(saveSlot int, character domain.PlayerCharacterType)
	SwitchToLoadedGame(saveSlot int)
	SwitchToReplayedGame(saveSlot int)
	SwitchToTutorial()
}
```

`game.Game` implements `Switcher` — the push-based variant: scenes call
`context.SceneSwitcher.SwitchToMainMenu()` instead of returning a next scene.

**Deferred switching** — store a constructor and builds on the *next*
`Update`; while `scene == nil`, `Draw` shows a pre-rendered `loadingScreen` image, so
heavy world generation hides behind "Loading..." (`game/game.go`):

```go
func (g *Game) switchScene(newSceneFunc func() Scene) {
	g.scene = nil
	g.nextSceneFunc = newSceneFunc
}

func (g *Game) Update() error {
	if g.nextSceneFunc != nil {
		g.scene = g.nextSceneFunc()
		g.nextSceneFunc = nil
	}
	// ...
}
```

**One scene, many entry points** — new game / load / replay / tutorial all
build the same `Board` scene through a strategy interface; each creator fills the world
its own way and delegates shared steps — the replay creator embeds the new-game one
(`scene/board.go`):

```go
type boardCreator interface {
	ExtraSystems() []System

	CreateActions(w donburi.World)
	CreatePlayer(w donburi.World)
	CreateBoard(w donburi.World)
	// ... CreateProgress, CreateResearch, CreateResources, CreateUI ...

	InitUnits(w donburi.World)
	InitCamera(w donburi.World)
	InitExtra(w donburi.World)
}
```

Construction is two-phase; the ordering comment is load-bearing (`scene/board.go`):

```go
	b.world = b.createWorld()

	creator.CreateActions(b.world)
	// ... other Create* steps ...
	creator.CreateUI(b.world)

	// Init initializes all systems, so they subscribe to events
	// Any setup that publishes events should be done after this point
	b.init()

	creator.InitUnits(b.world)
	creator.InitCamera(b.world)
```

## Rules

- One world per scene; never share a world across scenes. Restart = re-run
  `createWorld()` + `init()` inside the same scene.
- System `Init` (event subscriptions) runs **before** any setup that publishes
  [events](events.md) — split setup into pre-init and post-init phases when needed.
- In the pull model, `NextScene()` is the only switch channel — a scene never mutates
  the outer `Game`.
- Shared services (storage, switcher, screen size, audio toggle) travel in one
  `scene.Context` value passed to every scene constructor.
- Heavy scene construction (procgen) is deferred one frame behind a loading screen.
- Several entry points into one scene = one scene + creator strategies, not N copies.

## Pitfalls

- `HandleMessage` and the `client.Message` dependency belong only to networked games;
  single-player games keep the three-method base interface.
- Several `NewBoard*` constructors that each repeat `LoadGlobalData` + error handling
  drift apart; fold the shared part into one `newBoard`.

## Variations

- **Constructor-callback switching** — `game.Game` passes its own
  switch methods into scenes (`scene.NewGame(screenWidth, screenHeight,
  g.switchToTitle)`) and Scene is `{Update; Draw; OnLayoutChange(width, height int)}` —
  the resize-aware variant ([bootstrap](bootstrap.md)).
- `Init` can be required on every System/Drawable (interfaces embed `Initializable`) or
  optional via type assertion — prefer optional.
- Push (`Switcher` + `Context`) and pull (`NextScene`) both work; pick one per game,
  never both.
