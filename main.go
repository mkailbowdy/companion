package main

import (
	"log"
	"os"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	screenWidth  = 1280
	screenHeight = 720
)

func main() {
	inbox := NewExpressionInbox()
	game := NewGame(inbox, time.Now, time.Now().UnixNano())

	ebiten.SetWindowTitle("BMO")
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetVsyncEnabled(true)
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	if os.Getenv("BMO_WINDOWED") != "1" {
		ebiten.SetFullscreen(true)
	}

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
