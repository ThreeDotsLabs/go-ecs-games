# Layers & Y-sort

## How it works

Draw order has two mechanisms and only two. **Layers** order groups of entities:
`component.Layer` holds an int, drawn low→high. **Y-sort** orders entities *within*
one layer by world Y, so in top-down scenes things lower on screen draw in front.
Both are resolved per camera, per frame, inside [the renderer](cameras.md): it walks
the camera Root's transform subtree, buckets every drawable by layer, sorts the layer
ints, and within each layer draws non-Y-sorted entities first (creation order), then
the Y-sorted ones.

## Canonical code

**Banded layer constants** — gaps leave room for in-between layers; rename the set per
game (`component/layer.go`):

```go
const (
    LayerBackground LayerID = 10
    LayerGround     LayerID = 20
    LayerObjects    LayerID = 30
    LayerCharacters LayerID = 50
    LayerForeground LayerID = 100
    // LayerHeldItem rides above the foreground so a tool carried by the player
    // always reads on top of scenery, not tucked behind it.
    LayerHeldItem LayerID = 150
    LayerUI       LayerID = 200
)
```

**Collection** — per camera, an iterative DFS over the Root subtree with an explicit
stack and a `seen` map, tracking hierarchy depth; entities without their own `Layer`
fall back to the root's (`system/render.go`, `getAllChildren`):

```go
stack := []entryWithDepth{{entry: entry, depth: 0}}

for len(stack) > 0 {
    current := stack[len(stack)-1]
    stack = stack[:len(stack)-1]

    if !current.entry.Valid() || !isActive(current.entry) || seen[current.entry] {
        continue
    }
    seen[current.entry] = true

    if current.entry.HasComponent(component.Sprite) || current.entry.HasComponent(component.Text) || current.entry.HasComponent(component.Camera) {
        result = append(result, getEntryWithLayerAndDepth(current.entry, rootLayer, current.depth))
    }
```

**Within-layer sort** — stable, world Y plus per-entity offset, hierarchy depth as the
tiebreaker so children draw over parents at the same Y (`system/render.go`):

```go
sort.SliceStable(ySorted, func(i, j int) bool {
    posI := getYSortPosition(ySorted[i].entry)
    posJ := getYSortPosition(ySorted[j].entry)

    ySortI := component.YSort.Get(ySorted[i].entry)
    ySortJ := component.YSort.Get(ySorted[j].entry)

    posI += ySortI.YOffset
    posJ += ySortJ.YOffset

    if math.Abs(posI-posJ) < 0.01 { // Small epsilon for float comparison
        return ySorted[i].depth < ySorted[j].depth
    }

    return posI < posJ
})

// Render non-Y-sorted first, then Y-sorted
allEntities := append(nonYSorted, ySorted...)
```

**Multi-part objects sort as one unit** — a Y-sorted child walks up to its Y-sorted
ancestor and uses *its* Y, so a three-sprite oven (legs, middle, top — see
`archetype/oven.go`) never interleaves with the player
(`system/render.go`):

```go
func getYSortPosition(entry *donburi.Entry) float64 {
    parent, hasParent := transform.GetParent(entry)
    if hasParent && parent.Valid() && parent.HasComponent(component.YSort) {
        // Use parent's Y position recursively (in case of nested objects)
        return getYSortPosition(parent)
    }

    pos := transform.WorldPosition(entry)
    return pos.Y
}
```

**Builder helpers** (`archetype/entry.go`). `WithLayerInherit` gives a child
`parent+1` — the standard for attachments that must draw over their parent:

```go
func (b EntryBuilder) WithLayerInherit() EntryBuilder {
    b.With(component.Layer)

    parent, ok := transform.GetParent(b.entry)
    if !ok {
        panic("parent not found")
    }

    parentLayer := component.Layer.Get(parent).Layer
    component.Layer.Get(b.entry).Layer = parentLayer + 1
    return b
}
```

`WithYSort` derives the offset from the sprite's pivot so sorting always happens at the
sprite's *feet*, whatever the pivot (`archetype/entry.go`):

```go
var yOffset float64
switch sprite.Pivot {
case component.SpritePivotBottomCenter:
    // For bottom-center pivot, Y position is already at the bottom
    yOffset = 0
case component.SpritePivotCenter:
    // For center pivot, add half the sprite height
    yOffset = float64(sprite.Image.Bounds().Dy()) / 2
default: // SpritePivotTopLeft
    // For top-left pivot, add full sprite height to get to bottom
    yOffset = float64(sprite.Image.Bounds().Dy())
}

component.YSort.SetValue(b.entry, component.YSortData{YOffset: yOffset})
```

**Layer swap at the contact frame** — layers are mutable data. A dropped item falls
on `LayerHeldItem` (above the player) and joins the ground layer exactly when it
lands (`system/interact.go`, `drop`):

```go
if p < fallPart {
    t.LocalPosition = engine.LerpVec2(from, to, engine.EaseIn(p/fallPart))
} else {
    // Landed: from here the item belongs to the ground, so lower it
    // below the player as it squashes and recovers.
    component.Layer.Get(e).Layer = component.LayerObjects
```

## Rules

- A camera's Root **must** carry a `Layer` — the renderer panics otherwise. Everything
  without its own `Layer` inherits the root's at render time.
- World objects use `SpritePivotBottomCenter` so position, grid cell, and Y-sort all
  agree on "where the feet are" (see [sprite-effects.md](sprite-effects.md)).
- Every part of a multi-part object gets `WithYSort()` — the ancestor walk only works
  if the chain of parents is Y-sorted too.
- Attachments that must draw over their parent use `WithLayerInherit()` (`WithParent`
  first, or it panics); leave band gaps (steps of 10+) so `parent+1` never collides
  with the next band.
- No third ordering mechanism (z floats, draw lists) — change the layer int or add `YSort`.

## Pitfalls

- The renderer re-walks the whole tree, re-buckets, and re-sorts **every frame for
  every camera**, allocating as it goes — no caching, no dirty flags. Known cost,
  fine at a few hundred sprites; keep it until profiling says otherwise, and see
  [chunks.md](chunks.md) when the world outgrows the screen.
- `getYSortPosition` recurses per comparison inside the sort — same verdict: don't be
  surprised by it in a profile.

## Variations

- **Side-scrollers need no Y-sort** — layers alone; within a layer, traversal order
  decides. Adopt `YSort` only for top-down scenes.
- Don't use a `SpriteLayerInherit = 0` sentinel resolved per frame in the renderer
  (a child with layer 0 inherits `parent+1`); the builder-time `WithLayerInherit`
  resolves it once.
- Layer constants live in `component/layer.go`, not `domain/` — they are a rendering
  concern.
