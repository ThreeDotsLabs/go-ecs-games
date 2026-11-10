package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/domain"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
	"golang.org/x/image/colornames"
)

type Rendering struct {
	query       *donburi.OrderedQuery[component.SpriteData]
	cameraQuery *donburi.Query
	offscreen   *ebiten.Image
}

func NewRendering() *Rendering {
	return &Rendering{
		query: donburi.NewOrderedQuery[component.SpriteData](
			filter.Contains(component.Sprite),
		),
		cameraQuery: donburi.NewQuery(filter.Contains(component.Camera)),
		offscreen:   ebiten.NewImage(domain.MapWidth, domain.MapHeight),
	}
}

func (r *Rendering) Draw(w donburi.World, screen *ebiten.Image) {
	settings := component.MustFindSettings(w)

	r.offscreen.Fill(colornames.Dodgerblue)
	for entry := range r.query.IterOrdered(w, component.Sprite) {
		pos := transform.WorldPosition(entry)
		scale := transform.WorldScale(entry)
		rotation := transform.WorldRotation(entry)

		sprite := component.Sprite.Get(entry)
		bounds := sprite.Image.Bounds()

		op := &ebiten.DrawImageOptions{}

		halfW := float64(bounds.Dx()) / 2
		halfH := float64(bounds.Dy()) / 2

		// Sprites are centered on their position: move the image center to the origin,
		// flip, scale and rotate around it, then move it to the position.
		op.GeoM.Translate(-halfW, -halfH)
		if sprite.FlipX {
			op.GeoM.Scale(-1, 1)
		}
		op.GeoM.Scale(scale.X, scale.Y)
		op.GeoM.Rotate(rotation)
		op.GeoM.Translate(pos.X, pos.Y)

		r.offscreen.DrawImage(sprite.Image, op)

		if settings.Debug {
			if entry.HasComponent(component.Bounds) {
				rect := component.Bounds.Get(entry).Rect(entry)
				vector.StrokeRect(r.offscreen, float32(rect.X), float32(rect.Y), float32(rect.Width), float32(rect.Height), 1, colornames.Magenta, true)
			}
		}
	}

	cam, ok := r.cameraQuery.First(w)
	if !ok {
		return
	}
	camPos := transform.WorldPosition(cam)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-camPos.X, -camPos.Y)
	screen.DrawImage(r.offscreen, op)
}
