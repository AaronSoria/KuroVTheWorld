package controls

import (
	"fmt"
	_ "image/png"
	"log"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	// "github.com/hajimehoshi/ebiten/v2/examples/keyboard/keyboard"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/taquion/kurovtheworld/channels"
	"github.com/taquion/kurovtheworld/characters"
)

var keyboardImage *ebiten.Image

type KeyboardControl struct {
	keys   []ebiten.Key
	player characters.MainCharacter
}

func (g *KeyboardControl) Update() error {
	fmt.Println(g.keys[len(g.keys)-1])
	// g.keys = inpututil.AppendPressedKeys(g.keys[:0])
	// userCharacterAction := channels.NewUserCharacterChannel()
	// userCharacterAction.SetActionFromKeyBoard(g.keys[len(g.keys)-1])
	// g.player.SetAction(userCharacterAction)
	// g.player.Update()
	return nil
}

func (g *KeyboardControl) Draw(screen *ebiten.Image) {
	// const (
	// 	offsetX = 24
	// 	offsetY = 40
	// )

	// // Draw the base (grayed) keyboard image.
	// op := &ebiten.DrawImageOptions{}
	// op.GeoM.Translate(offsetX, offsetY)
	// op.ColorScale.Scale(0.5, 0.5, 0.5, 1)
	// screen.DrawImage(keyboardImage, op)

	// // Draw the highlighted keys.
	// op = &ebiten.DrawImageOptions{}
	// for _, p := range g.keys {
	// 	op.GeoM.Reset()
	// 	r, ok := keyboard.KeyRect(p)
	// 	if !ok {
	// 		continue
	// 	}
	// 	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	// 	op.GeoM.Translate(offsetX, offsetY)
	// 	screen.DrawImage(keyboardImage.SubImage(r).(*ebiten.Image), op)
	// }

	g.keys = inpututil.AppendPressedKeys(g.keys[:0])
	userCharacterAction := channels.NewUserCharacterChannel()
	userCharacterAction.SetActionFromKeyBoard(g.keys[len(g.keys)-1])
	g.player.SetAction(userCharacterAction)
	g.player.Update()

	var keyStrs []string
	var keyNames []string
	for _, k := range g.keys {
		keyStrs = append(keyStrs, k.String())
		if name := ebiten.KeyName(k); name != "" {
			keyNames = append(keyNames, name)
		}
	}
	log.Fatal(strings.Join(keyStrs, ", "))
	log.Fatal(strings.Join(keyNames, ", "))
}

func (g *KeyboardControl) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

func main() {
	if err := ebiten.RunGame(&KeyboardControl{}); err != nil {
		log.Fatal(err)
	}
}

func (g *KeyboardControl) init(screenWidth, screenHeight int) {
	if err := ebiten.RunGame(&KeyboardControl{}); err != nil {
		log.Fatal(err)
	}
	fmt.Println("estoy iniciando el teclado")
}

func NewKeyboardControl(screenWidth, screenHeight int, player characters.MainCharacter) *KeyboardControl {
	kbc := &KeyboardControl{
		player: player,
	}
	kbc.init(screenWidth, screenHeight)
	return kbc
}
