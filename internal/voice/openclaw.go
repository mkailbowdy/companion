package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"bmo.pushiro.com/internal/expression"
)

type Responder interface {
	Respond(context.Context, string) (expression.ReplyEnvelope, error)
}

type OpenClawClient struct {
	URL     string
	Model   string
	User    string
	Token   string
	Timeout time.Duration
	Client  *http.Client
}

type functionTool struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func (c *OpenClawClient) Respond(ctx context.Context, transcript string) (expression.ReplyEnvelope, error) {
	requestBody := map[string]any{
		"model":  c.Model,
		"user":   c.User,
		"input":  transcript,
		"stream": false,
		"instructions": "Respond as BMO. You must call deliver_response exactly once. " +
			"Put only the words that should be spoken in message and choose the face emotion and semantic activity.",
		"tools": []functionTool{deliverResponseTool()},
		"tool_choice": map[string]string{
			"type": "function",
			"name": "deliver_response",
		},
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return expression.ReplyEnvelope{}, fmt.Errorf("encode OpenClaw request: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, c.URL, bytes.NewReader(body))
	if err != nil {
		return expression.ReplyEnvelope{}, fmt.Errorf("create OpenClaw request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient().Do(request)
	if err != nil {
		return expression.ReplyEnvelope{}, fmt.Errorf("OpenClaw request: %w", err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return expression.ReplyEnvelope{}, fmt.Errorf("read OpenClaw response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return expression.ReplyEnvelope{}, decodeOpenClawError(response.StatusCode, data)
	}
	return decodeFunctionCall(data)
}

func (c *OpenClawClient) Validate(ctx context.Context) error {
	modelsURL, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("invalid OpenClaw URL: %w", err)
	}
	modelsURL.Path = "/v1/models"
	modelsURL.RawQuery = ""
	modelsURL.Fragment = ""
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL.String(), nil)
	if err != nil {
		return fmt.Errorf("create OpenClaw validation request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.Token)
	response, err := c.httpClient().Do(request)
	if err != nil {
		return fmt.Errorf("connect to OpenClaw: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		return decodeOpenClawError(response.StatusCode, data)
	}
	return nil
}

func (c *OpenClawClient) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{}
}

func deliverResponseTool() functionTool {
	emotions := make([]string, len(expression.Emotions))
	for i, emotion := range expression.Emotions {
		emotions[i] = string(emotion)
	}
	activities := make([]string, len(expression.Activities))
	for i, activity := range expression.Activities {
		activities[i] = string(activity)
	}
	return functionTool{
		Type:        "function",
		Name:        "deliver_response",
		Description: "Deliver BMO's spoken reply and matching face state.",
		Parameters: map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"message", "emotion", "activity"},
			"properties": map[string]any{
				"message":  map[string]any{"type": "string", "minLength": 1},
				"emotion":  map[string]any{"type": "string", "enum": emotions},
				"activity": map[string]any{"type": "string", "enum": activities},
			},
		},
	}
}

func decodeFunctionCall(data []byte) (expression.ReplyEnvelope, error) {
	var response struct {
		Output []struct {
			Type      string          `json:"type"`
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"output"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return expression.ReplyEnvelope{}, fmt.Errorf("decode OpenClaw response: %w", err)
	}
	for _, item := range response.Output {
		if item.Type != "function_call" || item.Name != "deliver_response" {
			continue
		}
		arguments := item.Arguments
		if len(arguments) == 0 {
			return expression.ReplyEnvelope{}, fmt.Errorf("deliver_response has no arguments")
		}
		if arguments[0] == '"' {
			var encoded string
			if err := json.Unmarshal(arguments, &encoded); err != nil {
				return expression.ReplyEnvelope{}, fmt.Errorf("decode deliver_response arguments: %w", err)
			}
			arguments = []byte(encoded)
		}
		reply, err := expression.DecodeReplyEnvelope(arguments)
		if err != nil {
			return expression.ReplyEnvelope{}, fmt.Errorf("validate deliver_response: %w", err)
		}
		return reply, nil
	}
	return expression.ReplyEnvelope{}, fmt.Errorf("OpenClaw response did not call deliver_response")
}

func decodeOpenClawError(status int, data []byte) error {
	var response struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &response) == nil && response.Error.Message != "" {
		message := strings.TrimSpace(response.Error.Message)
		if len(message) > 300 {
			message = message[:300]
		}
		if response.Error.Type != "" {
			return fmt.Errorf("OpenClaw returned HTTP %d (%s): %s", status, response.Error.Type, message)
		}
		return fmt.Errorf("OpenClaw returned HTTP %d: %s", status, message)
	}
	return fmt.Errorf("OpenClaw returned HTTP %d", status)
}
