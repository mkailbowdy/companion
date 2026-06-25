package voice

import (
	"context"
	"io"
	"log"
	"slices"
	"testing"
	"time"

	"bmo.pushiro.com/internal/expression"
)

type listenerFunc func(context.Context, func()) ([]int16, error)

func (f listenerFunc) Listen(ctx context.Context, onSpeech func()) ([]int16, error) {
	return f(ctx, onSpeech)
}

type transcriberFunc func(context.Context, string) (string, error)

func (f transcriberFunc) Transcribe(ctx context.Context, path string) (string, error) {
	return f(ctx, path)
}

type responderFunc func(context.Context, string) (expression.ReplyEnvelope, error)

func (f responderFunc) Respond(ctx context.Context, text string) (expression.ReplyEnvelope, error) {
	return f(ctx, text)
}

type speakerFunc func(context.Context, string, func()) error

func (f speakerFunc) Speak(ctx context.Context, text string, onPlaybackStart func()) error {
	return f(ctx, text, onPlaybackStart)
}

type stateRecorder struct {
	states []expression.FaceState
}

func (r *stateRecorder) Submit(state expression.FaceState) {
	r.states = append(r.states, state)
}

func TestOrchestratorCompleteTurnStateProgression(t *testing.T) {
	states := &stateRecorder{}
	orchestrator := &Orchestrator{
		Listener: listenerFunc(func(_ context.Context, onSpeech func()) ([]int16, error) {
			onSpeech()
			return []int16{1, 2, 3}, nil
		}),
		Transcriber: transcriberFunc(func(_ context.Context, _ string) (string, error) {
			return "How are you?", nil
		}),
		Responder: responderFunc(func(_ context.Context, text string) (expression.ReplyEnvelope, error) {
			if text != "How are you?" {
				t.Fatalf("transcript = %q", text)
			}
			return expression.ReplyEnvelope{
				Message: "Algebraic!", Emotion: expression.EmotionHappy, Activity: expression.ActivityLaughing,
			}, nil
		}),
		Speaker: speakerFunc(func(_ context.Context, text string, onPlaybackStart func()) error {
			if text != "Algebraic!" {
				t.Fatalf("speech = %q", text)
			}
			if got := states.states[len(states.states)-1].Activity; got != expression.ActivityThinking {
				t.Fatalf("state before playback = %q, want thinking", got)
			}
			onPlaybackStart()
			return nil
		}),
		States:         states,
		Logger:         log.New(io.Discard, "", 0),
		TempDir:        t.TempDir(),
		currentEmotion: expression.EmotionNeutral,
	}
	if err := orchestrator.runTurn(context.Background()); err != nil {
		t.Fatalf("runTurn: %v", err)
	}
	want := []expression.FaceState{
		{Emotion: expression.EmotionNeutral, Activity: expression.ActivityListening},
		{Emotion: expression.EmotionNeutral, Activity: expression.ActivityThinking},
		{Emotion: expression.EmotionHappy, Activity: expression.ActivityLaughing, Speaking: true},
		{Emotion: expression.EmotionHappy, Activity: expression.ActivityNeutral},
	}
	if !slices.Equal(states.states, want) {
		t.Fatalf("states = %+v, want %+v", states.states, want)
	}
	if orchestrator.currentEmotion != expression.EmotionHappy {
		t.Fatalf("retained emotion = %q", orchestrator.currentEmotion)
	}
}

func TestOrchestratorDoesNotAnimateSpeakingBeforePlaybackStarts(t *testing.T) {
	states := &stateRecorder{}
	orchestrator := &Orchestrator{
		Listener: listenerFunc(func(_ context.Context, onSpeech func()) ([]int16, error) {
			onSpeech()
			return []int16{1}, nil
		}),
		Transcriber: transcriberFunc(func(context.Context, string) (string, error) {
			return "hello", nil
		}),
		Responder: responderFunc(func(context.Context, string) (expression.ReplyEnvelope, error) {
			return expression.ReplyEnvelope{
				Message: "hi", Emotion: expression.EmotionHappy, Activity: expression.ActivityTalking,
			}, nil
		}),
		Speaker: speakerFunc(func(context.Context, string, func()) error {
			return io.ErrUnexpectedEOF
		}),
		States:         states,
		Logger:         log.New(io.Discard, "", 0),
		TempDir:        t.TempDir(),
		currentEmotion: expression.EmotionNeutral,
	}
	if err := orchestrator.runTurn(context.Background()); err == nil {
		t.Fatal("expected speaker preparation failure")
	}
	for _, state := range states.states {
		if state.Speaking {
			t.Fatalf("speaking activated before playback: %+v", states.states)
		}
	}
}

func TestOrchestratorFailureShowsConfusedAndResumes(t *testing.T) {
	states := &stateRecorder{}
	calls := 0
	orchestrator := &Orchestrator{
		Listener: listenerFunc(func(_ context.Context, _ func()) ([]int16, error) {
			calls++
			if calls == 1 {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, io.EOF
		}),
		Transcriber: transcriberFunc(func(context.Context, string) (string, error) { return "", nil }),
		Responder: responderFunc(func(context.Context, string) (expression.ReplyEnvelope, error) {
			return expression.ReplyEnvelope{}, nil
		}),
		Speaker: speakerFunc(func(_ context.Context, _ string, onPlaybackStart func()) error {
			onPlaybackStart()
			return nil
		}),
		States:  states,
		Logger:  log.New(io.Discard, "", 0),
		TempDir: t.TempDir(),
	}
	if err := orchestrator.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []expression.FaceState{
		{Emotion: expression.EmotionConfused, Activity: expression.ActivityNeutral},
		{Emotion: expression.EmotionNeutral, Activity: expression.ActivityNeutral},
	}
	if !slices.Equal(states.states, want) {
		t.Fatalf("states = %+v, want %+v", states.states, want)
	}
}

func TestOrchestratorWaitsForPlaybackCooldown(t *testing.T) {
	var firstFinished time.Time
	var secondListen time.Time
	calls := 0
	orchestrator := &Orchestrator{
		Listener: listenerFunc(func(_ context.Context, onSpeech func()) ([]int16, error) {
			calls++
			if calls == 1 {
				onSpeech()
				return []int16{1}, nil
			}
			secondListen = time.Now()
			return nil, io.EOF
		}),
		Transcriber: transcriberFunc(func(context.Context, string) (string, error) {
			return "hello", nil
		}),
		Responder: responderFunc(func(context.Context, string) (expression.ReplyEnvelope, error) {
			return expression.ReplyEnvelope{
				Message: "hi", Emotion: expression.EmotionHappy, Activity: expression.ActivityNeutral,
			}, nil
		}),
		Speaker: speakerFunc(func(_ context.Context, _ string, onPlaybackStart func()) error {
			onPlaybackStart()
			firstFinished = time.Now()
			return nil
		}),
		States:       &stateRecorder{},
		Logger:       log.New(io.Discard, "", 0),
		TempDir:      t.TempDir(),
		Cooldown:     25 * time.Millisecond,
		FailureDelay: 0,
	}
	if err := orchestrator.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed := secondListen.Sub(firstFinished); elapsed < orchestrator.Cooldown {
		t.Fatalf("second capture started after %v, want at least %v", elapsed, orchestrator.Cooldown)
	}
}
