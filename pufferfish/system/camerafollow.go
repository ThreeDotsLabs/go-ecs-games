package system

import (
	"github.com/ThreeDotsLabs/go-ecs-games/pufferfish/component"
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/features/transform"
	"github.com/yohamta/donburi/filter"
)

type CameraFollow struct {
	query *donburi.Query
}

func NewCameraFollow() *CameraFollow {
	return &CameraFollow{
		query: donburi.NewQuery(
			filter.Contains(
				component.Camera,
			),
		),
	}
}

func (s *CameraFollow) Update(w donburi.World) {
	for entry := range s.query.Iter(w) {
		cam := component.Camera.Get(entry)
		if cam.Target == nil {
			continue
		}

		tr := transform.GetTransform(entry)
		pos := transform.WorldPosition(cam.Target)

		settings := component.MustFindSettings(w)

		// The target is centered on its position, so center the screen on it.
		pos.X -= float64(settings.ScreenWidth) / 2.0
		pos.Y -= float64(settings.ScreenHeight) / 2.0

		tr.LocalPosition = pos
	}
}
