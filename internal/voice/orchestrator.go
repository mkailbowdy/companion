package voice

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"bmo.pushiro.com/internal/expression"
)

type FaceStateSink interface {
	Submit(expression.FaceState)
}

type Orchestrator struct {
	Listener     Listener
	Transcriber  Transcriber
	Responder    Responder
	Speaker      Speaker
	States       FaceStateSink
	Logger       *log.Logger
	TempDir      string
	Cooldown     time.Duration
	FailureDelay time.Duration

	currentEmotion expression.Emotion
}

func (o *Orchestrator) Run(ctx context.Context) error {
	if err := o.validate(); err != nil {
		return err
	}
	o.currentEmotion = expression.EmotionNeutral

	for {
		err := o.runTurn(ctx)
		if err == nil {
			if err := waitContext(ctx, o.Cooldown); err != nil {
				return nil
			}
			continue
		}
		if ctx.Err() != nil &&
			(errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			return nil
		}
		if errors.Is(err, io.EOF) {
			return nil
		}

		o.Logger.Printf("conversation turn failed: %v", err)
		o.States.Submit(expression.FaceState{
			Emotion:  expression.EmotionConfused,
			Activity: expression.ActivityNeutral,
		})
		if err := waitContext(ctx, o.FailureDelay); err != nil {
			return nil
		}
		o.States.Submit(expression.FaceState{
			Emotion:  o.currentEmotion,
			Activity: expression.ActivityNeutral,
		})
	}
}

func (o *Orchestrator) runTurn(ctx context.Context) error {
	samples, err := o.Listener.Listen(ctx, func() {
		o.States.Submit(expression.FaceState{
			Emotion:  o.currentEmotion,
			Activity: expression.ActivityListening,
		})
	})
	if err != nil {
		return err
	}
	if len(samples) == 0 {
		return fmt.Errorf("voice activity detector returned no audio")
	}

	o.States.Submit(expression.FaceState{
		Emotion:  o.currentEmotion,
		Activity: expression.ActivityThinking,
	})

	wavPath, err := tempPath(o.TempDir, "bmo-utterance-*.wav")
	if err != nil {
		return err
	}
	defer os.Remove(wavPath)
	if err := WriteWAV(wavPath, samples); err != nil {
		return err
	}
	transcript, err := o.Transcriber.Transcribe(ctx, wavPath)
	if err != nil {
		return err
	}
	reply, err := o.Responder.Respond(ctx, transcript)
	if err != nil {
		return err
	}

	o.States.Submit(expression.FaceState{
		Emotion:  reply.Emotion,
		Activity: reply.Activity,
		Speaking: true,
	})
	if err := o.Speaker.Speak(ctx, reply.Message); err != nil {
		return err
	}

	o.currentEmotion = reply.Emotion
	o.States.Submit(expression.FaceState{
		Emotion:  reply.Emotion,
		Activity: expression.ActivityNeutral,
	})
	return nil
}

func (o *Orchestrator) validate() error {
	switch {
	case o.Listener == nil:
		return fmt.Errorf("conversation listener is nil")
	case o.Transcriber == nil:
		return fmt.Errorf("conversation transcriber is nil")
	case o.Responder == nil:
		return fmt.Errorf("conversation responder is nil")
	case o.Speaker == nil:
		return fmt.Errorf("conversation speaker is nil")
	case o.States == nil:
		return fmt.Errorf("conversation face-state sink is nil")
	case o.TempDir == "":
		return fmt.Errorf("conversation temporary directory is empty")
	}
	if o.Logger == nil {
		o.Logger = log.Default()
	}
	return nil
}

func waitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
