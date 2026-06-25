package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bmo.pushiro.com/internal/expression"
	"bmo.pushiro.com/internal/game"
	"bmo.pushiro.com/internal/voice"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := voice.LoadConfig()
	if err != nil {
		return err
	}

	inbox := expression.NewFaceStateInbox()
	conversation, client, speaker, err := voice.NewOrchestrator(cfg, inbox, log.Default())
	if err != nil {
		return err
	}
	startupCtx, cancelStartup := context.WithTimeout(ctx, cfg.StartupTimeout)
	err = voice.ValidateServices(startupCtx, client, speaker)
	cancelStartup()
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return err
	}

	conversationErr := make(chan error, 1)
	go func() {
		conversationErr <- conversation.Run(ctx)
		stop()
	}()

	g := game.NewGame(inbox, time.Now, time.Now().UnixNano())
	g.SetDone(ctx.Done())
	ebiten.SetWindowTitle("BMO")
	ebiten.SetWindowSize(game.ScreenWidth, game.ScreenHeight)
	ebiten.SetVsyncEnabled(true)
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	if os.Getenv("BMO_WINDOWED") != "1" {
		ebiten.SetFullscreen(true)
	}

	gameErr := ebiten.RunGame(g)
	stop()
	err = <-conversationErr
	if gameErr != nil && !errors.Is(gameErr, ebiten.Termination) {
		return gameErr
	}
	return err
}
