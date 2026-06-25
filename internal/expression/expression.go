package expression

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

// ReplyEnvelope is the structured result requested from OpenClaw. Speaking is
// intentionally absent: it is local playback state, not model-generated state.
type ReplyEnvelope struct {
	Message  string   `json:"message"`
	Emotion  Emotion  `json:"emotion"`
	Activity Activity `json:"activity"`
}

type FaceState struct {
	Emotion  Emotion
	Activity Activity
	Speaking bool
}

func DecodeReplyEnvelope(data []byte) (ReplyEnvelope, error) {
	var raw struct {
		Message  string `json:"message"`
		Emotion  string `json:"emotion"`
		Activity string `json:"activity"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return ReplyEnvelope{}, fmt.Errorf("decode reply envelope: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ReplyEnvelope{}, fmt.Errorf("decode reply envelope: trailing JSON")
	}

	reply := ReplyEnvelope{
		Message:  strings.TrimSpace(raw.Message),
		Emotion:  Emotion(strings.ToLower(strings.TrimSpace(raw.Emotion))),
		Activity: Activity(strings.ToLower(strings.TrimSpace(raw.Activity))),
	}
	var validationErrors []error
	if reply.Message == "" {
		validationErrors = append(validationErrors, errors.New("message is required"))
	}
	if !ValidEmotion(reply.Emotion) {
		validationErrors = append(validationErrors, fmt.Errorf("unsupported emotion %q", raw.Emotion))
	}
	if !ValidActivity(reply.Activity) {
		validationErrors = append(validationErrors, fmt.Errorf("unsupported activity %q", raw.Activity))
	}
	if err := errors.Join(validationErrors...); err != nil {
		return ReplyEnvelope{}, err
	}
	return reply, nil
}

func NormalizeFaceState(state FaceState) FaceState {
	state.Emotion = Emotion(strings.ToLower(strings.TrimSpace(string(state.Emotion))))
	state.Activity = Activity(strings.ToLower(strings.TrimSpace(string(state.Activity))))
	if !ValidEmotion(state.Emotion) {
		state.Emotion = EmotionNeutral
	}
	if !ValidActivity(state.Activity) {
		state.Activity = ActivityNeutral
	}
	return state
}

func ValidEmotion(value Emotion) bool {
	for _, candidate := range Emotions {
		if value == candidate {
			return true
		}
	}
	return false
}

func ValidActivity(value Activity) bool {
	for _, candidate := range Activities {
		if value == candidate {
			return true
		}
	}
	return false
}

// FaceStateInbox is safe for concurrent producers. Its capacity is one: a new
// state replaces an unread stale state.
type FaceStateInbox struct {
	states chan FaceState
	mu     sync.Mutex
}

func NewFaceStateInbox() *FaceStateInbox {
	return &FaceStateInbox{
		states: make(chan FaceState, 1),
	}
}

func (i *FaceStateInbox) Submit(state FaceState) {
	i.mu.Lock()
	defer i.mu.Unlock()

	state = NormalizeFaceState(state)
	select {
	case i.states <- state:
		return
	default:
	}
	select {
	case <-i.states:
	default:
	}
	select {
	case i.states <- state:
	default:
	}
}

func (i *FaceStateInbox) Latest() (FaceState, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()

	var latest FaceState
	found := false
	for {
		select {
		case latest = <-i.states:
			found = true
		default:
			return latest, found
		}
	}
}

// These aliases keep construction source-compatible for embedders while the
// inbox payload has changed from HTTP expression commands to local face state.
type ExpressionInbox = FaceStateInbox

func NewExpressionInbox() *FaceStateInbox {
	return NewFaceStateInbox()
}
