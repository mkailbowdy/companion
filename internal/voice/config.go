package voice

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	defaultWhisperModel = "models/ggml-base.en-q5_0.bin"
	defaultOpenClawURL  = "http://127.0.0.1:18789/v1/responses"
)

type Config struct {
	ArecordCommand  string
	WhisperCommand  string
	OpenClawCommand string
	FFmpegCommand   string
	AplayCommand    string

	WhisperModel   string
	CaptureDevice  string
	PlaybackDevice string
	OpenClawURL    string
	OpenClawModel  string
	OpenClawUser   string
	GatewayToken   string

	StartupTimeout    time.Duration
	WhisperTimeout    time.Duration
	ResponseTimeout   time.Duration
	TTSTimeout        time.Duration
	PlaybackTimeout   time.Duration
	PlaybackCooldown  time.Duration
	FailureStateDelay time.Duration
	TempDir           string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		ArecordCommand:  envOr("BMO_ARECORD_COMMAND", "arecord"),
		WhisperCommand:  envOr("BMO_WHISPER_COMMAND", "whisper-cli"),
		OpenClawCommand: envOr("BMO_OPENCLAW_COMMAND", "openclaw"),
		FFmpegCommand:   envOr("BMO_FFMPEG_COMMAND", "ffmpeg"),
		AplayCommand:    envOr("BMO_APLAY_COMMAND", "aplay"),

		WhisperModel:   envOr("BMO_WHISPER_MODEL", defaultWhisperModel),
		CaptureDevice:  envOr("BMO_CAPTURE_DEVICE", "default"),
		PlaybackDevice: envOr("BMO_PLAYBACK_DEVICE", "default"),
		OpenClawURL:    envOr("BMO_OPENCLAW_URL", defaultOpenClawURL),
		OpenClawModel:  envOr("BMO_OPENCLAW_MODEL", "openclaw/default"),
		OpenClawUser:   envOr("BMO_OPENCLAW_USER", "bmo-rpi"),
		GatewayToken:   os.Getenv("OPENCLAW_GATEWAY_TOKEN"),

		TempDir: os.TempDir(),
	}

	var err error
	if cfg.StartupTimeout, err = envDuration("BMO_STARTUP_TIMEOUT", 15*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.WhisperTimeout, err = envDuration("BMO_WHISPER_TIMEOUT", 90*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.ResponseTimeout, err = envDuration("BMO_RESPONSE_TIMEOUT", 90*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.TTSTimeout, err = envDuration("BMO_TTS_TIMEOUT", 90*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.PlaybackTimeout, err = envDuration("BMO_PLAYBACK_TIMEOUT", 5*time.Minute); err != nil {
		return Config{}, err
	}
	if cfg.PlaybackCooldown, err = envDuration("BMO_PLAYBACK_COOLDOWN", time.Second); err != nil {
		return Config{}, err
	}
	if cfg.FailureStateDelay, err = envDuration("BMO_FAILURE_STATE_DELAY", time.Second); err != nil {
		return Config{}, err
	}
	return cfg, cfg.ValidateStatic()
}

func (c Config) ValidateStatic() error {
	commands := []struct {
		label string
		path  string
	}{
		{"arecord", c.ArecordCommand},
		{"whisper.cpp", c.WhisperCommand},
		{"OpenClaw", c.OpenClawCommand},
		{"ffmpeg", c.FFmpegCommand},
		{"aplay", c.AplayCommand},
	}
	for _, command := range commands {
		if command.path == "" {
			return fmt.Errorf("%s command is not configured", command.label)
		}
		if _, err := exec.LookPath(command.path); err != nil {
			return fmt.Errorf("%s command %q is unavailable: %w", command.label, command.path, err)
		}
	}

	if c.WhisperModel == "" {
		return fmt.Errorf("Whisper model is not configured")
	}
	info, err := os.Stat(c.WhisperModel)
	if err != nil {
		return fmt.Errorf("Whisper model %q is unavailable: %w", c.WhisperModel, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Whisper model %q is not a regular file", c.WhisperModel)
	}
	if c.GatewayToken == "" {
		return fmt.Errorf("OPENCLAW_GATEWAY_TOKEN is required")
	}
	if c.OpenClawURL == "" || c.OpenClawModel == "" || c.OpenClawUser == "" {
		return fmt.Errorf("OpenClaw URL, model, and user must be configured")
	}
	for name, timeout := range map[string]time.Duration{
		"startup": c.StartupTimeout, "Whisper": c.WhisperTimeout,
		"response": c.ResponseTimeout, "TTS": c.TTSTimeout,
		"playback": c.PlaybackTimeout,
	} {
		if timeout <= 0 {
			return fmt.Errorf("%s timeout must be positive", name)
		}
	}
	if c.PlaybackCooldown < 0 || c.FailureStateDelay < 0 {
		return fmt.Errorf("cooldown and failure-state delay cannot be negative")
	}
	if c.TempDir == "" {
		return fmt.Errorf("temporary directory is not configured")
	}
	return nil
}

func (c Config) WhisperModelPath() string {
	if path, err := filepath.Abs(c.WhisperModel); err == nil {
		return path
	}
	return c.WhisperModel
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return duration, nil
}
