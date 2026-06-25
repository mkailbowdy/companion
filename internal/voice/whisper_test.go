package voice

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"
)

type runnerFunc func(context.Context, string, ...string) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return f(ctx, name, args...)
}

func TestWhisperTranscriberWritesAndReadsTextOutput(t *testing.T) {
	tempDir := t.TempDir()
	wavPath := tempDir + "/input.wav"
	if err := os.WriteFile(wavPath, []byte("wav"), 0o600); err != nil {
		t.Fatal(err)
	}
	var gotCommand string
	var gotArgs []string
	transcriber := &WhisperTranscriber{
		Command: "whisper-cli",
		Model:   "/models/base.bin",
		Threads: 4,
		Timeout: time.Second,
		Runner: runnerFunc(func(_ context.Context, name string, args ...string) ([]byte, error) {
			gotCommand = name
			gotArgs = append([]string(nil), args...)
			index := slices.Index(args, "--output-file")
			if index < 0 {
				t.Fatal("missing --output-file")
			}
			return nil, os.WriteFile(args[index+1]+".txt", []byte("  hello BMO  \n"), 0o600)
		}),
	}
	transcript, err := transcriber.Transcribe(context.Background(), wavPath)
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if transcript != "hello BMO" {
		t.Fatalf("transcript = %q", transcript)
	}
	if gotCommand != "whisper-cli" {
		t.Fatalf("command = %q", gotCommand)
	}
	for _, expected := range []string{"--model", "/models/base.bin", "--threads", "4", "--language", "en"} {
		if !slices.Contains(gotArgs, expected) {
			t.Errorf("arguments %v do not contain %q", gotArgs, expected)
		}
	}
	if _, err := os.Stat(wavPath + ".whisper.txt"); !os.IsNotExist(err) {
		t.Fatalf("transcript sidecar was not removed: %v", err)
	}
}
