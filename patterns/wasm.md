# WASM & web deploy

## How it works

The browser build is the primary build. `web/` holds a two-file static shell; the game
compiles to `web/game.wasm` with the stock Go toolchain (no TinyGo, no cgo), and a push
to the default branch builds and deploys `web/` to GitHub Pages. The shell is the same
for every game.

## Canonical code

**Shell** — an iframe wrapper fixes the canvas size and centers it
(`web/index.html`):

```html
<!DOCTYPE html>
<style>
    body { background-color: #000000; }
    iframe { margin: 0 auto; display: block; border: 0; }
</style>
<body>
<iframe src="main.html" width="1024" height="900" allow="autoplay"></iframe>
</body>
```

`main.html` loads the runtime and instantiates, with the streaming polyfill
(`web/main.html`):

```html
<!DOCTYPE html>
<script src="wasm_exec.js"></script>
<script>
// Polyfill
if (!WebAssembly.instantiateStreaming) {
    WebAssembly.instantiateStreaming = async (resp, importObject) => {
        const source = await (await resp).arrayBuffer();
        return await WebAssembly.instantiate(source, importObject);
    };
}

const go = new Go();
WebAssembly.instantiateStreaming(fetch("game.wasm"), go.importObject).then(result => {
    go.run(result.instance);
});
</script>
```

**Build & run locally** (`CLAUDE.md`):

```
GOOS=js GOARCH=wasm go build -o web/game.wasm .
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
python3 -m http.server -d web 8080
```

`$(go env GOROOT)/lib/wasm/` is the **modern** wasm_exec.js path (Go 1.24+). The old
`misc/wasm/` path no longer exists; scripts and workflows that still use it break on
current Go.

**Wasm-first dev** — the wasm target is CGO-free, so it doubles as the compile/vet
check even when not shipping to the web. Native builds need Ebiten's C deps
(X11/OpenGL/ALSA headers), which dev containers usually lack; `GOOS=js GOARCH=wasm go
vet ./...` always works (`CLAUDE.md`).

**CI → GitHub Pages** (`.github/workflows/deploy-web.yml`):

```yaml
jobs:
  deploy:
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v2
        with:
          go-version: '^1.25'
      - run: go build -o web/game.wasm .
        env:
          GOOS: js
          GOARCH: wasm
      - run: cp $(go env GOROOT)/lib/wasm/wasm_exec.js web/

      - uses: actions/configure-pages@v2
      - uses: actions/upload-pages-artifact@v3
        with:
          path: web
      - id: deployment
        uses: actions/deploy-pages@v4
```

**Server deploy** (networked games) — the authoritative server gets a multi-stage
Dockerfile with BuildKit cache mounts and a stamped build
(`Dockerfile`):

```dockerfile
FROM golang:1.25 AS builder
ENV CGO_ENABLED=0
ARG BUILD_VERSION
COPY go.* /src
RUN --mount=type=cache,target=/gomod-cache go mod download
COPY . /src
RUN --mount=type=cache,target=/gomod-cache --mount=type=cache,target=/go-cache \
    go build -ldflags="-s -w -X main.BuildVersion=${BUILD_VERSION}" -trimpath -o /server ./server

FROM alpine
RUN apk add --no-cache ca-certificates
COPY --from=builder /server /server
CMD ["/server"]
```

`.github/workflows/deploy-server.yml` builds the image with
`--build-arg BUILD_VERSION=$(git rev-parse --short HEAD)` and deploys it to Cloud Run
(see [versioning.md](versioning.md) for the `-X` caveat).

## Rules

- `web/game.wasm` and `web/wasm_exec.js` are build artifacts: gitignored, rebuilt by CI.
  A `game.wasm` runs to 20+ MB — never commit it.
- One shell, unchanged across games; only the iframe dimensions vary.
- Anything platform-specific goes behind a build-tag pair (`storage_wasm.go` /
  `storage_native.go`, `connection_wasm.go` / `connection_native.go`) — never
  `runtime.GOOS` checks. Persistence on the web is localStorage via that pair (see
  [save-load.md](save-load.md)); browser networking is `syscall/js` WebSocket via the
  same trick (see [networking.md](networking.md)).
- Every file the wasm binary needs is `//go:embed`ed — the shell serves exactly three
  files.

## Pitfalls

- Use `lib/wasm/wasm_exec.js`, not `misc/wasm/wasm_exec.js`, in every script and
  workflow.
- A game whose native-only code isn't behind build tags won't compile for js at all
  (`storage.NewStorage` called unconditionally; see [save-load.md](save-load.md)). Keep the wasm build green from day one; it is the CI
  gate.
