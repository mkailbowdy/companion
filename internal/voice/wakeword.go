package voice

import (
	"regexp"
	"strings"
)

var wakeWordPrefix = regexp.MustCompile(`(?i)^[[:space:]]*(?:(?:hey|hi|ok|okay)\b[[:space:],.!?:;-]+)?(?:b[[:space:].]*m[[:space:].]*o|bee[[:space:]]*mo)\b\.?(?:[[:space:],.!?:;-]+|$)`)

func stripWakeWord(transcript string) (string, bool) {
	match := wakeWordPrefix.FindStringIndex(transcript)
	if match == nil {
		return "", false
	}
	command := strings.TrimSpace(transcript[match[1]:])
	command = strings.TrimLeft(command, " \t\r\n,.:;!?-")
	return strings.TrimSpace(command), true
}
