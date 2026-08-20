package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/taquion/kurovtheworld/channels"
	"github.com/taquion/kurovtheworld/characters"
	"github.com/taquion/kurovtheworld/controls"
)

const (
	screenWidth  = 320
	screenHeight = 240
)

type Game struct {
	mainCharacter characters.MainCharacter
	control       controls.KeyboardControl
	pixels        []byte
}

func (g *Game) Update() error {
	g.mainCharacter.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	if g.pixels == nil {
		g.pixels = make([]byte, screenWidth*screenHeight*4)
	}
	g.mainCharacter.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	action := make(chan channels.UserCharacterChannel)
	mainCharacter := *characters.NewCharacter(action, screenWidth, screenHeight, 250, 100)
	g := &Game{
		mainCharacter: mainCharacter,
		control:       *controls.NewKeyboardControl(screenWidth, screenHeight, mainCharacter),
	}

	ebiten.SetWindowSize(screenWidth*2, screenHeight*2)
	ebiten.SetWindowTitle("Game of Life (Ebitengine Demo)")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
