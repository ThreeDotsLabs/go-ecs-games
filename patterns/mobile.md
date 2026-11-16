# Mobile

## How it works

`ebitenmobile bind` compiles the game into a native library (`.xcframework` for iOS,
`.aar` for Android) that a **checked-in host project** (Xcode / Gradle) embeds. The Go
side is a tiny `package mobile` that registers the game in `init()`; everything else is
the same game, with platform differences behind build tags.

## Canonical code

**The mobile package** — `mobile/main.go`, `package mobile`, one `init()`, one exported
symbol so gomobile has something to bind (`mobile/main.go`):

```go
package mobile

import (
    "github.com/hajimehoshi/ebiten/v2/mobile"

    "github.com/you/mygame/game"
)

func init() {
    mobile.SetGame(game.NewGame(game.Config{
        StartScene:   "",
        ScreenWidth:  1300,
        ScreenHeight: 600,
        SampleRate:   44100,
    }))
}

func Dummy() {}
```

The mobile config's flags worth having (`mobile/main.go`):

```go
func init() {
    mobile.SetGame(game.NewGame(game.Config{
        Quick: true,  // skip the title scene straight into the game
        Safe:  true,  // recover() around Update/Draw, render the panic on screen
    }))
}
```

`Safe` matters on device, where there is no terminal: `game.Update`/`Draw` defer a
`recover()` that appends the panic to the loading-lines overlay instead of dying
silently (`game/game.go` — see [bootstrap.md](bootstrap.md)).

**Build tasks** (`Taskfile.yml`):

```yaml
mobile:
  deps:
    - task: store-version
  dir: ./mobile
  cmds:
    - ebitenmobile bind -target ios -o mygame.xcframework .

mobile-deploy:
  deps:
    - mobile
  dir: ./mobile/MyGame
  cmds:
    - xcodebuild -scheme MyGame -configuration Debug -sdk iphoneos -derivedDataPath build
    - ios-deploy --bundle build/Build/Products/Debug-iphoneos/MyGame.app --nostart --no-wifi

mobile-android:
  deps:
    - task: store-version
  dir: ./mobile
  env:
    ANDROID_HOME: "{{.HOME}}/Library/Android/sdk"
  cmds:
    - PATH="/Applications/Android Studio.app/Contents/jbr/Contents/Home/bin:{{.PATH}}"
      ebitenmobile bind -target android -androidapi 21 -javapkg com.example.mygame -o MyGame-android/libs/mygame.aar .
```

The host projects are committed: `mobile/MyGame/` (Xcode) and `mobile/MyGame-android/`
(Gradle, the `.aar` dropped into `libs/`). Note `store-version` runs before every bind
(see [versioning.md](versioning.md)).

**Platform save directory** — build-tagged, with a cgo Objective-C bridge on iOS
(`storage/storage_dir_ios.go`; implementation
`mobile/MyGame/MyGame/ApplicationSupportDirectory.m` returns
`NSApplicationSupportDirectory` and creates it) — details in
[save-load.md](save-load.md).

**Touch controls** — a capability flag as a build-tagged function pair, branched on
inside the input system (`system/controls_mobile.go` / `controls_desktop.go`):

```go
//go:build android || ios

package system

func UseTouchControls() bool {
    return true
}
```

The touch scheme itself (tap vs drag, crosshair confirm) is in
[input-touch.md](input-touch.md).

## Checklist for a new mobile target

1. Create `mobile/main.go` as above; verify `ebitenmobile bind -target ios -o
   X.xcframework .` runs from `mobile/`.
2. iOS host project (from `README.md`): new Xcode project (Game, SpriteKit);
   add the game `.xcframework` and `GameController.framework` as frameworks; change
   `GameViewController` to use `MobileEbitenViewController`. Commit the project.
3. Android: `-target android -androidapi 21 -javapkg <pkg>` into the Gradle project's
   `libs/`; commit the Gradle host project.
4. Wire platform save dirs and `UseTouchControls()` build-tag pairs; pass
   `Quick`/`Safe`-style config from the mobile `init()`.
5. Deploy: `xcodebuild` + `ios-deploy` (the `mobile-deploy` task) or open the Gradle
   project in Android Studio.

## Rules

- The mobile package contains configuration only — no logic. Same `game.NewGame` as
  desktop, different `Config`.
- Ship `Safe: true` on mobile; keep it off on desktop where the stack trace is worth
  more than surviving the frame.
- Host projects and generated frameworks: host projects are committed; bind outputs
  (`*.aar`, `mobile/<X>.xcframework`) are gitignored.
- Everything platform-specific is a build-tag pair returning data or a small function —
  systems never import platform packages.

## Gotchas (README-documented)

- Slow app launch from Xcode: Cmd+Shift+, → disable "Debug executable"
  (`README.md`).
- Android needs Android Studio's JDK on `PATH` and `ANDROID_HOME` set (the
  `mobile-android` task hardcodes both).
- Don't hardcode the Android save path (`/data/data/com.example.mygame`) — resolve it
  from the activity context.
- You can also bundle the desktop build as a macOS `.app` with a hand-rolled
  Info.plist (`dev/make-app.sh`) — the cheap path to a
  double-clickable Mac build, unrelated to `ebitenmobile`.
