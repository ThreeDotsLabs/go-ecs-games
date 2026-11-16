# Asset loading & validation

## How it works

Every asset is `//go:embed`ded and loaded **once, at startup, through one entry point**
(`assets.LoadAssets` / `MustLoadAssets`). The inventory is a typed manifest struct in
`domain/`: category structs whose fields are `*ebiten.Image` (sprites) or `[]byte` (decoded
audio), each tagged with its file path. `assets/loader/` walks that manifest with reflection
and fills it from the embedded FS — the same struct is the loading manifest *and* the
type-safe access API (`assets.Assets.Sprites.Items.Bread`: compile-time checked, no string
keys). A validation pass then errors on nil images and empty byte slices. **Assets are
validated, then trusted** — this is why the rest of the game legitimately skips runtime
nil-checks. Adding an asset = dropping the file into `assets/` and adding one tagged field.

## Canonical code

**Manifest** — plain data in `domain/`; tags carry the directory per category and the file
per field; `scale` sets a per-category default that individual fields can override; audio
categories are the same shape with `[]byte` fields (`domain/assets.go`, trimmed):

```go
type Sprites struct {
    Items  ItemSprites  `path:"items" scale:"true"`
    Emotes EmoteSprites `path:"emotes" scale:"false"`
    // … Player, Objects, Decorations, NPCs, UI
}

type ItemSprites struct {
    Bread  *ebiten.Image `path:"bread.png"`
    Coffee *ebiten.Image `path:"coffee.png"`
    // … one field per file
}
```

**Embed + entry point** — one embedded FS, one package-level `Assets` pointer, a
double-load guard (`assets/assets.go`):

```go
var (
    //go:embed level/* sounds/* items/* emotes/* objects/* decorations/* player/* npc/* ui/* *.tsx
    gameAssetsFS embed.FS

    Assets *domain.Assets
)

func LoadAssets(progressChan chan<- string, errorChan chan<- error) {
    if Assets != nil {
        errorChan <- errors.New("assets already loaded")
        return
    }
    assets, err := loader.LoadAssets(gameAssetsFS, progressChan)
    if err != nil {
        errorChan <- err
        return
    }
    Assets = assets
    errorChan <- nil
}
```

Fonts load separately and synchronously — `assets.MustLoadFonts()` runs before the async
load starts, because the loading screen itself needs a font ([fonts.md](fonts.md)).

**The reflection loop** — one mechanism for every sprite category
(`assets/loader/loader.go`):

```go
func loadSpritesUsingReflection(assetsFS fs.FS, target any, basePath string, defaultScale bool) error {
    v := reflect.ValueOf(target)
    if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
        return fmt.Errorf("target must be a pointer to struct")
    }

    structValue := v.Elem()
    structType := structValue.Type()

    for i := 0; i < structValue.NumField(); i++ {
        field := structType.Field(i)
        fieldValue := structValue.Field(i)

        if !fieldValue.CanSet() {
            continue
        }

        pathTag := field.Tag.Get("path")
        if pathTag == "" {
            continue
        }

        // A per-field `scale:"false"` overrides the category default.
        shouldScale := defaultScale
        if scaleTag := field.Tag.Get("scale"); scaleTag != "" {
            var err error
            shouldScale, err = strconv.ParseBool(scaleTag)
            if err != nil {
                return fmt.Errorf("invalid scale tag for field %s: %v", field.Name, err)
            }
        }

        fullPath := path.Join(basePath, pathTag)
        data, err := fs.ReadFile(assetsFS, fullPath)
        if err != nil {
            return fmt.Errorf("failed to load asset %s for field %s: %v", fullPath, field.Name, err)
        }

        var img *ebiten.Image
        if shouldScale {
            img, err = newScaledImageFromBytes(data)
        } else {
            img, err = newImageFromBytes(data)
        }
        if err != nil {
            return fmt.Errorf("failed to create image from %s for field %s: %v", fullPath, field.Name, err)
        }

        fieldValue.Set(reflect.ValueOf(img))
    }

    return nil
}
```

**Upscale once, at load** — nearest-neighbor, so pixel art stays crisp:

```go
func newScaledImageFromBytes(data []byte) (*ebiten.Image, error) {
    img, _, err := image.Decode(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    original := ebiten.NewImageFromImage(img)

    bounds := original.Bounds()
    scaled := ebiten.NewImage(bounds.Dx()*domain.SpriteScale, bounds.Dy()*domain.SpriteScale)

    opts := &ebiten.DrawImageOptions{}
    opts.GeoM.Scale(float64(domain.SpriteScale), float64(domain.SpriteScale))
    opts.Filter = ebiten.FilterNearest
    scaled.DrawImage(original, opts)

    return scaled, nil
}
```

Audio uses the same walk (`loadAudioUsingReflection`); MP3s are decoded **at load time** to
raw PCM bytes (`mp3.DecodeWithoutResampling` + `io.ReadAll`), so playback is just a
`bytes.Reader` with no per-play decode ([audio.md](audio.md)). On the image side,
`newScaledImageFromBytes` upscales 16px art once, nearest-neighbor (`ebiten.FilterNearest`),
by `domain.SpriteScale` (`GridSize / SpriteSize`, e.g. 8×) — draw code never scales.

**Validation** — reflect over everything, fail startup loudly (core of
`validateStructAssets`, same file):

```go
if field.Type.String() == "*github.com/hajimehoshi/ebiten/v2.Image" {
    if fieldValue.IsNil() {
        return fmt.Errorf("required asset %s.%s is nil", categoryName, field.Name)
    }
} else if field.Type.String() == "[]uint8" { // []byte is []uint8 internally
    if fieldValue.Len() == 0 {
        return fmt.Errorf("required asset %s.%s is empty", categoryName, field.Name)
    }
}
```

## Progress, lazy backgrounds, settings

The loader streams progress strings (`progressChan <- fmt.Sprintf("Loading level %v", name)`);
the bootstrap collects them into a text loading screen shown until the first scene exists
(`game/game.go`; the handoff is [bootstrap.md](bootstrap.md)):

```go
go assets.LoadAssets(progressChan, errorChan)
go func() {
    for line := range progressChan {
        g.loadingLines = append(g.loadingLines, fmt.Sprintf("%v...", line))
    }
}()
```

**Lazy level backgrounds** — the one sanctioned exception to load-everything-up-front:
full-screen webp backgrounds are big, so `domain.Level.Background` is a `func() *ebiten.Image`
closure called on level entry. The loader still `fs.Stat`s the path at startup, so a missing
file fails the load, not the level switch (`assets/loader/loader.go`):

```go
loadBackground := func() *ebiten.Image {
    bgBytes, err := fs.ReadFile(assetsFS, bgPath)
    background, err := newImageFromBytes(bgBytes) // err checks trimmed
    return background
}
```

**Tunables ship as an asset** — `settings.toml` decodes via `BurntSushi/toml` straight into
a `domain.Settings` struct (`assets/loader/loader.go`):
`toml.DecodeFS(assetsFS, "game/settings.toml", &settings)`.

**Cross-reference validation** — `validateAssets` goes beyond nil checks: every
passage a POI references exists in the story, every target level and entrypoint index
exists, every music track a macro plays exists, every required item/fact is obtainable.
Catching a dangling reference at startup, not mid-playthrough, is exactly right. Keep it
covered by a test that feeds it broken data, so an early `return nil` left in while
debugging can't silently turn the whole pass into dead code.

## Rules

- All loading happens at startup through the single entry point; no decodes, embeds, or
  file reads outside `assets/`.
- The manifest struct lives in `domain/`, the loader in `assets/loader/`; `assets/` holds
  no game logic.
- Validate at startup, then trust: never nil-check an asset at runtime.
- Access assets by typed field (`Assets.Sprites.Items.Bread`), never by string key.
- Adding an asset = one file + one tagged field. If it can dangle (a level name, a
  passage), extend the cross-reference validation to cover it.

## Pitfalls

- A `domain.Settings` with toml tags whose loader hardcodes the values in Go defeats the
  point — decode it with `toml.DecodeFS`.
- Keep loader caches (`levelNames`, `characterCollider`, `tilesetCache`) in a loader
  struct, not package globals.

## Variations

- **No loader machinery**: one packed sheet embedded and sliced into a named catalog
  ([tile-catalog.md](tile-catalog.md)) — right-sized for one sheet, no audio.
- **Synchronous `MustLoadAssets()`** that panics — workable without a loading screen;
  prefer the error-channel form.
- **Every sprite generated in code** ([placeholder-art.md](placeholder-art.md)).
- TMX levels load inside the same pipeline (`loadLevels`) — [tiled.md](tiled.md).
