# Profiling

The question it answers: *which system is eating the frame?* (For a raw CPU profile,
see [debug](debug.md).)

## How it works

`engine.Perf` is a global profiler, like `engine.TimeMultiplier` is a global clock.
The scene brackets every system's `Update` and every drawable's `Draw` in a **span**
named after the type (`scene/game.go`), plus one for the event queue, and opens a
**phase** around each half of the frame so the Update and Draw totals are measured
too. A span is two clock reads; sampling is always on, headless tests included.

Each span keeps a ring of the last `engine.DefaultPerfWindow` (120) ticks for the
live view, and lifetime totals since the last `Reset` for the batch report. Once a
second the profiler reads `runtime.MemStats`: heap, allocation rate, GC count and
pause. A `Snapshot()` sorts the rows by cost and the same snapshot feeds the panel
and the report, so what you see in the window is what the batch run prints.

## Canonical code

**Wiring in the scene** (`scene/game.go`) — span names are read once, at construction;
`Update` and `Draw` build no strings:

```go
// spanName is the system's type name: *system.Render → "Render".
func spanName(v any) string {
    t := reflect.TypeOf(v)
    for t.Kind() == reflect.Pointer {
        t = t.Elem()
    }
    return t.Name()
}

// in the constructor
for _, s := range g.systems {
    g.systemNames = append(g.systemNames, spanName(s))
}
for _, d := range g.drawables {
    g.drawableNames = append(g.drawableNames, spanName(d))
}

func (g *Game) Update() {
    engine.Perf.BeginPhase(engine.PhaseUpdate)
    defer engine.Perf.EndPhase()

    for i, sys := range g.systems {
        span := engine.Perf.Start(g.systemNames[i])
        sys.Update(g.world)
        span.End()
    }

    span := engine.Perf.Start("Events")
    events.ProcessAllEvents(g.world)
    span.End()
}

func (g *Game) Draw(screen *ebiten.Image) {
    engine.Perf.BeginPhase(engine.PhaseDraw)
    defer engine.Perf.EndPhase()

    for i, drawable := range g.drawables {
        span := engine.Perf.Start(g.drawableNames[i])
        drawable.Draw(g.world, screen)
        span.End()
    }
}
```

**Spans** (`engine/profiler.go`) — a span adds to its section's pending time for this
tick; `EndPhase` commits every pending section into its ring. A name reused within a
tick adds up:

```go
// Span is an open measurement; End closes it.
type Span struct {
    p       *Profiler
    section *section
    started time.Time
}

// Start opens a span under the current phase. Outside a phase (a system
// driven directly by a test) the span is filed under Update and commits
// on End.
func (p *Profiler) Start(name string) Span {
    ph := p.current
    if !p.inPhase {
        ph = PhaseUpdate
    }
    s, ok := p.byName[ph][name]
    if !ok {
        s = newSection(name, ph, p.window)
        p.byName[ph][name] = s
        p.sections[ph] = append(p.sections[ph], s)
    }
    return Span{p: p, section: s, started: time.Now()}
}

// End closes the span and adds its time to this tick's sample.
func (sp Span) End() {
    sp.section.pending += time.Since(sp.started)
    if !sp.p.inPhase {
        sp.section.commit(sp.section.pending)
        sp.section.pending = 0
    }
}
```

**A section** keeps the window ring for the live view and lifetime totals for the
report:

```go
type section struct {
    name  string
    phase Phase

    samples []time.Duration // ring of the last window ticks
    head    int
    filled  int

    pending time.Duration // this tick's spans, committed at phase end
    last    time.Duration

    runTotal time.Duration
    runCount int
    runMax   time.Duration
}

func (s *section) commit(d time.Duration) {
    s.last = d
    s.samples[s.head] = d
    s.head = (s.head + 1) % len(s.samples)
    if s.filled < len(s.samples) {
        s.filled++
    }
    s.runTotal += d
    s.runCount++
    if d > s.runMax {
        s.runMax = d
    }
}
```

`Snapshot()` turns the sections into rows sorted by cost (average, maximum, share of the
phase); the panel and the report both render a snapshot.

## Reading it in the game

**Shift+P** toggles `system.PerfPanel` (flag: `Game.Debug.Perf`, independent of the
`/` debug flag). Top to bottom:

- TPS, FPS and the tick budget (1000 ms / TPS).
- Update and Draw: average, window maximum, share of the budget, and a sparkline of
  the last two seconds scaled to its own peak. Green under a quarter of the budget,
  amber under a half, red beyond.
- One row per system and drawable, sorted by average cost. The bar is the share of
  its phase. Sub-millisecond rows read in microseconds.
- Heap, allocation rate, goroutines, GCs and entities.

The panel renders into its own image every six frames and blits it in between, so
its row stays small and the numbers hold still.

## Profiling without watching (how an agent does it)

```sh
make perf                                   # 600 ticks, report on stdout
make perf TICKS=1200 ARGS="world.seed=7"    # any setting passes through
make perf CPU=1 MEM=1                       # also perf.cpu.pprof, perf.mem.pprof
go tool pprof -top -nodecount=30 perf.cpu.pprof
go tool pprof -sample_index=alloc_space -top perf.mem.pprof
```

The settings behind it are `run.` rows of the settings table
(`boot/settings.go`; `go run . help`, see [dev shortcuts](dev-shortcuts.md)):

| Setting | Effect |
|---|---|
| `run.ticks=n` | exit after n ticks (`run.screenshot` still exits on its own schedule) |
| `run.warmup=n` | `Perf.Reset()` after n ticks (default 60) so first-frame uploads stay out of the maxima |
| `run.report=path` | write the report at exit; `-` is stdout, a `.json` suffix writes the snapshot as JSON |
| `run.perflog=s` | print the report every s seconds while running |
| `client.perfpanel` | boot with the panel open (pair with `run.screenshot=` to capture it) |
| `run.cpuprofile=path` | pprof CPU profile of the whole run |
| `run.memprofile=path` | heap profile at exit (after a forced GC) |
| `run.vsync=off` | uncap the frame rate, to measure Draw against the GPU instead of the monitor |

On macOS, a run that opens behind other windows (a screenshot run that shouldn't
steal focus) has to draw with OpenGL, because Metal stalls on a covered window. Its
timings come from another driver presenting nothing, so never read performance from
it; measure with a focused run on the default driver.

The report prints lifetime and window figures side by side:

```
UPDATE  run avg 0.225 ms  run max 0.434 ms  (1% of budget) · window avg 0.218 ms  max 0.434 ms
  section                        run avg   run max  share   win avg   win max
  Render                        0.117 ms  0.173 ms    52%  0.117 ms  0.173 ms
```

An embedded server (single player runs the authoritative server in-process, see
[networking](networking.md)) ticks on its own goroutine, so its cost is not in any
Update row. It reports every step through an `Observe` callback, which `main.go`
feeds to `engine.Perf.Observe(engine.PhaseServer, d)`, and the report prints it as
one line with its own tick count and rate (no spans: the domain tick is one unit):

```
SERVER  run avg 0.041 ms  run max 0.148 ms  (0% of budget) · window avg 0.042 ms  max 0.125 ms · 540 ticks · 60.1 TPS
```

The panel shows the same as a third sparkline. A headless `cmd/server` leaves
`Observe` nil and the line is absent. The real per-tick cost of the game is UPDATE
plus SERVER.

The network link gets a block of its own the same way. The client measures on its
own clock (bytes on the reader and writer goroutines, the snapshot age and the
corrections and the stalls on the game loop, the round trip from a ping once a
second) and exposes the numbers as stats; the scene feeds them to
`engine.Perf.ObserveNet` once a tick, and `Snapshot.NetRows` formats them for the
panel and the report alike. Offline there are no numbers and no block appears:

```
NETWORK link               up
NETWORK rtt                16.563 ms
NETWORK snapshot age       0.000 ms  max 33.333 ms
NETWORK snapshots          20.0/s
NETWORK corrections        0.0/s
NETWORK stalls             0.0/s
NETWORK bytes in           15 KB/s
NETWORK bytes out          1 KB/s
```

Artificial lag on the embedded pipe (`net.lag=120ms net.jitter=20ms`) is the latency
lab: the RTT reads about 240 ms and the snapshot age climbs past a snapshot interval.

Compare two reports from the same settings before and after a change; the `share`
column says where a regression went, and pprof says which function inside it.
The display must be awake for a windowed run: on macOS `make perf` wraps `caffeinate -u`.

## Finer splits

A system that is one fat row opens spans of its own. A `/` nests the row under
its parent in both the panel and the report:

```go
func (r *Render) Draw(w donburi.World, screen *ebiten.Image) {
    defer engine.Perf.Start("Render/composite").End()
    …
}
```

Spans opened outside a phase (a system driven by a unit test) file under Update
and commit on `End`.

## Rules

- **Wiring is in the scene, not the systems.** A system never times itself as a
  whole; registering it in `g.systems` is what gives it a row. The headless
  `TestPerfSpansHeadless` checks every registered system has one.
- **Names are type names.** `reflect` reads them once at scene construction;
  `Update` and `Draw` build no strings.
- **Sampling stays on.** Gate the panel, not the measurement, so a report is
  available the moment it is asked for and the window holds history.
- **Spans stay on the game loop.** Another goroutine never calls `Start`; it
  measures itself and reports through `Perf.Observe`, the one goroutine-safe entry.
- **No `ReadMemStats` per tick.** It stops the world; once a second is the rule.
- **The warm-up reset is the only reset.** `R` rebuilds the world but keeps the
  history: a rebuild is a thing worth seeing in the sparkline.
- Use `engine.Budget(tps)` for the per-tick allowance; never hard-code 16.67 ms.
