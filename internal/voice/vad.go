package voice

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
)

const (
	SampleRate      = 16000
	channels        = 1
	bitsPerSample   = 16
	vadFrameSamples = SampleRate / 50 // 20 ms
)

type VADConfig struct {
	PreRollFrames      int
	ActivationFrames   int
	TrailingFrames     int
	MaxUtteranceFrames int
	MinimumSpeechRMS   float64
	NoiseMultiplier    float64
	NoiseSmoothing     float64
}

func DefaultVADConfig() VADConfig {
	return VADConfig{
		PreRollFrames:      20,   // 400 ms
		ActivationFrames:   5,    // 100 ms
		TrailingFrames:     40,   // 800 ms
		MaxUtteranceFrames: 1000, // 20 seconds
		MinimumSpeechRMS:   500,
		NoiseMultiplier:    3,
		NoiseSmoothing:     0.95,
	}
}

type VAD struct {
	config VADConfig
}

func NewVAD(config VADConfig) (*VAD, error) {
	if config.PreRollFrames < config.ActivationFrames ||
		config.ActivationFrames <= 0 ||
		config.TrailingFrames <= 0 ||
		config.MaxUtteranceFrames <= config.PreRollFrames ||
		config.MinimumSpeechRMS <= 0 ||
		config.NoiseMultiplier <= 1 ||
		config.NoiseSmoothing < 0 ||
		config.NoiseSmoothing >= 1 {
		return nil, errors.New("invalid VAD configuration")
	}
	return &VAD{config: config}, nil
}

func (v *VAD) Detect(ctx context.Context, input io.Reader, onSpeech func()) ([]int16, error) {
	frameBytes := make([]byte, vadFrameSamples*2)
	preRoll := make([][]int16, 0, v.config.PreRollFrames)
	utterance := make([]int16, 0, v.config.MaxUtteranceFrames*vadFrameSamples)
	noiseFloor := v.config.MinimumSpeechRMS / v.config.NoiseMultiplier
	activationFrames := 0
	silenceFrames := 0
	started := false

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		n, err := io.ReadFull(input, frameBytes)
		if err != nil {
			if started && (errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)) {
				if n >= 2 {
					utterance = append(utterance, decodePCM(frameBytes[:n-n%2])...)
				}
				return utterance, nil
			}
			return nil, err
		}

		frame := decodePCM(frameBytes)
		rms := frameRMS(frame)
		threshold := math.Max(v.config.MinimumSpeechRMS, noiseFloor*v.config.NoiseMultiplier)
		isSpeech := rms >= threshold

		if !started {
			preRoll = append(preRoll, frame)
			if len(preRoll) > v.config.PreRollFrames {
				preRoll = preRoll[1:]
			}
			if isSpeech {
				activationFrames++
			} else {
				activationFrames = 0
				noiseFloor = smoothNoise(noiseFloor, rms, v.config.NoiseSmoothing)
			}
			if activationFrames < v.config.ActivationFrames {
				continue
			}

			started = true
			for _, buffered := range preRoll {
				utterance = append(utterance, buffered...)
			}
			preRoll = nil
			if onSpeech != nil {
				onSpeech()
			}
		} else {
			utterance = append(utterance, frame...)
		}

		if isSpeech {
			silenceFrames = 0
		} else {
			silenceFrames++
		}
		if silenceFrames >= v.config.TrailingFrames ||
			len(utterance) >= v.config.MaxUtteranceFrames*vadFrameSamples {
			maxSamples := v.config.MaxUtteranceFrames * vadFrameSamples
			if len(utterance) > maxSamples {
				utterance = utterance[:maxSamples]
			}
			return utterance, nil
		}
	}
}

func decodePCM(data []byte) []int16 {
	samples := make([]int16, len(data)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return samples
}

func frameRMS(samples []int16) float64 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, sample := range samples {
		value := float64(sample)
		sum += value * value
	}
	return math.Sqrt(sum / float64(len(samples)))
}

func smoothNoise(current, sample, smoothing float64) float64 {
	return smoothing*current + (1-smoothing)*sample
}
