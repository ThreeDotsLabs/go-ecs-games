# Audio

## How it works

One `audio.Context` for the whole process, created at game level before any scene:
`audio.NewContext(44100)` (`game/game.go`, `game/game.go`).
Sounds are decoded **once at load** into raw PCM `[]byte`; players are cheap
`ctx.NewPlayerFromBytes` wrappers. SFX is a pure **event-listener system**: it subscribes
in `Init`, its `Update` is empty (or music-only) — gameplay never calls audio directly, it
publishes events ([events.md](events.md)).

## Canonical code

**Pre-decode at load** (`assets/loader/loader.go`):

```go
func loadMP3Stream(fileSystem fs.FS, path string) ([]byte, error) {
    data, err := fs.ReadFile(fileSystem, path)
    if err != nil {
        return nil, err
    }
    stream, err := mp3.DecodeWithoutResampling(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    return io.ReadAll(stream)
}
```

**The SFX system — subscribe, don't poll** (`system/audio.go`):

```go
func NewAudio() *Audio {
    ctx := audio.CurrentContext()
    return &Audio{
        audioContext: ctx,
        click1Player: ctx.NewPlayerFromBytes(assets.Click1Sound),
        // ... one preloaded *audio.Player per sound
    }
}

func (a *Audio) Init(w donburi.World) {
    events.RoadSpawnedEvent.Subscribe(w, a.onRoadSpawned)
    events.OriginSettlementSpawnedEvent.Subscribe(w, a.onOriginSettlementSpawned)
    events.SettlementSpawnedEvent.Subscribe(w, a.onSettlementSpawned)
    events.SettlementUpgradedEvent.Subscribe(w, a.onSettlementUpgraded)
    events.ButtonClickedEvent.Subscribe(w, a.onButtonClicked)
    events.GameOverEvent.Subscribe(w, a.onGameOver)
    events.GameWonEvent.Subscribe(w, a.onGameWon)
}

func (a *Audio) Update(w donburi.World) {}

func (a *Audio) onRoadSpawned(w donburi.World, event events.RoadSpawned) {
    _ = a.click2Player.Rewind()
    a.click2Player.Play()
}
```

`Rewind()+Play()` is the replay idiom. Handlers can branch on payload — settlement level
2–5 picks chime 1–4 (`onSettlementUpgraded`), and UI clicks respect the button's declared
sound (`ButtonData.Sound`, [ui-ecs.md](ui-ecs.md)):

```go
func (a *Audio) onButtonClicked(w donburi.World, event events.ButtonClicked) {
    switch event.Button.Sound {
    case component.ButtonSoundNone:
    case component.ButtonSoundDefault:
        _ = a.click1Player.Rewind()
        a.click1Player.Play()
    }
}
```

The same shape scales to 17 event subscriptions with an audio-only `Update`.

**Music, three strategies:**

1. *Manual loop* — `if !player.IsPlaying() { Rewind(); Play() }` each tick
   (`game/game.go`, `Update`).
2. *`audio.NewInfiniteLoop`* for seamless held tones (an oven beep,
   `system/audio.go`): wrap the PCM bytes, `ctx.NewPlayer(loop)`.
3. *Cross-fade between tracks* — 3s `Timer` + eased complementary volumes, driven by a
   `MusicChangedEvent` (`system/audio.go`):

```go
func (a *Audio) Update(w donburi.World) {
    if a.mixInProgress {
        a.musicMixTimer.Update()
        if a.musicMixTimer.IsReady() {
            if a.fadingMusicPlayer != nil {
                a.fadingMusicPlayer.Pause()
                _ = a.fadingMusicPlayer.Close()
                a.fadingMusicPlayer = nil
            }
            a.mixInProgress = false
        } else {
            value := engine.EaseInOut(a.musicMixTimer.PercentDone())
            if a.fadingMusicPlayer != nil {
                a.fadingMusicPlayer.SetVolume(1 - value)
            }
            if a.currentMusicPlayer != nil {
                a.currentMusicPlayer.SetVolume(value)
            }
        }
    } else if a.currentMusicPlayer != nil && !a.currentMusicPlayer.IsPlaying() {
        _ = a.currentMusicPlayer.Rewind()   // loop current track
        a.currentMusicPlayer.Play()
    }
}
```

`mix(newTrack)` moves `currentMusicPlayer` to `fadingMusicPlayer`, starts the new track at
volume 0 (`""` = fade to silence). Publish `MusicChangedEvent` from Twine
`PlayMusic` macros (`domain/story.go`) — content drives music ([dialogue.md](dialogue.md)).

**Teardown** — the audio system implements the optional `Stopper` interface; scene switch
calls it so OS audio players close ([ecs-core.md](ecs-core.md);
`system/audio.go`, invoked in `scene/game.go switchToTitle`):

```go
func (a *Audio) Stop(w donburi.World) {
    if a.currentMusicPlayer != nil {
        a.currentMusicPlayer.Pause()
        _ = a.currentMusicPlayer.Close()
        a.currentMusicPlayer = nil
    }
    // ... same for fadingMusicPlayer
}
```

**Mute toggle across scenes** — keep the theme player at game level and pass
`SetMusic func(bool)` down through `scene.Context`; the menu's music button just calls
`m.context.SetMusic(musicOn)` (`game/game.go`, `scene/main_menu.go`).

## Rules

- Exactly one `audio.Context`, created in game bootstrap ([bootstrap.md](bootstrap.md)) —
  systems grab it with `audio.CurrentContext()`.
- Decode to PCM at load time; never construct decoders per play.
- SFX system: subscriptions in `Init`, empty `Update`, one preloaded player per sound,
  `Rewind()+Play()` to fire. New sound = new event + handler, not a call site in gameplay.
- Music state (current track, fade timers) lives in the audio system — the one sanctioned
  exception to "systems hold no state", since players are OS resources, not game state.
  That is exactly why it must implement `Stopper`.

## Pitfalls

- **No volume settings UI, no mixer/bus, no pitch or pan variation** — README staples,
  design fresh when first needed. Repeated SFX firing at identical pitch
  is the first thing to fix in a real game.
- Don't ship a cross-fade wired to an empty `MusicTracks` struct with nothing
  publishing `MusicChangedEvent` — dead wiring reads as a working feature.
- `audio.NewInfiniteLoop`'s length is the PCM data length in bytes; with raw PCM decoded
  at load, that is `len(pcmBytes)`.
