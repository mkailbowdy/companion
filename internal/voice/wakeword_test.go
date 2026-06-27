package voice

import "testing"

func TestStripWakeWordVariants(t *testing.T) {
	tests := []struct {
		name       string
		transcript string
		want       string
	}{
		{name: "letters", transcript: "BMO, what time is it?", want: "what time is it?"},
		{name: "dotted letters", transcript: "B.M.O. what time is it?", want: "what time is it?"},
		{name: "beemo", transcript: "Beemo tell me a joke", want: "tell me a joke"},
		{name: "bee mo", transcript: "Bee Mo - dance", want: "dance"},
		{name: "leading attention", transcript: "hey BMO, come here", want: "come here"},
		{name: "okay attention", transcript: "Okay, beemo: dance", want: "dance"},
		{name: "wake only", transcript: "BMO", want: ""},
	}
	for _, test := range tests {
		got, ok := stripWakeWord(test.transcript)
		if !ok {
			t.Errorf("%s: stripWakeWord did not detect wake word", test.name)
			continue
		}
		if got != test.want {
			t.Errorf("%s: command = %q, want %q", test.name, got, test.want)
		}
	}
}

func TestStripWakeWordRejectsUnrelatedTranscripts(t *testing.T) {
	for _, transcript := range []string{
		"what time is it?",
		"the BMO toy is here",
		"beemore is not a wake word",
		"okay then",
	} {
		if command, ok := stripWakeWord(transcript); ok {
			t.Errorf("stripWakeWord(%q) = %q, true; want false", transcript, command)
		}
	}
}
