# BMO voice face

An animated BMO face and single-turn local voice loop for a Raspberry Pi:

```text
ALSA microphone -> RMS VAD -> whisper.cpp -> OpenClaw -> ElevenLabs -> ALSA speaker
```

Ebitengine stays on the main thread. Audio capture, transcription, network
requests, speech generation, and playback run in one cancellable conversation
goroutine.

## Runtime requirements

- 64-bit Linux, intended for Raspberry Pi 5
- `arecord` and `aplay` from ALSA utilities
- `whisper-cli` from whisper.cpp
- English `ggml-base.en-q5_0.bin` Whisper model
- `openclaw` with the Responses endpoint enabled
- OpenClaw TTS configured with ElevenLabs
- `ffmpeg`

OpenClaw must enable `gateway.http.endpoints.responses.enabled`. Configure its
TTS provider and voice in OpenClaw; this application does not contain
ElevenLabs credentials or a voice ID.

## Configuration

The only required secret is:

```sh
export OPENCLAW_GATEWAY_TOKEN="..."
```

Defaults can be overridden with:

| Variable | Default |
| --- | --- |
| `BMO_ARECORD_COMMAND` | `arecord` |
| `BMO_WHISPER_COMMAND` | `whisper-cli` |
| `BMO_OPENCLAW_COMMAND` | `openclaw` |
| `BMO_FFMPEG_COMMAND` | `ffmpeg` |
| `BMO_APLAY_COMMAND` | `aplay` |
| `BMO_WHISPER_MODEL` | `models/ggml-base.en-q5_0.bin` |
| `BMO_CAPTURE_DEVICE` | `default` |
| `BMO_PLAYBACK_DEVICE` | `default` |
| `BMO_OPENCLAW_URL` | `http://127.0.0.1:18789/v1/responses` |
| `BMO_OPENCLAW_MODEL` | `openclaw/default` |
| `BMO_OPENCLAW_USER` | `bmo-rpi` |
| `BMO_VAD_MIN_RMS` | `60` |
| `BMO_VAD_NOISE_MULTIPLIER` | `2` |
| `BMO_STARTUP_TIMEOUT` | `15s` |
| `BMO_WHISPER_TIMEOUT` | `90s` |
| `BMO_RESPONSE_TIMEOUT` | `90s` |
| `BMO_TTS_TIMEOUT` | `90s` |
| `BMO_PLAYBACK_TIMEOUT` | `5m` |
| `BMO_PLAYBACK_COOLDOWN` | `1s` |
| `BMO_FAILURE_STATE_DELAY` | `1s` |

At startup BMO verifies command availability, the Whisper model, gateway
authentication, and that OpenClaw reports ElevenLabs as its TTS provider.

## Run

```sh
go run ./cmd/bmo
```

For desktop development:

```sh
BMO_WINDOWED=1 go run ./cmd/bmo
```

To require the wake word before BMO responds:

```sh
go run ./cmd/bmo --wake-word
```

Press Left/Right to cycle emotions, Up/Down to cycle activities, and Space to
return to neutral.

## Voice behavior

Audio is captured as 16 kHz mono signed 16-bit PCM. The adaptive RMS detector
uses 400 ms of pre-roll, requires 100 ms of speech to activate, ends after
800 ms of silence, and caps an utterance at 20 seconds. Microphone capture is
stopped during transcription, response generation, TTS, playback, and the
one-second playback cooldown.

For unusually quiet microphones, lower `BMO_VAD_MIN_RMS`. For a microphone
peaking near `-40 dBFS`, start with `40`. Lower
`BMO_VAD_NOISE_MULTIPLIER` toward `1.5` if speech remains too close to the
ambient noise floor; increase either value if ambient noise triggers BMO.

With `--wake-word`, BMO ignores transcripts until it hears "BMO" at the start
of an utterance. Whisper spellings such as "B.M.O.", "Beemo", and "Bee Mo" are
accepted. "BMO, what time is it?" is handled in one turn; saying only "BMO"
arms the next utterance as the command.

OpenClaw is required to call the pinned `deliver_response` function with a
spoken `message`, an enum-validated `emotion`, and an enum-validated
`activity`. Speaking is local renderer state, so mouth animation composes with
semantic states such as laughing and crying.

Raw audio files are temporary and removed after every turn. Conversation text
is not logged by default. SIGINT and SIGTERM cancel active child processes.

## Test

```sh
go test ./internal/...
go build ./...
```

Ebitengine tests require an X server; use `xvfb-run` on headless Linux.
