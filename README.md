# BMO face

An animated, procedural BMO face for a 1280×720 Raspberry Pi display, built
with Ebitengine.

The code is arranged in a `Let's Go`-style tree:

- `cmd/bmo` contains the executable entrypoint.
- `internal/expression` handles JSON input and the latest-wins inbox.
- `internal/game` owns the Ebitengine scene and animation logic.

## Run

The normal build starts fullscreen and hides the pointer:

```sh
go run ./cmd/bmo
```

For desktop development:

```sh
BMO_WINDOWED=1 go run ./cmd/bmo
```

Use Left/Right to cycle emotions, Up/Down to cycle activities, and Space to
return to neutral.

## Sending expression updates

Create the game with an `ExpressionInbox`. A receiver goroutine can decode an
LLM response and submit it without touching Ebitengine state:

```go
command, warning := DecodeExpression(responseJSON)
if warning != nil {
    log.Printf("expression warning: %v", warning)
}
inbox.Submit(command)
```

The accepted JSON shape is:

```json
{"emotion":"happy","activity":"laughing"}
```

Supported emotions are `neutral`, `happy`, `sad`, `angry`, `surprised`,
`scared`, `confused`, `sleepy`, and `excited`. Supported activities are
`neutral`, `blinking`, `talking`, `laughing`, `crying`, `thinking`, and
`listening`. Unknown or missing values become `neutral` and return a warning.

The inbox has capacity one. If the producer runs faster than the display,
unread commands are replaced so the face always converges on the latest state.
