package voice

import (
	"math"
	"testing"

	"bmo.pushiro.com/internal/expression"
)

func TestLoadConfigReadsVADSensitivity(t *testing.T) {
	// LoadConfig also validates external commands and services, so test the
	// environment parser directly and verify wiring separately.
	t.Setenv("BMO_VAD_MIN_RMS", "40")
	t.Setenv("BMO_VAD_NOISE_MULTIPLIER", "1.5")

	minimum, err := envFloat("BMO_VAD_MIN_RMS", 60)
	if err != nil {
		t.Fatalf("parse minimum RMS: %v", err)
	}
	multiplier, err := envFloat("BMO_VAD_NOISE_MULTIPLIER", 2)
	if err != nil {
		t.Fatalf("parse noise multiplier: %v", err)
	}
	if minimum != 40 || multiplier != 1.5 {
		t.Fatalf("got minimum=%v multiplier=%v", minimum, multiplier)
	}
}

func TestNewOrchestratorUsesConfiguredVADSensitivity(t *testing.T) {
	cfg := Config{
		VADMinimumSpeechRMS: 40,
		VADNoiseMultiplier:  1.5,
	}
	orchestrator, _, _, err := NewOrchestrator(cfg, expression.NewFaceStateInbox(), nil)
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	listener, ok := orchestrator.Listener.(*ALSAListener)
	if !ok {
		t.Fatalf("listener type = %T", orchestrator.Listener)
	}
	if listener.VAD.config.MinimumSpeechRMS != 40 ||
		listener.VAD.config.NoiseMultiplier != 1.5 {
		t.Fatalf("VAD config = %+v", listener.VAD.config)
	}
}

func TestNewVADRejectsNonFiniteSensitivity(t *testing.T) {
	config := DefaultVADConfig()
	config.MinimumSpeechRMS = math.NaN()
	if _, err := NewVAD(config); err == nil {
		t.Fatal("expected NaN minimum RMS to fail")
	}
	config = DefaultVADConfig()
	config.NoiseMultiplier = math.Inf(1)
	if _, err := NewVAD(config); err == nil {
		t.Fatal("expected infinite noise multiplier to fail")
	}
}
