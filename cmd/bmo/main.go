package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"bmo.pushiro.com/internal/expression"
	"bmo.pushiro.com/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func home(w http.ResponseWriter, r *http.Request, inbox *expression.ExpressionInbox){
	cmd, _ := expression.DecodeExpression(io.ReadAll(r.Body))
	inbox.Submit(expression.NormalizeCommand(cmd))
}

func main() {


	// Face Rendering Logic
	inbox := expression.NewExpressionInbox()

	// HTTP server
	go startServer(inbox)	

	g := game.NewGame(inbox, time.Now, time.Now().UnixNano())
	ebiten.SetWindowTitle("BMO")
	ebiten.SetWindowSize(game.ScreenWidth, game.ScreenHeight)
	ebiten.SetVsyncEnabled(true)
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	if os.Getenv("BMO_WINDOWED") != "1" {
		ebiten.SetFullscreen(true)
	}

	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}

}

func startServer(inbox *expression.ExpressionInbox) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request){
		home(w, r, inbox)
	})
	err := http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}
