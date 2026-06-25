package expression

import (
	"strings"
	"testing"
)

func TestDecodeReplyEnvelope(t *testing.T) {
	reply, err := DecodeReplyEnvelope([]byte(`{"message":" Hello! ","emotion":" HAPPY ","activity":"Laughing"}`))
	if err != nil {
		t.Fatalf("DecodeReplyEnvelope: %v", err)
	}
	want := ReplyEnvelope{Message: "Hello!", Emotion: EmotionHappy, Activity: ActivityLaughing}
	if reply != want {
		t.Fatalf("got %+v, want %+v", reply, want)
	}
}

func TestDecodeReplyEnvelopeRejectsInvalidFields(t *testing.T) {
	_, err := DecodeReplyEnvelope([]byte(`{"message":" ","emotion":"delighted","activity":""}`))
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, part := range []string{"message is required", "unsupported emotion", "unsupported activity"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q does not contain %q", err, part)
		}
	}
}

func TestDecodeReplyEnvelopeRejectsInvalidJSON(t *testing.T) {
	if _, err := DecodeReplyEnvelope([]byte(`{`)); err == nil {
		t.Fatal("expected JSON error")
	}
	if _, err := DecodeReplyEnvelope([]byte(`{"message":"hi","emotion":"happy","activity":"neutral","extra":true}`)); err == nil {
		t.Fatal("expected unknown field error")
	}
}

func TestFaceStateInboxLatestWins(t *testing.T) {
	inbox := NewFaceStateInbox()
	inbox.Submit(FaceState{Emotion: EmotionHappy, Activity: ActivityNeutral, Speaking: true})
	inbox.Submit(FaceState{Emotion: EmotionSad, Activity: ActivityCrying})

	got, ok := inbox.Latest()
	if !ok {
		t.Fatal("expected a state")
	}
	want := FaceState{Emotion: EmotionSad, Activity: ActivityCrying}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if _, ok := inbox.Latest(); ok {
		t.Fatal("expected inbox to be empty")
	}
}

func TestAllVocabularyValuesAreValid(t *testing.T) {
	for _, emotion := range Emotions {
		if !ValidEmotion(emotion) {
			t.Errorf("emotion %q is not valid", emotion)
		}
	}
	for _, activity := range Activities {
		if !ValidActivity(activity) {
			t.Errorf("activity %q is not valid", activity)
		}
	}
}
