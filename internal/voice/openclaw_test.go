package voice

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bmo.pushiro.com/internal/expression"
)

func TestOpenClawClientRequestAndFunctionCall(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"output":[{
				"type":"function_call",
				"name":"deliver_response",
				"arguments":"{\"message\":\"Mathematical!\",\"emotion\":\"happy\",\"activity\":\"laughing\"}"
			}]
		}`))
	}))
	defer server.Close()

	client := &OpenClawClient{
		URL: server.URL, Model: "openclaw/default", User: "bmo-rpi",
		Token: "secret", Timeout: time.Second,
	}
	reply, err := client.Respond(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Respond: %v", err)
	}
	want := expression.ReplyEnvelope{
		Message: "Mathematical!", Emotion: expression.EmotionHappy, Activity: expression.ActivityLaughing,
	}
	if reply != want {
		t.Fatalf("got %+v, want %+v", reply, want)
	}
	if requestBody["model"] != "openclaw/default" || requestBody["user"] != "bmo-rpi" || requestBody["input"] != "hello" {
		t.Fatalf("unexpected request identity: %#v", requestBody)
	}
	if requestBody["stream"] != false {
		t.Fatalf("stream = %#v", requestBody["stream"])
	}
	toolChoice := requestBody["tool_choice"].(map[string]any)
	if toolChoice["name"] != "deliver_response" {
		t.Fatalf("tool choice = %#v", toolChoice)
	}
	tools := requestBody["tools"].([]any)
	tool := tools[0].(map[string]any)
	parameters := tool["parameters"].(map[string]any)
	if parameters["additionalProperties"] != false {
		t.Fatalf("tool schema is not closed: %#v", parameters)
	}
}

func TestOpenClawClientRejectsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"type":"authentication_error","message":"bad token"}}`))
	}))
	defer server.Close()

	client := &OpenClawClient{URL: server.URL, Token: "bad", Timeout: time.Second}
	_, err := client.Respond(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") || !strings.Contains(err.Error(), "authentication_error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeFunctionCallRejectsMalformedReplies(t *testing.T) {
	tests := []string{
		`{"output":[]}`,
		`{"output":[{"type":"function_call","name":"other","arguments":"{}"}]}`,
		`{"output":[{"type":"function_call","name":"deliver_response","arguments":"{"}]}`,
		`{"output":[{"type":"function_call","name":"deliver_response","arguments":{"message":"","emotion":"unknown","activity":"neutral"}}]}`,
	}
	for _, input := range tests {
		if _, err := decodeFunctionCall([]byte(input)); err == nil {
			t.Errorf("decodeFunctionCall(%s) succeeded", input)
		}
	}
}

func TestOpenClawValidateUsesModelsEndpointAndAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("missing auth")
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()
	client := &OpenClawClient{URL: server.URL + "/v1/responses", Token: "secret", Timeout: time.Second}
	if err := client.Validate(context.Background()); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
