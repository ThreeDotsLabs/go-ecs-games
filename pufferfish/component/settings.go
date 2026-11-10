package component

import (
	"github.com/yohamta/donburi"
	"github.com/yohamta/donburi/filter"
)

type SettingsData struct {
	ScreenWidth  int
	ScreenHeight int

	Debug bool
}

var Settings = donburi.NewComponentType[SettingsData]()

var settingsQuery = donburi.NewQuery(filter.Contains(Settings))

func MustFindSettings(w donburi.World) *SettingsData {
	entry, ok := settingsQuery.First(w)
	if !ok {
		panic("settings not found")
	}

	return Settings.Get(entry)
}
