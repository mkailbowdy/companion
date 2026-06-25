package voice

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

type Transcriber interface {
	Transcribe(context.Context, string) (string, error)
}

type WhisperTranscriber struct {
	Runner  CommandRunner
	Command string
	Model   string
	Threads int
	Timeout time.Duration
}

func (w *WhisperTranscriber) Transcribe(ctx context.Context, wavPath string) (string, error) {
	outputBase := wavPath + ".whisper"
	outputPath := outputBase + ".txt"
	defer os.Remove(outputPath)

	runCtx, cancel := context.WithTimeout(ctx, w.Timeout)
	defer cancel()
	_, err := w.Runner.Run(
		runCtx,
		w.Command,
		"--model", w.Model,
		"--file", wavPath,
		"--threads", fmt.Sprintf("%d", w.Threads),
		"--language", "en",
		"--no-timestamps",
		"--output-txt",
		"--output-file", outputBase,
	)
	if err != nil {
		return "", fmt.Errorf("transcribe audio: %w", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("read Whisper transcript: %w", err)
	}
	transcript := strings.TrimSpace(string(data))
	if transcript == "" {
		return "", fmt.Errorf("Whisper returned an empty transcript")
	}
	return transcript, nil
}
