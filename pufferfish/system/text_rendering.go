package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/assets"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

type TextRendering struct {
	query       *donburi.Query
	cameraQuery *donburi.Query
	offscreen   *ebiten.Image
}

func NewTextRendering() *TextRendering {
	return &TextRendering{
		query: donburi.NewQuery(
			filter.Contains(component.Text),
		),
		cameraQuery: donburi.NewQuery(filter.Contains(component.Camera)),
		offscreen:   ebiten.NewImage(domain.MapWidth, domain.MapHeight),
	}
}

func (r *TextRendering) Draw(w donburi.World, screen *ebiten.Image) {
	r.offscreen.Clear()
	for entry := range r.query.Iter(w) {
		pos := transform.WorldPosition(entry)

		txt := component.Text.Get(entry)

		op := &text.DrawOptions{}
		op.PrimaryAlign = text.AlignCenter
		op.GeoM.Translate(pos.X, pos.Y)
		text.Draw(r.offscreen, txt.Text, assets.Font, op)
	}

	cam, ok := r.cameraQuery.First(w)
	if !ok {
		return
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-transform.WorldPosition(cam).X, -transform.WorldPosition(cam).Y)
	screen.DrawImage(r.offscreen, op)
}
