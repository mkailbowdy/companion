package voice

import (
	"context"
	"fmt"
	"log"

	"bmo.pushiro.com/internal/expression"
)

func NewOrchestrator(cfg Config, states *expression.FaceStateInbox, logger *log.Logger) (*Orchestrator, *OpenClawClient, *OpenClawSpeaker, error) {
	vadConfig := DefaultVADConfig()
	vadConfig.MinimumSpeechRMS = cfg.VADMinimumSpeechRMS
	vadConfig.NoiseMultiplier = cfg.VADNoiseMultiplier
	vad, err := NewVAD(vadConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	runner := ExecRunner{}
	client := &OpenClawClient{
		URL:     cfg.OpenClawURL,
		Model:   cfg.OpenClawModel,
		User:    cfg.OpenClawUser,
		Token:   cfg.GatewayToken,
		Timeout: cfg.ResponseTimeout,
	}
	speaker := &OpenClawSpeaker{
		Runner:          runner,
		OpenClawCommand: cfg.OpenClawCommand,
		FFmpegCommand:   cfg.FFmpegCommand,
		AplayCommand:    cfg.AplayCommand,
		PlaybackDevice:  cfg.PlaybackDevice,
		TTSTimeout:      cfg.TTSTimeout,
		PlaybackTimeout: cfg.PlaybackTimeout,
		TempDir:         cfg.TempDir,
	}
	orchestrator := &Orchestrator{
		Listener: &ALSAListener{
			Command: cfg.ArecordCommand,
			Device:  cfg.CaptureDevice,
			VAD:     vad,
		},
		Transcriber: &WhisperTranscriber{
			Runner:  runner,
			Command: cfg.WhisperCommand,
			Model:   cfg.WhisperModelPath(),
			Threads: 4,
			Timeout: cfg.WhisperTimeout,
		},
		Responder:    client,
		Speaker:      speaker,
		States:       states,
		Logger:       logger,
		TempDir:      cfg.TempDir,
		Cooldown:     cfg.PlaybackCooldown,
		FailureDelay: cfg.FailureStateDelay,
	}
	return orchestrator, client, speaker, nil
}

func ValidateServices(ctx context.Context, client *OpenClawClient, speaker *OpenClawSpeaker) error {
	if err := client.Validate(ctx); err != nil {
		return fmt.Errorf("OpenClaw gateway validation failed: %w", err)
	}
	if err := speaker.Validate(ctx); err != nil {
		return fmt.Errorf("OpenClaw ElevenLabs validation failed: %w", err)
	}
	return nil
}
