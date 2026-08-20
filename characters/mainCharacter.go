package characters

import (
	"bytes"
	"image"
	_ "image/png"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/images"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/taquion/kurovtheworld/channels"
)

const (
	screenWidth  = 320
	screenHeight = 240

	frameOX     = 0
	frameOY     = 32
	frameWidth  = 32
	frameHeight = 32
	frameCount  = 8
)

var (
	runnerImage *ebiten.Image
)

type MainCharacter struct {
	count        int
	screen       *ebiten.Image
	sX           int
	sY           int
	posX         float64
	posY         float64
	action       channels.UserCharacterChannel
	screenWidth  int
	screenHeight int
	keys         []ebiten.Key
	key          ebiten.Key
}

func (g *MainCharacter) Update() error {
	g.count++
	if inpututil.IsKeyJustPressed(g.key) {
		g.posX++
	}
	return nil
}

func (g *MainCharacter) Draw(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-float64(frameWidth)/2, -float64(frameHeight)/2)
	op.GeoM.Translate(g.posX, g.posY)

	screen.DrawImage(runnerImage.SubImage(image.Rect(g.sX, g.sY, g.sX+frameWidth, g.sY+frameHeight)).(*ebiten.Image), op)
}

func (g *MainCharacter) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func (g *MainCharacter) SetAction(action channels.UserCharacterChannel) {
	g.action = action
}

func (g *MainCharacter) Init() {
	i := (g.count / 5) % frameCount
	g.sX, g.sY = frameOX+i*frameWidth, frameOY
	g.posX = screenHeight / 2
	g.posY = screenHeight / 2

	img, _, err := image.Decode(bytes.NewReader(images.Runner_png))
	if err != nil {
		log.Fatal(err)
	}
	runnerImage = ebiten.NewImageFromImage(img)

	ebiten.SetWindowSize(g.screenWidth*2, g.screenHeight*2)
	ebiten.SetWindowTitle("Animation (Ebitengine Demo)")

	if err := ebiten.RunGame(&MainCharacter{}); err != nil {
		log.Fatal(err)
	}
}

func NewCharacter(action chan channels.UserCharacterChannel, screenWidth, screenHeight, posX, posY int) *MainCharacter {
	w := &MainCharacter{
		count: 0,
		// action:       action,
		screenWidth:  screenWidth,
		screenHeight: screenHeight,
		// posX:         posX,
		// posY:         posY,
	}

	w.Init()
	return w
}
