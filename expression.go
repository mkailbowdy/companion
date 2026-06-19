package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

type Emotion string

const (
	EmotionNeutral   Emotion = "neutral"
	EmotionHappy     Emotion = "happy"
	EmotionSad       Emotion = "sad"
	EmotionAngry     Emotion = "angry"
	EmotionSurprised Emotion = "surprised"
	EmotionScared    Emotion = "scared"
	EmotionConfused  Emotion = "confused"
	EmotionSleepy    Emotion = "sleepy"
	EmotionExcited   Emotion = "excited"
)

var Emotions = []Emotion{
	EmotionNeutral, EmotionHappy, EmotionSad, EmotionAngry, EmotionSurprised,
	EmotionScared, EmotionConfused, EmotionSleepy, EmotionExcited,
}

type Activity string

const (
	ActivityNeutral   Activity = "neutral"
	ActivityBlinking  Activity = "blinking"
	ActivityTalking   Activity = "talking"
	ActivityLaughing  Activity = "laughing"
	ActivityCrying    Activity = "crying"
	ActivityThinking  Activity = "thinking"
	ActivityListening Activity = "listening"
)

var Activities = []Activity{
	ActivityNeutral, ActivityBlinking, ActivityTalking, ActivityLaughing,
	ActivityCrying, ActivityThinking, ActivityListening,
}

type ExpressionCommand struct {
	Emotion  Emotion  `json:"emotion"`
	Activity Activity `json:"activity"`
}

func DecodeExpression(data []byte) (ExpressionCommand, error) {
	var raw struct {
		Emotion  string `json:"emotion"`
		Activity string `json:"activity"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ExpressionCommand{}, fmt.Errorf("decode expression: %w", err)
	}

	cmd := ExpressionCommand{
		Emotion:  Emotion(strings.ToLower(strings.TrimSpace(raw.Emotion))),
		Activity: Activity(strings.ToLower(strings.TrimSpace(raw.Activity))),
	}
	var warnings []error
	if !validEmotion(cmd.Emotion) {
		warnings = append(warnings, fmt.Errorf("unsupported emotion %q; using neutral", raw.Emotion))
		cmd.Emotion = EmotionNeutral
	}
	if !validActivity(cmd.Activity) {
		warnings = append(warnings, fmt.Errorf("unsupported activity %q; using neutral", raw.Activity))
		cmd.Activity = ActivityNeutral
	}
	return cmd, errors.Join(warnings...)
}

func validEmotion(value Emotion) bool {
	for _, candidate := range Emotions {
		if value == candidate {
			return true
		}
	}
	return false
}

func validActivity(value Activity) bool {
	for _, candidate := range Activities {
		if value == candidate {
			return true
		}
	}
	return false
}

// ExpressionInbox is safe for concurrent producers. Its capacity is one:
// a new command replaces an unread stale command.
type ExpressionInbox struct {
	commands chan ExpressionCommand
	mu       sync.Mutex
}

func NewExpressionInbox() *ExpressionInbox {
	return &ExpressionInbox{commands: make(chan ExpressionCommand, 1)}
}

func (i *ExpressionInbox) Submit(cmd ExpressionCommand) {
	i.mu.Lock()
	defer i.mu.Unlock()

	cmd = normalizeCommand(cmd)
	select {
	case i.commands <- cmd:
		return
	default:
	}
	select {
	case <-i.commands:
	default:
	}
	select {
	case i.commands <- cmd:
	default:
	}
}

func (i *ExpressionInbox) Latest() (ExpressionCommand, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()

	var latest ExpressionCommand
	found := false
	for {
		select {
		case latest = <-i.commands:
			found = true
		default:
			return latest, found
		}
	}
}

func normalizeCommand(cmd ExpressionCommand) ExpressionCommand {
	cmd.Emotion = Emotion(strings.ToLower(strings.TrimSpace(string(cmd.Emotion))))
	cmd.Activity = Activity(strings.ToLower(strings.TrimSpace(string(cmd.Activity))))
	if !validEmotion(cmd.Emotion) {
		cmd.Emotion = EmotionNeutral
	}
	if !validActivity(cmd.Activity) {
		cmd.Activity = ActivityNeutral
	}
	return cmd
}
