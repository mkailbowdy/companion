package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

type Listener interface {
	Listen(context.Context, func()) ([]int16, error)
}

type ALSAListener struct {
	Command string
	Device  string
	VAD     *VAD
}

func (l *ALSAListener) Listen(ctx context.Context, onSpeech func()) ([]int16, error) {
	if l.VAD == nil {
		return nil, fmt.Errorf("audio capture: VAD is nil")
	}
	captureCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := commandContext(
		captureCtx,
		l.Command,
		"-q",
		"-D", l.Device,
		"-f", "S16_LE",
		"-r", "16000",
		"-c", "1",
		"-t", "raw",
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("audio capture stdout: %w", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start audio capture: %w", err)
	}

	samples, detectErr := l.VAD.Detect(captureCtx, stdout, onSpeech)
	cancel()
	waitErr := cmd.Wait()
	if detectErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if waitErr != nil && !isExpectedPipeClosure(detectErr) {
			return nil, fmt.Errorf("audio capture failed: %w", waitErr)
		}
		return nil, fmt.Errorf("audio capture: %w", detectErr)
	}
	return samples, nil
}

func isExpectedPipeClosure(err error) bool {
	return err == io.EOF || err == io.ErrUnexpectedEOF
}
