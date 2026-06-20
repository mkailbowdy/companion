package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"bmo.pushiro.com/internal/expression"
	"bmo.pushiro.com/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func home(w http.ResponseWriter, r *http.Request, inbox *expression.ExpressionInbox){
	var expressionCmd expression.ExpressionCommand
	err := json.NewDecoder(r.Body).Decode(&expressionCmd)
	if err != nil {
		log.Printf("expression warning: decode expression: %v", err)
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	
	fmt.Println(expressionCmd)

	inbox.Submit(expressionCmd)
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
