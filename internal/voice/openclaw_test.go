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

func TestOpenClawClientFallsBackToStrictJSONWhenToolCallIsMissing(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
		}
		switch requests {
		case 1:
			if _, ok := request["tool_choice"]; !ok {
				t.Error("primary request did not pin deliver_response")
			}
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{
				"error":{
					"type":"api_error",
					"message":"tool_choice required a deliver_response tool call, but the agent did not produce one"
				}
			}`))
		case 2:
			if _, ok := request["tools"]; ok {
				t.Error("fallback request unexpectedly included tools")
			}
			if _, ok := request["tool_choice"]; ok {
				t.Error("fallback request unexpectedly included tool_choice")
			}
			instructions, _ := request["instructions"].(string)
			if !strings.Contains(instructions, "exactly one JSON object") {
				t.Errorf("fallback instructions = %q", instructions)
			}
			_, _ = w.Write([]byte(`{
				"output":[{
					"type":"message",
					"content":[{
						"type":"output_text",
						"text":"{\"message\":\"Hi there!\",\"emotion\":\"happy\",\"activity\":\"neutral\"}"
					}]
				}]
			}`))
		default:
			t.Errorf("unexpected request %d", requests)
		}
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
		Message: "Hi there!", Emotion: expression.EmotionHappy, Activity: expression.ActivityNeutral,
	}
	if reply != want {
		t.Fatalf("got %+v, want %+v", reply, want)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}

func TestOpenClawClientFallsBackToPlainTextWhenAgentIgnoresJSON(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{
				"error":{
					"type":"api_error",
					"message":"tool_choice required a deliver_response tool call, but the agent did not produce one"
				}
			}`))
			return
		}
		_, _ = w.Write([]byte(`{
			"output":[{
				"type":"message",
				"content":[{
					"type":"output_text",
					"text":"I am ready to play! What should we do?"
				}]
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
		Message:  "I am ready to play! What should we do?",
		Emotion:  expression.EmotionNeutral,
		Activity: expression.ActivityNeutral,
	}
	if reply != want {
		t.Fatalf("got %+v, want %+v", reply, want)
	}
}

func TestOpenClawClientDoesNotFallbackForOtherHTTPFailures(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":{"type":"api_error","message":"upstream unavailable"}}`))
	}))
	defer server.Close()

	client := &OpenClawClient{URL: server.URL, Token: "secret", Timeout: time.Second}
	_, err := client.Respond(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "upstream unavailable") {
		t.Fatalf("unexpected error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestDecodeOutputTextAcceptsFencedJSONButStillValidatesSchema(t *testing.T) {
	response, err := json.Marshal(map[string]string{
		"output_text": "```json\n" +
			`{"message":"Hello","emotion":"happy","activity":"talking"}` +
			"\n```",
	})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := decodeOutputText(response)
	if err != nil {
		t.Fatalf("decodeOutputText: %v", err)
	}
	if reply.Message != "Hello" || reply.Emotion != expression.EmotionHappy {
		t.Fatalf("unexpected reply: %+v", reply)
	}

	_, err = decodeOutputText([]byte(`{
		"output_text":"{\"message\":\"Hello\",\"emotion\":\"invalid\",\"activity\":\"talking\"}"
	}`))
	if err == nil {
		t.Fatal("expected invalid fallback enum to fail")
	}
}

func TestDecodeOutputTextSanitizesAndLimitsPlainText(t *testing.T) {
	response, err := json.Marshal(map[string]string{
		"output_text": "  Hello,\n\nfriend!\t" + strings.Repeat("x", maxPlainTextRunes),
	})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := decodeOutputText(response)
	if err != nil {
		t.Fatalf("decodeOutputText: %v", err)
	}
	if !strings.HasPrefix(reply.Message, "Hello, friend!") {
		t.Fatalf("message was not normalized: %q", reply.Message)
	}
	if !strings.HasSuffix(reply.Message, "…") {
		t.Fatalf("message was not truncated: %q", reply.Message)
	}
	if len([]rune(reply.Message)) > maxPlainTextRunes+1 {
		t.Fatalf("message has %d runes", len([]rune(reply.Message)))
	}
	if reply.Emotion != expression.EmotionNeutral || reply.Activity != expression.ActivityNeutral {
		t.Fatalf("plain-text state = %+v", reply)
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
