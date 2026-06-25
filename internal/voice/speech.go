package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Speaker interface {
	Speak(context.Context, string) error
}

type OpenClawSpeaker struct {
	Runner          CommandRunner
	OpenClawCommand string
	FFmpegCommand   string
	AplayCommand    string
	PlaybackDevice  string
	TTSTimeout      time.Duration
	PlaybackTimeout time.Duration
	TempDir         string
}

func (s *OpenClawSpeaker) Validate(ctx context.Context) error {
	output, err := s.Runner.Run(ctx, s.OpenClawCommand, "infer", "tts", "status", "--json")
	if err != nil {
		return fmt.Errorf("check OpenClaw TTS configuration: %w", err)
	}
	var status any
	if err := json.Unmarshal(output, &status); err != nil {
		return fmt.Errorf("OpenClaw TTS status returned invalid JSON: %w", err)
	}
	if jsonReportsFailure(status) {
		return fmt.Errorf("OpenClaw TTS status reported failure")
	}
	if !jsonMentionsProvider(status, "elevenlabs") {
		return fmt.Errorf("OpenClaw TTS is not configured to use ElevenLabs")
	}
	return nil
}

func (s *OpenClawSpeaker) Speak(ctx context.Context, text string) error {
	requestedPath, err := tempPath(s.TempDir, "bmo-tts-*.mp3")
	if err != nil {
		return err
	}
	defer os.Remove(requestedPath)
	wavPath := strings.TrimSuffix(requestedPath, filepath.Ext(requestedPath)) + ".wav"
	defer os.Remove(wavPath)

	ttsCtx, cancelTTS := context.WithTimeout(ctx, s.TTSTimeout)
	output, err := s.Runner.Run(
		ttsCtx,
		s.OpenClawCommand,
		"infer", "tts", "convert",
		"--text", text,
		"--output", requestedPath,
		"--json",
	)
	cancelTTS()
	if err != nil {
		return fmt.Errorf("generate speech: %w", err)
	}
	result, err := decodeInferResult(output, "OpenClaw TTS")
	if err != nil {
		return err
	}
	if !strings.EqualFold(result.Provider, "elevenlabs") {
		return fmt.Errorf("OpenClaw TTS used provider %q, want ElevenLabs", result.Provider)
	}
	sourcePath := requestedPath
	if len(result.Outputs) > 0 && result.Outputs[0].Path != "" {
		sourcePath = result.Outputs[0].Path
	}
	if !temporaryTTSPath(sourcePath, s.TempDir) {
		return fmt.Errorf("OpenClaw TTS returned an unexpected output path %q", sourcePath)
	}
	defer os.Remove(sourcePath)
	if _, err := os.Stat(sourcePath); err != nil {
		return fmt.Errorf("OpenClaw TTS did not create %q: %w", sourcePath, err)
	}

	convertCtx, cancelConvert := context.WithTimeout(ctx, s.TTSTimeout)
	_, err = s.Runner.Run(
		convertCtx,
		s.FFmpegCommand,
		"-nostdin", "-y", "-loglevel", "error",
		"-i", sourcePath,
		"-ac", "1",
		"-c:a", "pcm_s16le",
		wavPath,
	)
	cancelConvert()
	if err != nil {
		return fmt.Errorf("convert speech audio: %w", err)
	}

	playCtx, cancelPlay := context.WithTimeout(ctx, s.PlaybackTimeout)
	_, err = s.Runner.Run(playCtx, s.AplayCommand, "-q", "-D", s.PlaybackDevice, wavPath)
	cancelPlay()
	if err != nil {
		return fmt.Errorf("play speech audio: %w", err)
	}
	return nil
}

func tempPath(dir, pattern string) (string, error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("create temporary audio path: %w", err)
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(path)
		return "", fmt.Errorf("close temporary audio path: %w", err)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("prepare temporary audio path: %w", err)
	}
	return path, nil
}

type inferResult struct {
	OK       bool   `json:"ok"`
	Provider string `json:"provider"`
	Outputs  []struct {
		Path string `json:"path"`
	} `json:"outputs"`
}

func decodeInferResult(data []byte, operation string) (inferResult, error) {
	var response inferResult
	if err := json.Unmarshal(data, &response); err != nil {
		return inferResult{}, fmt.Errorf("%s returned invalid JSON: %w", operation, err)
	}
	if !response.OK {
		return inferResult{}, fmt.Errorf("%s reported failure", operation)
	}
	if response.Provider == "" {
		return inferResult{}, fmt.Errorf("%s did not report its provider", operation)
	}
	return response, nil
}

func temporaryTTSPath(path, tempDir string) bool {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absoluteDir, err := filepath.Abs(tempDir)
	if err != nil {
		return false
	}
	return filepath.Dir(absolutePath) == absoluteDir &&
		strings.HasPrefix(filepath.Base(absolutePath), "bmo-tts-")
}

func jsonMentionsProvider(value any, provider string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if strings.Contains(strings.ToLower(key), "provider") {
				if name, ok := child.(string); ok && strings.EqualFold(name, provider) {
					return true
				}
			}
			if jsonMentionsProvider(child, provider) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if jsonMentionsProvider(child, provider) {
				return true
			}
		}
	}
	return false
}

func jsonReportsFailure(value any) bool {
	object, ok := value.(map[string]any)
	if !ok {
		return false
	}
	okValue, exists := object["ok"]
	if !exists {
		return false
	}
	result, valid := okValue.(bool)
	return valid && !result
}
