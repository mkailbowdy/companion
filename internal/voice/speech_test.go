package voice

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestOpenClawSpeakerRunsTTSConversionAndPlayback(t *testing.T) {
	var commands []string
	playbackStarted := false
	speaker := &OpenClawSpeaker{
		OpenClawCommand: "openclaw",
		FFmpegCommand:   "ffmpeg",
		AplayCommand:    "aplay",
		PlaybackDevice:  "hw:1,0",
		TTSTimeout:      time.Second,
		PlaybackTimeout: time.Second,
		TempDir:         t.TempDir(),
		Runner: runnerFunc(func(_ context.Context, name string, args ...string) ([]byte, error) {
			commands = append(commands, name)
			switch name {
			case "openclaw":
				index := slices.Index(args, "--output")
				if index < 0 {
					t.Fatal("missing TTS output")
				}
				if !slices.Contains(args, "--text") || !slices.Contains(args, "hello") {
					t.Fatalf("unexpected TTS args: %v", args)
				}
				result := `{"ok":true,"provider":"elevenlabs","outputs":[{"path":"` + args[index+1] + `"}]}`
				return []byte(result), os.WriteFile(args[index+1], []byte("mp3"), 0o600)
			case "ffmpeg":
				if playbackStarted {
					t.Fatal("playback callback ran before audio conversion finished")
				}
				return nil, os.WriteFile(args[len(args)-1], []byte("wav"), 0o600)
			case "aplay":
				if !playbackStarted {
					t.Fatal("playback callback did not run before aplay")
				}
				if !slices.Contains(args, "hw:1,0") {
					t.Fatalf("unexpected playback args: %v", args)
				}
			}
			return nil, nil
		}),
	}
	if err := speaker.Speak(context.Background(), "hello", func() {
		playbackStarted = true
	}); err != nil {
		t.Fatalf("Speak: %v", err)
	}
	if got, want := commands, []string{"openclaw", "ffmpeg", "aplay"}; !slices.Equal(got, want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	entries, err := os.ReadDir(speaker.TempDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, entry := range entries {
			names[i] = filepath.Base(entry.Name())
		}
		t.Fatalf("temporary files remain: %v", names)
	}
}

func TestOpenClawSpeakerValidateRequiresElevenLabs(t *testing.T) {
	speaker := &OpenClawSpeaker{
		OpenClawCommand: "openclaw",
		Runner: runnerFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return []byte(`{"ok":true,"provider":"elevenlabs"}`), nil
		}),
	}
	if err := speaker.Validate(context.Background()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	speaker.Runner = runnerFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(`{"ok":true,"provider":"openai"}`), nil
	})
	if err := speaker.Validate(context.Background()); err == nil {
		t.Fatal("expected non-ElevenLabs configuration to fail")
	}
}
