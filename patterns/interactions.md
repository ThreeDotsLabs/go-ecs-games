# Interactions & item storage

## How it works

Interaction behavior belongs to the **object**: each interactable entity carries a bag of
verbs — `Can*` predicates and `On*` callbacks — declared **as closures at construction**
in its archetype. **One system** drives all of them: it finds what the player is facing
(via a detector collider), checks the object's predicate, and calls its callback. No
system ever switches on object type; ovens, crates, doors and cash registers differ only
in the closures they were built with. A second generic component, `ItemStorage`, gives any
object slots that items parent into — counters, display cases, crates, and the portable
baking sheet all reuse it.

## Canonical code

**The verb bag** — predicates, one-shot verbs, and a continuous-interaction lifecycle
(`component/interactable.go`):

```go
type InteractableData struct {
    // Condition checks - return true if the action is possible
    CanPickup             func(self *donburi.Entry, player *donburi.Entry, heldItem *donburi.Entry) bool
    CanInteract           func(self *donburi.Entry, player *donburi.Entry, heldItem *donburi.Entry) bool
    CanContinuousInteract func(self *donburi.Entry, player *donburi.Entry, heldItem *donburi.Entry) bool

    // One-time actions
    OnPickup   func(w donburi.World, player *donburi.Entry, self *donburi.Entry, heldItem *donburi.Entry)
    OnInteract func(w donburi.World, player *donburi.Entry, self *donburi.Entry, heldItem *donburi.Entry)

    // Continuous interaction lifecycle
    OnContinuousStart    func(w donburi.World, player *donburi.Entry, self *donburi.Entry, heldItem *donburi.Entry)
    OnContinuousUpdate   func(w donburi.World, player *donburi.Entry, self *donburi.Entry, heldItem *donburi.Entry)
    OnContinuousComplete func(w donburi.World, player *donburi.Entry, self *donburi.Entry, heldItem *donburi.Entry)
    OnContinuousCancel   func(w donburi.World, player *donburi.Entry, self *donburi.Entry, heldItem *donburi.Entry)

    RequiresContinuousHold bool // Whether interaction cancels when key is released
}
```

**Generic storage** — slots + validation, pure data with pure helpers
(`component/item_storage.go`):

```go
type ItemStorageData struct {
    MaxSlots      int
    SlotPositions []math.Vec2 // Local positions for each slot relative to container

    CanAcceptItem func(item *donburi.Entry) bool // Optional validation function
    AllowCombine  bool // Whether to try combining items when storage is full

    StoredItems []*donburi.Entry // maintained in order of addition
}
```

**Wiring an object** — the archetype declares verbs from shared factories plus per-object
closures (`archetype/display_case.go`):

```go
component.ItemStorage.SetValue(obj, component.ItemStorageData{
    MaxSlots:      4,
    SlotPositions: []math.Vec2{{X: -30, Y: -60}, {X: 30, Y: -60}, {X: -30, Y: -30}, {X: 30, Y: -30}},
    CanAcceptItem: func(item *donburi.Entry) bool {
        itemData := component.Item.Get(item)
        itemInfo := itemData.GetItem()
        return itemInfo.IsFood() && itemInfo.CanBeSold()
    },
})

component.Interactable.SetValue(obj, component.InteractableData{
    OnPickup: CreatePickupHandler(), // standard place-into / take-from storage
})
```

A crate instead wires the continuous lifecycle — hold the key to unpack
(`archetype/crate.go`):

```go
component.Interactable.SetValue(obj, component.InteractableData{
    CanContinuousInteract: func(self *donburi.Entry, player *donburi.Entry, heldItem *donburi.Entry) bool {
        return heldItem == nil // Only if hands are empty
    },
    OnContinuousUpdate:     CreateCrateUnpackHandler(), // starts the Working timer + progress bar
    RequiresContinuousHold: true,
})
```

Shared behavior lives as **factory functions** in `archetype/interaction_helpers.go`:
`CreatePickupHandler` (held item goes into storage, empty hands take the last item back
out), `AddToStorage` (reparents the item to the container, places it at the next slot's
`SlotPosition`, publishes the drop-sound event), `TakeLastFromStorage` (LIFO),
`AttachItemToPlayerHands`. Every storage mutation goes through these helpers — never
append to `StoredItems` by hand.

**The one driving system** — checks the predicate, then hands control to the object
(`system/interaction.go`; the input system calls these on key presses):

```go
if currentSelected.HasComponent(component.Interactable) {
    interactable := component.Interactable.Get(currentSelected)
    if interactable.CanPickup != nil && !interactable.CanPickup(currentSelected, playerEntity, heldItem) {
        return // Cannot pickup
    }
    if interactable.OnPickup != nil {
        interactable.OnPickup(w, playerEntity, currentSelected, heldItem)
        return
    }
}
```

**Detection** — the player carries two child entities: `ItemHands` (where held items
parent) and a collider child tagged `component.InteractionDetector` on
`CollisionLayerHands`, positioned ahead of the player and re-aimed by
`system.InteractionDetectorPositioning` as facing changes. Objects expose a separate
child collider on `CollisionLayerInteractable`; the interaction system reads the
detector's collisions each tick, keeps the current selection sticky while it remains a
candidate, and stores it on `PlayerSelection` with a highlight (see
[collisions.md](collisions.md)). From `archetype/player.go`:

```go
interactionDetector := New(w).
    WithParent(player).
    WithLayerInherit().
    With(component.InteractionDetector).
    With(component.Collider).
    Entry()
```

**Nested storage** — the baking sheet is an *item* that itself carries `ItemStorage`
(4 slots, `archetype/baking_sheet.go`). Placed on a counter it sits inside the
counter's storage; the pickup handlers detect a sheet in a counter's slots and route items
onto it, so "carry four loaves at once into the display case" falls out of the same two
components with no special system.

## Rules

- **Interaction behavior belongs to the object** — closures on its `Interactable`
  component, declared in the archetype. Never a god-system that switches on object type.
- Predicates (`Can*`) are separate from actions (`On*`); the driving system always checks
  the predicate first. Both are optional — leave nil what the object doesn't support.
- Reusable verb implementations are factories in `archetype/interaction_helpers.go`;
  per-object quirks stay inline closures.
- Any object that holds items uses `ItemStorage` + the `AddToStorage` /
  `TakeLastFromStorage` helpers, which keep reparenting, slot positions, Y-sort offsets
  and sound events consistent (see [layers-ysort.md](layers-ysort.md)).
- Continuous interactions run on the object's `Working` timer; the working system fires
  `OnContinuousStart/Complete/Cancel` as the timer state changes
  (`system/working.go`), while the input path only calls `OnContinuousUpdate`.

## Pitfalls

- The interaction system attracts unrelated modes (build mode, blueprints) — keep it to
  the dispatch core and give each mode its own system. The source grew to 763 lines.
- Give the predicate and callback the same argument order; the source mixed
  `(self, player, heldItem)` and `(w, player, self, heldItem)`.

## Variations

- **Minimal grid variant** (`system/interact.go`): on Space, resolve the
  cell the player faces (`entityCell(board, player).Add(player.Facing)`); empty hands pick
  up the ground item there, full hands put it down if the cell is free. Pickup/drop is
  reparenting (`transform.ChangeParent`) with explicit restore of local scale, layer
  (`LayerHeldItem` while carried, `LayerObjects` on landing) and sprite flip, wrapped in
  glide/carry/drop tweens — the choreography is documented in
  [animations.md](animations.md). Right shape for one verb; grow toward the
  component above when objects need *different* verbs.
