package main

// This file is not part of the ECS example. It adds an optional debug overlay
// that shows the components of the tile under the cursor, on top of the
// regular game (tiles made non-clickable). Run with --inspect (or open the web
// build with ?inspect=1).

import (
	"os"
	"slices"

	"github.com/ThreeDotsLabs/go-ecs-games/tiles/internal/inspector"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

func attachInspector(g *Game) {
	if !slices.Contains(os.Args, "--inspect") {
		return
	}

	query := donburi.NewQuery(
		filter.Contains(Position, Color),
	)
	overlay := inspector.New(query, TileSize, func(entry *donburi.Entry) inspector.Tile {
		pos := Position.Get(entry)

		return inspector.Tile{
			X:     pos.X,
			Y:     pos.Y,
			Color: Color.Get(entry).Color,
		}
	})

	// Clicks only select tiles here. Collected first, as removing while iterating skips entities.
	var clickable []*donburi.Entry
	for entry := range donburi.NewQuery(filter.Contains(Clickable)).Iter(g.world) {
		clickable = append(clickable, entry)
	}
	for _, entry := range clickable {
		entry.RemoveComponent(Clickable)
	}

	g.systems = append(g.systems, overlay)
	g.drawers = append(g.drawers, overlay)
}
