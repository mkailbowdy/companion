package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestVADDetectsSpeechWithPreRollAndTrailingSilence(t *testing.T) {
	vad, err := NewVAD(DefaultVADConfig())
	if err != nil {
		t.Fatal(err)
	}
	input := pcmFrames(
		frames(30, 20),
		frames(10, 100),
		frames(40, 0),
	)
	started := 0
	samples, err := vad.Detect(context.Background(), bytes.NewReader(input), func() { started++ })
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if started != 1 {
		t.Fatalf("speech callback count = %d, want 1", started)
	}
	wantFrames := 20 + 5 + 40
	if got := len(samples) / vadFrameSamples; got != wantFrames {
		t.Fatalf("captured %d frames, want %d", got, wantFrames)
	}
	if samples[0] != 20 {
		t.Fatalf("pre-roll starts with %d, want noise sample", samples[0])
	}
}

func TestVADAdaptsToSteadyBackgroundNoise(t *testing.T) {
	vad, err := NewVAD(DefaultVADConfig())
	if err != nil {
		t.Fatal(err)
	}
	input := pcmFrames(
		frames(200, 20),
		frames(5, 35),
		frames(8, 100),
		frames(40, 20),
	)
	started := false
	samples, err := vad.Detect(context.Background(), bytes.NewReader(input), func() { started = true })
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !started || len(samples) == 0 {
		t.Fatal("expected loud speech to activate VAD")
	}
}

func TestVADDetectsLowLevelVoiceHATSpeech(t *testing.T) {
	vad, err := NewVAD(DefaultVADConfig())
	if err != nil {
		t.Fatal(err)
	}
	input := pcmFrames(
		frames(50, 20),
		frames(10, 100),
		frames(40, 20),
	)
	started := false
	samples, err := vad.Detect(context.Background(), bytes.NewReader(input), func() { started = true })
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !started || len(samples) == 0 {
		t.Fatal("expected low-level VoiceHAT speech to activate VAD")
	}
}

func TestVADStopsAtMaximumUtterance(t *testing.T) {
	config := DefaultVADConfig()
	config.MaxUtteranceFrames = 30
	vad, err := NewVAD(config)
	if err != nil {
		t.Fatal(err)
	}
	input := pcmFrames(frames(10, 0), frames(100, 3000))
	samples, err := vad.Detect(context.Background(), bytes.NewReader(input), nil)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got, want := len(samples), config.MaxUtteranceFrames*vadFrameSamples; got != want {
		t.Fatalf("captured %d samples, want %d", got, want)
	}
}

func TestVADReturnsEOFWithoutSpeech(t *testing.T) {
	vad, err := NewVAD(DefaultVADConfig())
	if err != nil {
		t.Fatal(err)
	}
	_, err = vad.Detect(context.Background(), bytes.NewReader(pcmFrames(frames(20, 0))), nil)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("got %v, want EOF", err)
	}
}

func frames(count int, amplitude int16) [][]int16 {
	result := make([][]int16, count)
	for i := range result {
		result[i] = make([]int16, vadFrameSamples)
		for j := range result[i] {
			result[i][j] = amplitude
		}
	}
	return result
}

func pcmFrames(groups ...[][]int16) []byte {
	var buffer bytes.Buffer
	for _, group := range groups {
		for _, frame := range group {
			_ = binary.Write(&buffer, binary.LittleEndian, frame)
		}
	}
	return buffer.Bytes()
}
