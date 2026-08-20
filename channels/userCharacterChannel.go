package channels

import "github.com/hajimehoshi/ebiten/v2"

type UserCharacterChannel struct {
	currentAction int
}

const Nothig = 0
const Up = 1
const Left = 2
const Right = 3
const Down = 4

func (ucc *UserCharacterChannel) GetCurrentAction() int {
	return ucc.currentAction
}

func (ucc *UserCharacterChannel) SetActionFromKeyBoard(key ebiten.Key) {
	//innerKey := key.String()
	ucc.currentAction = Up
	// if innerKey == "w" {
	// 	ucc.currentAction = Up
	// }
	// if innerKey == "a" {
	// 	ucc.currentAction = Left
	// }
	// if innerKey == "s" {
	// 	ucc.currentAction = Down
	// }
	// if innerKey == "d" {
	// 	ucc.currentAction = Right
	// } else {
	// 	ucc.currentAction = Nothig
	// }

}

func NewUserCharacterChannel() UserCharacterChannel {
	ucc := UserCharacterChannel{currentAction: Nothig}
	return ucc
}
