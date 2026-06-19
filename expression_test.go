package main

import (
	"strings"
	"testing"
)

func TestDecodeExpression(t *testing.T) {
	cmd, err := DecodeExpression([]byte(`{"emotion":" HAPPY ","activity":"Laughing"}`))
	if err != nil {
		t.Fatalf("unexpected warning: %v", err)
	}
	if cmd.Emotion != EmotionHappy || cmd.Activity != ActivityLaughing {
		t.Fatalf("unexpected command: %+v", cmd)
	}
}

func TestDecodeExpressionFallsBackToNeutral(t *testing.T) {
	cmd, err := DecodeExpression([]byte(`{"emotion":"delighted","activity":""}`))
	if err == nil {
		t.Fatal("expected validation warning")
	}
	if !strings.Contains(err.Error(), "unsupported emotion") || !strings.Contains(err.Error(), "unsupported activity") {
		t.Fatalf("incomplete warning: %v", err)
	}
	if cmd != (ExpressionCommand{Emotion: EmotionNeutral, Activity: ActivityNeutral}) {
		t.Fatalf("unexpected fallback: %+v", cmd)
	}
}

func TestDecodeExpressionRejectsInvalidJSON(t *testing.T) {
	if _, err := DecodeExpression([]byte(`{`)); err == nil {
		t.Fatal("expected JSON error")
	}
}

func TestExpressionInboxLatestWins(t *testing.T) {
	inbox := NewExpressionInbox()
	inbox.Submit(ExpressionCommand{Emotion: EmotionHappy, Activity: ActivityTalking})
	inbox.Submit(ExpressionCommand{Emotion: EmotionSad, Activity: ActivityCrying})

	got, ok := inbox.Latest()
	if !ok {
		t.Fatal("expected a command")
	}
	want := ExpressionCommand{Emotion: EmotionSad, Activity: ActivityCrying}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if _, ok := inbox.Latest(); ok {
		t.Fatal("expected inbox to be empty")
	}
}

func TestAllVocabularyValuesAreValid(t *testing.T) {
	for _, emotion := range Emotions {
		if !validEmotion(emotion) {
			t.Errorf("emotion %q is not valid", emotion)
		}
	}
	for _, activity := range Activities {
		if !validActivity(activity) {
			t.Errorf("activity %q is not valid", activity)
		}
	}
}
