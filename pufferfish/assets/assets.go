package assets

import (
	"bytes"
	"embed"
	"image"
	_ "image/png"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/text/language"
)

var (
	//go:embed fish/* font.ttf
	assetsFS embed.FS

	BlueFish   = mustLoadImage("fish/fish_blue_outline.png")
	RedFish    = mustLoadImage("fish/fish_red_outline.png")
	BrownFish  = mustLoadImage("fish/fish_brown_outline.png")
	OrangeFish = mustLoadImage("fish/fish_orange_outline.png")
	GreenFish  = mustLoadImage("fish/fish_green_outline.png")

	BlueFishSkeleton   = mustLoadImage("fish/fish_blue_skeleton_outline.png")
	RedFishSkeleton    = mustLoadImage("fish/fish_red_skeleton_outline.png")
	OrangeFishSkeleton = mustLoadImage("fish/fish_orange_skeleton_outline.png")
	GreenFishSkeleton  = mustLoadImage("fish/fish_green_skeleton_outline.png")

	Bubble1 = mustLoadImage("fish/bubble_a.png")
	Bubble2 = mustLoadImage("fish/bubble_b.png")
	Bubble3 = mustLoadImage("fish/bubble_c.png")

	Font = mustLoadFont("font.ttf", 28)
)

func mustLoadFont(path string, size int) *text.GoTextFace {
	data, err := fs.ReadFile(assetsFS, path)
	if err != nil {
		panic(err)
	}

	source, err := text.NewGoTextFaceSource(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}

	return &text.GoTextFace{
		Source:    source,
		Direction: text.DirectionLeftToRight,
		Size:      float64(size),
		Language:  language.English,
	}
}

func mustLoadImage(path string) *ebiten.Image {
	data, err := fs.ReadFile(assetsFS, path)
	if err != nil {
		panic(err)
	}

	return mustNewEbitenImage(data)
}

func mustNewEbitenImage(data []byte) *ebiten.Image {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		panic(err)
	}

	return ebiten.NewImageFromImage(img).SubImage(opaqueBounds(img)).(*ebiten.Image)
}

// opaqueBounds trims transparent padding, so the image's bounds match its visible pixels.
func opaqueBounds(img image.Image) image.Rectangle {
	b := img.Bounds()

	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X-1, b.Min.Y-1

	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a == 0 {
				continue
			}
			minX = min(minX, x)
			maxX = max(maxX, x)
			minY = min(minY, y)
			maxY = max(maxY, y)
		}
	}

	if maxX < b.Min.X {
		return image.Rectangle{}
	}

	return image.Rect(minX, minY, maxX+1, maxY+1)
}
