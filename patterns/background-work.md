# Background work over game snapshots

## How it works

Some work takes longer than a frame: finding reachable ground, evaluating threats,
or asking a CPU or LLM for a command. Give that work a detached domain snapshot.
Workers compute results without touching the ECS world or renderer. The game loop
receives the results and decides whether they still belong to the displayed state.

The pattern has two forms of ownership: a cache and result channel per snapshot
for derived overlays, and a version number plus cancellation for seat decisions.
Both build on [domain separation](domain.md) and the channel-to-game-loop boundary
in [networking](networking.md).

## Derived results: replace the cache with the snapshot

`component.Overlays` owns a `Reacher`, result maps, a set of requested keys, and a
buffered channel of closures that apply finished results. `GameLogic` creates a
new `Overlays` whenever it replaces `BoardData.View`.

The game loop requests an absent result once. The worker captures the old cache's
search object and channel, computes an answer, and sends a closure. Only the game
loop invokes that closure and mutates the result maps. From `system/threat.go`:

```go
func request(o *component.Overlays, key any, search func() func(*component.Overlays)) {
    if o.Pending[key] {
        return
    }
    o.Pending[key] = true
    results := o.Results
    go func() {
        apply := search()
        select {
        case results <- apply:
        default:
        }
    }()
}
```

`drainOverlays` drains only the current cache's channel. Once the view changes,
old results remain on an abandoned channel and cannot overwrite current answers.
The worker does not look up the newest cache when it finishes. This also handles
undo, where the board can return to an earlier arrangement.

The channel has room for 256 results, intended to cover the searches started by one
view; a nonblocking send prevents an obsolete producer from hanging. A dropped
result leaves its key pending for that cache's lifetime. If the workload can exceed
that capacity, add explicit retry or completion handling rather than assuming this
policy guarantees delivery. `Pending` deduplicates requests; it is not a worker
limit or a cancellation mechanism.

## Commands: version and cancel the request

`system.CPU` asks all non-human seats through the same `Seat.Play(ctx, view)`
contract. It starts one request only after queued events, movement, and dice
animations have finished, and captures the board's `Version` with the snapshot.

When the reply arrives, the loop publishes `CommandRequested` only if the returned
version still matches. The ordinary command adapter validates and executes it.
When the view changes during a request, the loop cancels its context; it still
waits for that request to return before starting another. This keeps a stateful
seat's conversation or plan from being accessed by overlapping calls.

Cancellation saves work only when the worker observes it. The version check remains
necessary even with cancellation, since a reply can race with it. The CPU seat
ignores the context while computing; an LLM seat can pass it to its I/O. When an LLM
seat errors, times out or answers with something that isn't one of the offered options,
the CPU seat answers instead and the log says why: a turn never stalls.

## Rules and limits

- Detach mutable slices, maps, and pointers before handing state to a worker.
  Shared search helpers need their own synchronization; a snapshot does not make
  mutable internal caches safe automatically.
- Keep ECS queries, component mutations, and graphics operations on the game loop.
  Capturing a world or a live component pointer defeats this boundary.
- Apply results only to the state that requested them. A board revision must change
  even when undo restores an earlier position; equality of positions is insufficient.
- A worker must be able to finish after its scene or result consumer is abandoned.
  Use bounded/nonblocking delivery or cancellation-aware sends, with an explicit
  policy for dropped results.
- On WASM these goroutines share one thread: starting a CPU search in a goroutine
  does not guarantee a smooth frame. Heavy searches may need smaller work units.
- Verify stale results after undo/restart, duplicate requests, and worker completion
  after the consumer is gone when implementing this boundary. Use the race detector
  for native code that shares search helpers.
