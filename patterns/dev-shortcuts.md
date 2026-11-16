# Dev shortcuts

## How it works

Every second between `go run` and "the thing I'm testing" compounds. The shortcuts that
pay for themselves: boot straight into any scene, skip menus with defaults for every
choice they collect, rebuild the world with a key, toggle fullscreen with a key. They
are wired in `main.go` / the top-level game, not in systems — they must work before any
world or singleton exists.

Env vars and one-off flags (`START_SCENE=…`, `-quick`) work for the first two hooks and
stop scaling around the tenth. Give every shortcut a row in **one settings table**
instead, and shape a run with `group.name=value` args:

```sh
go run . help                                        # every setting, grouped
go run . client.scene=fitting_room                   # boot into a dev scene
go run . preset=quick run.screenshot=out.png         # a named scenario plus one more key
```

The prefix says who reads a setting: `world.` the game state, `client.` this player's
scene, `net.` how the game connects, `run.` the process (screenshots, tick limits,
profiles — see [profiling](profiling.md)). Keep `os.Getenv` for what the platform sets
(a container's `PORT`) and nothing else.

## Canonical code

**The table** (`boot/settings.go`). One row per shortcut: key, accepted values, help, an
example that parses, and whether it can act on a running game. `Set` validates, so a
typo fails the run instead of silently booting the menu:

```go
// When says where a row can act.
type When int

const (
    BootOnly When = iota // a fact about a fresh world: changing it means a restart
    Live                 // also acts on a running game, from the DEV panel
)

// A Setting is one row: a key, how to read it, what it does.
type Setting struct {
    Key     string // group.name; the prefix is the group
    Values  string // what it takes, for help: "menu | game | fitting_room"
    Help    string
    Example string // a value that parses, shown in help and checked by a test
    When    When
    // Set reads the value into Settings. A bad value is an error, so a typo
    // fails the run instead of booting the wrong thing.
    Set func(s *Settings, v string) error
}

// Settings is everything the table can set, read by main.go and the scenes.
type Settings struct {
    Scene      string
    Seed       int64
    Screenshot string
    Ticks      int
}

var scenes = []string{"menu", "game", "fitting_room"}

var Table = []Setting{
    {Key: "client.scene", Values: strings.Join(scenes, " | "), Help: "boot straight into a scene", Example: "fitting_room",
        Set: func(s *Settings, v string) error {
            if !slices.Contains(scenes, v) {
                return fmt.Errorf("unknown scene %q (%s)", v, strings.Join(scenes, ", "))
            }
            s.Scene = v
            return nil
        }},
    {Key: "world.seed", Values: "an integer", Help: "seed the world generator", Example: "42",
        Set: func(s *Settings, v string) error {
            _, err := fmt.Sscan(v, &s.Seed)
            return err
        }},
    {Key: "run.screenshot", Values: "a .png path", Help: "save a screenshot after run.ticks and exit", Example: "out.png",
        Set: func(s *Settings, v string) error { s.Screenshot = v; return nil }},
}

// Presets are named scenarios; "quick" is the old -quick flag.
var Presets = map[string]string{
    "quick": "client.scene=game world.seed=1",
}
```

**Parsing** — one pair per arg, or several in one arg (a pasted line); a bare key means
`on`; `preset=name` expands in place, later keys win:

```go
func Parse(args []string) (Settings, error) {
    s := Settings{Scene: "menu", Ticks: 60}
    var apply func(pair string) error
    apply = func(pair string) error {
        key, value, ok := strings.Cut(pair, "=")
        if !ok {
            value = "on"
        }
        if key == "preset" {
            line, found := Presets[value]
            if !found {
                return fmt.Errorf("unknown preset %q", value)
            }
            for _, p := range strings.Fields(line) {
                if err := apply(p); err != nil {
                    return fmt.Errorf("preset %s: %w", value, err)
                }
            }
            return nil
        }
        i := slices.IndexFunc(Table, func(row Setting) bool { return row.Key == key })
        if i < 0 {
            return fmt.Errorf("unknown setting %q (go run . help)", key)
        }
        if err := Table[i].Set(&s, value); err != nil {
            return fmt.Errorf("%s: %w", key, err)
        }
        return nil
    }
    for _, arg := range args {
        for _, pair := range strings.Fields(arg) {
            if err := apply(pair); err != nil {
                return Settings{}, err
            }
        }
    }
    return s, nil
}
```

**The table tests itself** (`boot/settings_test.go`) — every row has help and a group
prefix, every example and preset parses, and a typo fails:

```go
func TestTable(t *testing.T) {
    for _, row := range Table {
        if row.Help == "" {
            t.Errorf("%s: no help", row.Key)
        }
        if !strings.Contains(row.Key, ".") {
            t.Errorf("%s: no group prefix", row.Key)
        }
        if _, err := Parse([]string{row.Key + "=" + row.Example}); err != nil {
            t.Errorf("%s: example: %v", row.Key, err)
        }
    }
    for name := range Presets {
        if _, err := Parse([]string{"preset=" + name}); err != nil {
            t.Errorf("preset %s: %v", name, err)
        }
    }
    if _, err := Parse([]string{"client.scene=fiting_room"}); err == nil {
        t.Error("a typo must fail")
    }
}
```

**Booting into a scene** — `main.go` switches on `Settings.Scene` through the scene
registry (see [scenes](scenes.md)). Skipping a menu means supplying a default for every
choice that menu collects:

```go
switch settings.Scene {
case "menu":
    g.switchToMainMenu()
case "game":
    g.switchToGame([]system.ChosenPlayer{
        {PlayerNumber: 1, Faction: 0},
    })
case "fitting_room":
    g.switchToFittingRoom()
}
```

A mobile build can ship the quick path hardcoded (`mobile/main.go` boots
`preset=quick`), since it has no command line.

**`R` — rebuild the world mid-game** (`scene/game.go`):

```go
// Press R to rebuild the world from scratch — handy while iterating.
if inpututil.IsKeyJustPressed(ebiten.KeyR) {
    g.createWorld()
    g.init()
}
```

**`F` — fullscreen toggle** (`game/game.go`):

```go
if inpututil.IsKeyJustPressed(ebiten.KeyF) {
    ebiten.SetFullscreen(!ebiten.IsFullscreen())
}
```

## A scenario is a line of text

The same line works everywhere, so a bug report is a command anyone can run:

- the command line: `go run . preset=quick world.seed=7`;
- the browser: the page's query string carries the same pairs
  (`main.html?client.scene=game&world.seed=7`), passed to Go as args
  ([wasm](wasm.md));
- a headless server: `go run ./cmd/server world.seed=7` starts every room from it;
- the in-game **DEV panel** (F10): it lists the `Live` rows as buttons, applies them to
  the running game, and writes the current scenario back as one line to copy.

**Boot, live, or both.** Each row says when it can act. Rows that are actions on the
game state (grant an item, open a breach) are `Live` and run the same function at boot
and from the panel. Rows that describe a moment rather than an action (the start
position, a seed) are `BootOnly`.

**Cheats are local single player only.** Live rows act through a cheat handle that only
a solo game gets; networked games never do. The handle runs the row in-process against
the authoritative state, so nothing goes over the wire and servers have no dev mode.
**Restart** from the panel rebuilds from the current scenario.

## Dev scenes

Build browsers and editors as **ordinary scenes**, then `client.scene` makes them one
command away. A fitting room (`scene/fitting_room.go`) is the model: a paper-doll editor
cycling every hair/head/chest/legs/weapon sprite in the catalog — an asset browser inside
the game, sharing its loaders and renderer. Dev scenes can stay out of the shipped menu;
the setting still reaches them. The bigger sibling is a second binary (`cmd/sandbox`)
around a scene for staring at map generation with altitude/temperature/humidity
overlays (see [procgen](procgen.md)) — a separate `cmd/` entrypoint only when the tool
needs its own window and config; otherwise a scene is cheaper.

## Rules

- Every dev shortcut is a row in the table. Adding one means adding a row (and a preset
  if it is a common starting point), never touching `main.go` call sites or adding an
  `os.Getenv`.
- Unknown keys and bad values fail the run at boot, or show on the panel when live.
- Wire shortcuts at the bootstrap level ([bootstrap](bootstrap.md)), gated on nothing.
- Every menu the game grows keeps `preset=quick` honest: quick mode supplies a default
  for each choice the menu collects.
- Cheats change state through the same functions and events as real play
  ([debug](debug.md)).

## Pitfalls

- A boot hook with nothing registered under it (only the empty value handled) is dead
  weight. Add a dev scene or delete the row.
- Don't give servers a dev message type for cheats; a networked cheat path is a
  security hole and a second code path to keep in sync.
