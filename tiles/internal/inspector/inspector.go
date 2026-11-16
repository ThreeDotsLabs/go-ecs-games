package inspector

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/yohamta/donburi"
)

// Tile is read from the selected entity whenever the inspector needs its components.
type Tile struct {
	X, Y  float64
	Color color.Color
}

// New creates an overlay that queries the game's world and reads live components.
func New(query *donburi.Query, tileSize int, readTile func(*donburi.Entry) Tile) *System {
	return &System{
		query:    query,
		readTile: readTile,
		tileSize: tileSize,
		panel:    ebiten.NewImage(190, 64),
	}
}

type System struct {
	query              *donburi.Query
	readTile           func(*donburi.Entry) Tile
	tileSize           int
	selected           *donburi.Entry
	panel              *ebiten.Image
	touchIDs           []ebiten.TouchID
	touchSelection     bool
	mouseX, mouseY     int
	pointerX, pointerY int
}

func (s *System) Update(w donburi.World) {
	x, y := ebiten.CursorPosition()
	mouseMoved := x != s.mouseX || y != s.mouseY
	s.mouseX, s.mouseY = x, y

	s.touchIDs = ebiten.AppendTouchIDs(s.touchIDs[:0])
	if len(s.touchIDs) > 0 {
		x, y = ebiten.TouchPosition(s.touchIDs[0])
		s.touchSelection = true
	} else if s.touchSelection && !mouseMoved {
		// Keep the tapped tile readable after the finger is lifted.
		return
	} else {
		s.touchSelection = false
	}

	s.pointerX, s.pointerY = x, y
	s.selected = s.tileAt(w, x, y)
}

func (s *System) tileAt(w donburi.World, x, y int) *donburi.Entry {
	for entry := range s.query.Iter(w) {
		tile := s.readTile(entry)

		// Exclusive upper bounds select exactly one tile at shared edges.
		if float64(x) >= tile.X && float64(x) < tile.X+float64(s.tileSize) &&
			float64(y) >= tile.Y && float64(y) < tile.Y+float64(s.tileSize) {
			return entry
		}
	}

	return nil
}

func (s *System) Draw(w donburi.World, screen *ebiten.Image) {
	if s.selected == nil || !s.selected.Valid() {
		return
	}

	pos := s.readTile(s.selected)
	c := color.NRGBAModel.Convert(pos.Color).(color.NRGBA)

	vector.StrokeRect(screen, float32(pos.X)+2, float32(pos.Y)+2,
		float32(s.tileSize)-4, float32(s.tileSize)-4, 4, color.Black, false)
	vector.StrokeRect(screen, float32(pos.X)+2, float32(pos.Y)+2,
		float32(s.tileSize)-4, float32(s.tileSize)-4, 2, color.White, false)

	s.panel.Fill(color.NRGBA{R: 24, G: 28, B: 36, A: 245})
	text := fmt.Sprintf("Entity: %d\nPosition: X %.0f, Y %.0f\nColor: #%02X%02X%02X",
		s.selected.Id(), pos.X, pos.Y, c.R, c.G, c.B)
	ebitenutil.DebugPrintAt(s.panel, text, 8, 8)
	vector.DrawFilledRect(s.panel, 168, 42, 12, 12, c, false)

	// Follow the pointer, flipping sides near the canvas edges.
	const gap = 16
	width, height := screen.Bounds().Dx(), screen.Bounds().Dy()

	panelWidth, panelHeight := s.panel.Bounds().Dx()*2, s.panel.Bounds().Dy()*2
	panelX, panelY := s.pointerX+gap, s.pointerY+gap

	if panelX+panelWidth > width-gap {
		panelX = s.pointerX - panelWidth - gap
	}

	if s.touchSelection || panelY+panelHeight > height-gap {
		panelY = s.pointerY - panelHeight - gap
		if panelY < gap {
			panelY = s.pointerY + gap
		}
	}

	panelX = max(gap, min(panelX, width-panelWidth-gap))
	panelY = max(gap, min(panelY, height-panelHeight-gap))

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2, 2)
	op.GeoM.Translate(float64(panelX), float64(panelY))

	screen.DrawImage(s.panel, op)
}
