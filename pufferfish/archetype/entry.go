package archetype

import (
	"time"

	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/engine"
	"github.com/yohamta/donburi"
	donburicomponent "github.com/yohamta/donburi/component"
	"github.com/yohamta/donburi/features/math"
	"github.com/yohamta/donburi/features/transform"
)

type EntryBuilder struct {
	entry *donburi.Entry
}

func New(w donburi.World) EntryBuilder {
	return EntryBuilder{
		entry: w.Entry(w.Create(transform.Transform)),
	}
}

func (b EntryBuilder) With(c donburicomponent.IComponentType) EntryBuilder {
	if !b.entry.HasComponent(c) {
		b.entry.AddComponent(c)
	}

	return b
}

func (b EntryBuilder) WithPosition(pos math.Vec2) EntryBuilder {
	transform.Transform.Get(b.entry).LocalPosition = pos
	return b
}

func (b EntryBuilder) WithScale(scale math.Vec2) EntryBuilder {
	transform.Transform.Get(b.entry).LocalScale = scale
	return b
}

func (b EntryBuilder) WithParent(parent *donburi.Entry) EntryBuilder {
	transform.AppendChild(parent, b.entry, false)
	return b
}

func (b EntryBuilder) WithSprite(sprite component.SpriteData) EntryBuilder {
	b.With(component.Sprite)
	component.Sprite.SetValue(b.entry, sprite)
	return b
}

func (b EntryBuilder) WithSpriteBounds() EntryBuilder {
	b.With(component.Bounds)

	sprite := component.Sprite.Get(b.entry)
	imageBounds := sprite.Image.Bounds()

	component.Bounds.SetValue(b.entry, component.BoundsData{
		Width:  float64(imageBounds.Dx()),
		Height: float64(imageBounds.Dy()),
	})

	return b
}

func (b EntryBuilder) Entry() *donburi.Entry {
	return b.entry
}

func (b EntryBuilder) WithTimeToLive(d time.Duration) EntryBuilder {
	b.With(component.TimeToLive)
	component.TimeToLive.SetValue(b.entry, component.TimeToLiveData{
		Timer: engine.NewTimer(d),
	})
	return b
}
