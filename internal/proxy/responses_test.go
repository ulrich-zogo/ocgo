package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ocgo/internal/compat"
	"ocgo/internal/config"
)

func TestWriteOpenAIErrorShape(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteOpenAIError(rr, http.StatusBadRequest, "bad model", "invalid_request_error", "model", "missing")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env.Error.Message != "bad model" {
		t.Errorf("message = %q, want bad model", env.Error.Message)
	}
	if env.Error.Type != "invalid_request_error" {
		t.Errorf("type = %q, want invalid_request_error", env.Error.Type)
	}
	if env.Error.Param != "model" {
		t.Errorf("param = %q, want model", env.Error.Param)
	}
	if env.Error.Code != "missing" {
		t.Errorf("code = %q, want missing", env.Error.Code)
	}
}

func TestWriteOpenAIErrorDefaultType(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteOpenAIError(rr, http.StatusBadRequest, "oops", "", "", "")
	var env OpenAIErrorEnvelope
	json.Unmarshal(rr.Body.Bytes(), &env)
	if env.Error.Type != "invalid_request_error" {
		t.Errorf("type = %q, want default invalid_request_error", env.Error.Type)
	}
}

func TestWriteUpstreamOpenAIErrorWithEnvelope(t *testing.T) {
	rr := httptest.NewRecorder()
	body := []byte(`{"error":{"message":"invalid model","type":"invalid_request_error","param":"model","code":"model_invalid"}}`)
	WriteUpstreamOpenAIError(rr, http.StatusBadRequest, body)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rr.Code)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env.Error.Message != "invalid model" {
		t.Errorf("message = %q, want 'invalid model'", env.Error.Message)
	}
	if env.Error.Type != "invalid_request_error" {
		t.Errorf("type = %q, want 'invalid_request_error'", env.Error.Type)
	}
	if env.Error.Param != "model" {
		t.Errorf("param = %q, want 'model'", env.Error.Param)
	}
	if env.Error.Code != "model_invalid" {
		t.Errorf("code = %q, want 'model_invalid'", env.Error.Code)
	}
}

func TestWriteUpstreamOpenAIErrorWithPlainBody(t *testing.T) {
	rr := httptest.NewRecorder()
	body := []byte(`service unavailable`)
	WriteUpstreamOpenAIError(rr, http.StatusBadGateway, body)
	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rr.Code)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env.Error.Type != "upstream_error" {
		t.Errorf("type = %q, want 'upstream_error'", env.Error.Type)
	}
	if env.Error.Code != "upstream_failure" {
		t.Errorf("code = %q, want 'upstream_failure'", env.Error.Code)
	}
}

func TestWriteUpstreamOpenAIErrorEmptyBody(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteUpstreamOpenAIError(rr, http.StatusInternalServerError, nil)
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), `"type":"upstream_error"`) {
		t.Errorf("body should contain upstream_error type; got %s", rr.Body.String())
	}
}

func TestValidateResponsesRequestMissingModel(t *testing.T) {
	rr := compat.ResponsesRequest{Model: "  "}
	err := ValidateResponsesRequest(rr)
	if err == nil {
		t.Fatal("expected error for empty model")
	}
	var ve *ResponsesValidationError
	if !errorsAs(err, &ve) {
		t.Fatalf("expected ResponsesValidationError, got %T", err)
	}
	if ve.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", ve.Status)
	}
	if ve.Param != "model" {
		t.Errorf("param = %q, want model", ve.Param)
	}
}

func TestValidateResponsesRequestMissingInputAndInstructions(t *testing.T) {
	rr := compat.ResponsesRequest{Model: "minimax-m3"}
	err := ValidateResponsesRequest(rr)
	if err == nil {
		t.Fatal("expected error when both input and instructions are missing")
	}
	var ve *ResponsesValidationError
	if !errorsAs(err, &ve) {
		t.Fatalf("expected ResponsesValidationError, got %T", err)
	}
	if ve.Param != "input" {
		t.Errorf("param = %q, want input", ve.Param)
	}
}

func TestValidateResponsesRequestWithInstructionsOnly(t *testing.T) {
	rr := compat.ResponsesRequest{Model: "minimax-m3", Instructions: "You are strict."}
	if err := ValidateResponsesRequest(rr); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateResponsesRequestWithInputOnly(t *testing.T) {
	rr := compat.ResponsesRequest{Model: "minimax-m3", Input: json.RawMessage(`"hello"`)}
	if err := ValidateResponsesRequest(rr); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateResponsesRequestWithEmptyInputString(t *testing.T) {
	rr := compat.ResponsesRequest{Model: "minimax-m3", Input: json.RawMessage(`"   "`)}
	err := ValidateResponsesRequest(rr)
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
}

func TestValidateResponsesRequestWithEmptyInputArray(t *testing.T) {
	rr := compat.ResponsesRequest{Model: "minimax-m3", Input: json.RawMessage(`[]`)}
	err := ValidateResponsesRequest(rr)
	if err == nil {
		t.Fatal("expected error for empty input array")
	}
}

func TestValidateResponsesRequestNegativeMaxTokens(t *testing.T) {
	rr := compat.ResponsesRequest{Model: "minimax-m3", Input: json.RawMessage(`"hi"`), MaxTokens: -1}
	err := ValidateResponsesRequest(rr)
	if err == nil {
		t.Fatal("expected error for negative max_output_tokens")
	}
	var ve *ResponsesValidationError
	if !errorsAs(err, &ve) {
		t.Fatalf("expected ResponsesValidationError, got %T", err)
	}
	if ve.Param != "max_output_tokens" {
		t.Errorf("param = %q, want max_output_tokens", ve.Param)
	}
}

func TestResponsesToChatSafeInputString(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model: "minimax-m3",
		Input: json.RawMessage(`"Bonjour"`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(or.Messages))
	}
	if or.Messages[0].Role != "user" {
		t.Errorf("role = %q, want user", or.Messages[0].Role)
	}
	if or.Messages[0].Content != "Bonjour" {
		t.Errorf("content = %v, want Bonjour", or.Messages[0].Content)
	}
}

func TestResponsesToChatSafeInputArraySingleUser(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model: "minimax-m3",
		Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"Analyse ceci."}]}]`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(or.Messages))
	}
	if or.Messages[0].Role != "user" {
		t.Errorf("role = %q, want user", or.Messages[0].Role)
	}
	if or.Messages[0].Content != "Analyse ceci." {
		t.Errorf("content = %v, want 'Analyse ceci.'", or.Messages[0].Content)
	}
}

func TestResponsesToChatSafeMultiTurn(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model: "minimax-m3",
		Input: json.RawMessage(`[
			{"role":"user","content":"Bonjour"},
			{"role":"assistant","content":"Bonjour, comment aider ?"},
			{"role":"user","content":"Résume le contexte."}
		]`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(or.Messages))
	}
	wantRoles := []string{"user", "assistant", "user"}
	wantContent := []string{"Bonjour", "Bonjour, comment aider ?", "Résume le contexte."}
	for i, m := range or.Messages {
		if m.Role != wantRoles[i] {
			t.Errorf("messages[%d].Role = %q, want %q", i, m.Role, wantRoles[i])
		}
		if m.Content != wantContent[i] {
			t.Errorf("messages[%d].Content = %v, want %q", i, m.Content, wantContent[i])
		}
	}
}

func TestResponsesToChatSafeInstructions(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model:        "minimax-m3",
		Instructions: "Tu es un assistant strict.",
		Input:        json.RawMessage(`"Réponds en JSON."`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(or.Messages))
	}
	if or.Messages[0].Role != "system" {
		t.Errorf("first role = %q, want system", or.Messages[0].Role)
	}
	if or.Messages[0].Content != "Tu es un assistant strict." {
		t.Errorf("system content = %v, want strict instruction", or.Messages[0].Content)
	}
	if or.Messages[1].Role != "user" {
		t.Errorf("second role = %q, want user", or.Messages[1].Role)
	}
}

func TestResponsesToChatSafeInstructionsAndExistingSystem(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model:        "minimax-m3",
		Instructions: "First system",
		Input: json.RawMessage(`[
			{"role":"system","content":"Second system"},
			{"role":"user","content":"Hello"}
		]`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(or.Messages))
	}
	if or.Messages[0].Role != "system" || or.Messages[0].Content != "First system" {
		t.Errorf("first message = %+v, want system 'First system'", or.Messages[0])
	}
	if or.Messages[1].Role != "system" || or.Messages[1].Content != "Second system" {
		t.Errorf("second message = %+v, want system 'Second system'", or.Messages[1])
	}
}

func TestResponsesToChatSafeTextAndImage(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model: "minimax-m3",
		Input: json.RawMessage(`[{
			"role":"user",
			"content":[
				{"type":"input_text","text":"Décris l'image"},
				{"type":"input_image","image_url":"data:image/png;base64,AAA"}
			]
		}]`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(or.Messages))
	}
	msg := or.Messages[0]
	if msg.Role != "user" {
		t.Errorf("role = %q, want user", msg.Role)
	}
	parts, ok := msg.Content.([]compat.OAIContentPart)
	if !ok {
		t.Fatalf("content = %T, want []OAIContentPart", msg.Content)
	}
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	if parts[0].Type != "text" || parts[0].Text != "Décris l'image" {
		t.Errorf("part[0] = %+v", parts[0])
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,AAA" {
		t.Errorf("part[1] = %+v", parts[1])
	}
}

func TestResponsesToChatSafeFunctionTools(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model: "minimax-m3",
		Input: json.RawMessage(`"Hello"`),
		Tools: []compat.ResponseTool{
			{Type: "function", Name: "lookup", Description: "Lookup something", Parameters: json.RawMessage(`{"type":"object"}`)},
			{Type: "web_search", SearchContextSize: "high"},
			{Type: "file_search"},
			{Type: "computer_use"},
		},
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Tools) != 1 {
		t.Fatalf("tools = %d, want 1 (only function tools)", len(or.Tools))
	}
	if or.Tools[0].Type != "function" {
		t.Errorf("tool[0].Type = %q, want function", or.Tools[0].Type)
	}
	if or.Tools[0].Function.Name != "lookup" {
		t.Errorf("tool[0].Function.Name = %q, want lookup", or.Tools[0].Function.Name)
	}
	if or.Tools[0].Function.Description != "Lookup something" {
		t.Errorf("tool[0].Function.Description = %q", or.Tools[0].Function.Description)
	}
}

func TestResponsesToChatSafeFunctionCallAndOutput(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model: "minimax-m3",
		Input: json.RawMessage(`[
			{"type":"function_call","call_id":"call_123","name":"lookup","arguments":"{\"id\":\"1\"}"},
			{"type":"function_call_output","call_id":"call_123","output":"result-data"}
		]`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(or.Messages))
	}
	if or.Messages[0].Role != "assistant" {
		t.Errorf("messages[0].Role = %q, want assistant", or.Messages[0].Role)
	}
	if len(or.Messages[0].ToolCalls) != 1 {
		t.Fatalf("messages[0].ToolCalls = %d, want 1", len(or.Messages[0].ToolCalls))
	}
	tc := or.Messages[0].ToolCalls[0]
	if tc.ID != "call_123" || tc.Function.Name != "lookup" {
		t.Errorf("toolCall = %+v", tc)
	}
	if or.Messages[1].Role != "tool" {
		t.Errorf("messages[1].Role = %q, want tool", or.Messages[1].Role)
	}
	if or.Messages[1].ToolCallID != "call_123" {
		t.Errorf("messages[1].ToolCallID = %q, want call_123", or.Messages[1].ToolCallID)
	}
}

func TestResponsesToChatSafeUnknownFieldsTolerated(t *testing.T) {
	raw := `{
		"model":"kimi-k2.6",
		"input":"Hello",
		"previous_response_id":"resp_xyz",
		"store":true,
		"metadata":{"key":"value"},
		"parallel_tool_calls":true,
		"truncation":"auto",
		"include":["reasoning.encrypted_content"],
		"response_format":{"type":"json_object"},
		"service_tier":"auto",
		"user":"user-123",
		"reasoning":{"effort":"high"},
		"text":{"format":{"type":"text"}}
	}`
	rr := compat.ResponsesRequest{Model: "minimax-m3"}
	if err := json.Unmarshal([]byte(raw), &rr); err != nil {
		t.Fatalf("unmarshal failed for unknown fields: %v", err)
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 1 {
		t.Errorf("messages = %d, want 1", len(or.Messages))
	}
	if or.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want high", or.ReasoningEffort)
	}
}

func TestBuildResponsesResponseObjectShape(t *testing.T) {
	resp := BuildResponsesResponseObject("minimax-m3", "Hello", nil, compat.TokenUsage{
		InputTokens:  5,
		OutputTokens: 3,
		TotalTokens:  8,
		Present:      true,
	})
	if resp["object"] != "response" {
		t.Errorf("object = %v, want response", resp["object"])
	}
	if resp["model"] != "minimax-m3" {
		t.Errorf("model = %v, want minimax-m3", resp["model"])
	}
	if resp["status"] != "completed" {
		t.Errorf("status = %v, want completed", resp["status"])
	}
	if _, ok := resp["id"].(string); !ok {
		t.Errorf("id = %v, want string", resp["id"])
	}
	if _, ok := resp["created_at"].(int64); !ok {
		t.Errorf("created_at = %v, want int64", resp["created_at"])
	}
	output, ok := resp["output"].([]map[string]any)
	if !ok {
		t.Fatalf("output = %T, want []map[string]any", resp["output"])
	}
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	if output[0]["type"] != "message" {
		t.Errorf("output[0].type = %v, want message", output[0]["type"])
	}
	usage, ok := resp["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage = %T, want map", resp["usage"])
	}
	if usage["input_tokens"] != 5 || usage["output_tokens"] != 3 || usage["total_tokens"] != 8 {
		t.Errorf("usage = %v", usage)
	}
}

func TestBuildResponsesResponseObjectWithToolCalls(t *testing.T) {
	resp := BuildResponsesResponseObject("minimax-m3", "", []compat.OAIToolCall{
		{ID: "call_1", Type: "function", Function: compat.OAICallFunction{Name: "lookup", Arguments: `{"id":"1"}`}},
	}, compat.TokenUsage{})
	output := responseOutput(t, resp)
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	if output[0]["type"] != "function_call" {
		t.Errorf("output[0].type = %v, want function_call", output[0]["type"])
	}
	if output[0]["name"] != "lookup" {
		t.Errorf("output[0].name = %v, want lookup", output[0]["name"])
	}
}

func TestWriteResponsesResponseObjectProducesValidJSON(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteResponsesResponseObject(rr, "minimax-m3", "hi", nil, compat.TokenUsage{
		InputTokens:  1,
		OutputTokens: 2,
		TotalTokens:  3,
		Present:      true,
	})
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	var parsed map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response not valid JSON: %v", err)
	}
	if parsed["object"] != "response" {
		t.Errorf("object = %v, want response", parsed["object"])
	}
}

func TestConvertChatCompletionToResponsesPreservesToolCalls(t *testing.T) {
	upstream := `{
		"id":"chatcmpl-abc",
		"object":"chat.completion",
		"model":"kimi-k2.6",
		"choices":[{
			"index":0,
			"message":{
				"role":"assistant",
				"content":"",
				"tool_calls":[
					{"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"id\":\"1\"}"}}
				]
			},
			"finish_reason":"tool_calls"
		}],
		"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}
	}`
	resp, err := ConvertChatCompletionToResponses("minimax-m3", []byte(upstream))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if resp["id"] != "resp_chatcmpl-abc" {
		t.Errorf("id = %v, want resp_chatcmpl-abc", resp["id"])
	}
	output := responseOutput(t, resp)
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	if output[0]["type"] != "function_call" {
		t.Errorf("output[0].type = %v, want function_call", output[0]["type"])
	}
	if output[0]["name"] != "lookup" {
		t.Errorf("output[0].name = %v, want lookup", output[0]["name"])
	}
	usage := responseUsage(t, resp)
	if intFromUsageValue(t, usage["input_tokens"]) != 7 || intFromUsageValue(t, usage["output_tokens"]) != 2 || intFromUsageValue(t, usage["total_tokens"]) != 9 {
		t.Errorf("usage = %v", usage)
	}
}

func TestResponsesHandlerMethodNotAllowed(t *testing.T) {
	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodGet, "/v1/responses", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env.Error.Type != "invalid_request_error" {
		t.Errorf("type = %q, want invalid_request_error", env.Error.Type)
	}
}

func TestResponsesHandlerInvalidJSON(t *testing.T) {
	upstream := startFakeUpstream(t, `{"id":"x","object":"chat.completion","model":"kimi-k2.6","choices":[]}`)
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("{not json"))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env.Error.Type != "invalid_request_error" {
		t.Errorf("type = %q, want invalid_request_error", env.Error.Type)
	}
}

func TestResponsesHandlerMissingModel(t *testing.T) {
	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env.Error.Param != "model" {
		t.Errorf("param = %q, want model", env.Error.Param)
	}
}

func TestResponsesHandlerMissingInputAndInstructions(t *testing.T) {
	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if env.Error.Param != "input" {
		t.Errorf("param = %q, want input", env.Error.Param)
	}
}

func TestResponsesHandlerValidForwardsToUpstream(t *testing.T) {
	upstreamHit := atomic.Int32{}
	upstream := startFakeUpstream(t, `{
		"id":"chatcmpl-1",
		"object":"chat.completion",
		"model":"kimi-k2.6",
		"choices":[{
			"index":0,
			"message":{"role":"assistant","content":"Hello, world."},
			"finish_reason":"stop"
		}],
		"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}
	}`, func(r *http.Request) { upstreamHit.Add(1) })
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	body := `{"model":"kimi-k2.6","input":"Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if upstreamHit.Load() == 0 {
		t.Fatal("upstream was not called")
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if resp["object"] != "response" {
		t.Errorf("object = %v, want response", resp["object"])
	}
	output, ok := resp["output"].([]any)
	if !ok {
		t.Fatalf("output type = %T, want []any", resp["output"])
	}
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	msg, ok := output[0].(map[string]any)
	if !ok {
		t.Fatalf("output[0] type = %T, want map", output[0])
	}
	content, ok := msg["content"].([]any)
	if !ok {
		t.Fatalf("content type = %T, want []any", msg["content"])
	}
	part, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] type = %T", content[0])
	}
	if part["text"] != "Hello, world." {
		t.Errorf("text = %v, want 'Hello, world.'", part["text"])
	}
	usage := responseUsage(t, resp)
	if intFromUsageValue(t, usage["input_tokens"]) != 3 || intFromUsageValue(t, usage["output_tokens"]) != 2 || intFromUsageValue(t, usage["total_tokens"]) != 5 {
		t.Errorf("usage = %v", usage)
	}
}

func TestResponsesHandlerModelMapping(t *testing.T) {
	var seenModel string
	upstream := startFakeUpstream(t, `{
		"id":"x","object":"chat.completion","model":"kimi-k2.6",
		"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
	}`, func(r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &body)
		seenModel = body.Model
	})
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if seenModel != "kimi-k2.6" {
		t.Errorf("upstream model = %q, want kimi-k2.6", seenModel)
	}
}

func TestResponsesHandlerUnsupportedFieldsTolerated(t *testing.T) {
	upstream := startFakeUpstream(t, `{
		"id":"x","object":"chat.completion","model":"kimi-k2.6",
		"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}
	}`)
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	body := `{
		"model":"kimi-k2.6",
		"input":"hi",
		"previous_response_id":"resp_xyz",
		"store":true,
		"metadata":{"k":"v"},
		"parallel_tool_calls":true,
		"truncation":"auto",
		"include":["reasoning.encrypted_content"],
		"response_format":{"type":"json_object"},
		"service_tier":"auto",
		"user":"user-1"
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestResponsesHandlerImageRejectionForIncompatibleModel(t *testing.T) {
	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	body := `{
		"model":"some-non-image-model",
		"input":[{
			"role":"user",
			"content":[
				{"type":"input_text","text":"describe"},
				{"type":"input_image","image_url":"data:image/png;base64,AAAA"}
			]
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestResponsesHandlerImageAllowedForKimiModel(t *testing.T) {
	upstream := startFakeUpstream(t, `{
		"id":"x","object":"chat.completion","model":"kimi-k2.6",
		"choices":[{"index":0,"message":{"role":"assistant","content":"img"},"finish_reason":"stop"}],
		"usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0}
	}`)
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	body := `{
		"model":"kimi-k2.6",
		"input":[{
			"role":"user",
			"content":[
				{"type":"input_text","text":"describe"},
				{"type":"input_image","image_url":"data:image/png;base64,AAAA"}
			]
		}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
}

func TestResponsesHandlerAnthropicRoute(t *testing.T) {
	var anthropicHit atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		anthropicHit.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"id":"msg_1",
			"type":"message",
			"role":"assistant",
			"model":"minimax-m3",
			"content":[{"type":"text","text":"anthropic-route"}],
			"stop_reason":"end_turn",
			"usage":{"input_tokens":4,"output_tokens":2}
		}`))
	}))
	defer upstream.Close()
	old := AnthropicURL
	AnthropicURL = upstream.URL
	defer func() { AnthropicURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"minimax-m3","input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if anthropicHit.Load() == 0 {
		t.Fatal("anthropic upstream was not called")
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	output := responseOutput(t, resp)
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	content, ok := output[0]["content"].([]any)
	if !ok {
		t.Fatalf("content type = %T", output[0]["content"])
	}
	if len(content) == 0 {
		t.Fatalf("content empty")
	}
	part, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] type = %T", content[0])
	}
	if part["text"] != "anthropic-route" {
		t.Errorf("text = %v, want 'anthropic-route'", part["text"])
	}
}

func TestResponsesHandlerStreaming(t *testing.T) {
	streamBody := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello \"}}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"world.\"}}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{}}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\n" +
		"data: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(streamBody))
	}))
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","stream":true,"input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"event: response.created",
		"event: response.output_text.delta",
		"event: response.completed",
		"data: [DONE]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stream missing %q\nbody:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "Hello ") || !strings.Contains(body, "world.") {
		t.Errorf("stream missing text deltas; body:\n%s", body)
	}
}

func TestResponsesHandlerStreamingWithToolCalls(t *testing.T) {
	streamBody := "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":null,\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"id\\\":\\\"1\\\"}\"}}]}}]}\n\n" +
		"data: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(streamBody))
	}))
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","stream":true,"input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{
		"event: response.output_item.added",
		"event: response.function_call_arguments.delta",
		"event: response.output_item.done",
		"data: [DONE]",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("stream missing %q\nbody:\n%s", want, body)
		}
	}
}

func TestResponsesHandlerStreamingToolCallAccumulator(t *testing.T) {
	chunks := []string{
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":null,\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"lookup\",\"arguments\":\"\"}}]}}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"id\\\":\\\"\"}}]}}]}\n\n",
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"1\\\"}\"}}]}}]}\n\n",
		"data: [DONE]\n\n",
	}
	streamBody := strings.Join(chunks, "")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(streamBody))
	}))
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","stream":true,"input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()

	if c := strings.Count(body, "event: response.output_item.added"); c != 1 {
		t.Errorf("response.output_item.added count = %d, want 1; body:\n%s", c, body)
	}
	if c := strings.Count(body, "event: response.output_item.done"); c != 1 {
		t.Errorf("response.output_item.done count = %d, want 1; body:\n%s", c, body)
	}

	added := extractSSEData(t, body, "response.output_item.added")
	if added == "" {
		t.Fatalf("missing response.output_item.added data; body:\n%s", body)
	}
	var addedItem map[string]any
	if err := json.Unmarshal([]byte(added), &addedItem); err != nil {
		t.Fatalf("added item not JSON: %v", err)
	}
	if addedItem["id"] != "call_1" {
		t.Errorf("added item id = %v, want call_1", addedItem["id"])
	}
	if addedItem["call_id"] != "call_1" {
		t.Errorf("added item call_id = %v, want call_1", addedItem["call_id"])
	}
	if addedItem["name"] != "lookup" {
		t.Errorf("added item name = %v, want lookup", addedItem["name"])
	}

	done := extractSSEData(t, body, "response.output_item.done")
	if done == "" {
		t.Fatalf("missing response.output_item.done data; body:\n%s", body)
	}
	var doneItem map[string]any
	if err := json.Unmarshal([]byte(done), &doneItem); err != nil {
		t.Fatalf("done item not JSON: %v", err)
	}
	if doneItem["id"] != "call_1" {
		t.Errorf("done item id = %v, want call_1", doneItem["id"])
	}
	if doneItem["name"] != "lookup" {
		t.Errorf("done item name = %v, want lookup", doneItem["name"])
	}
	if got, _ := doneItem["arguments"].(string); got != `{"id":"1"}` {
		t.Errorf("done item arguments = %q, want %q", got, `{"id":"1"}`)
	}

	deltas := extractAllSSEData(t, body, "response.function_call_arguments.delta")
	if len(deltas) != 2 {
		t.Fatalf("function_call_arguments.delta count = %d, want 2; deltas=%q", len(deltas), deltas)
	}
	wantDeltas := []string{`{"id":"`, `1"}`}
	concat := ""
	for i, d := range deltas {
		var parsed struct {
			Type    string `json:"type"`
			ItemID  string `json:"item_id"`
			Delta   string `json:"delta"`
		}
		if err := json.Unmarshal([]byte(d), &parsed); err != nil {
			t.Fatalf("delta[%d] not JSON: %v", i, err)
		}
		if parsed.Type != "response.function_call_arguments.delta" {
			t.Errorf("delta[%d].type = %q, want response.function_call_arguments.delta", i, parsed.Type)
		}
		if parsed.ItemID != "call_1" {
			t.Errorf("delta[%d].item_id = %q, want call_1", i, parsed.ItemID)
		}
		if parsed.Delta != wantDeltas[i] {
			t.Errorf("delta[%d].delta = %q, want %q", i, parsed.Delta, wantDeltas[i])
		}
		concat += parsed.Delta
	}
	if concat != `{"id":"1"}` {
		t.Errorf("concat of deltas = %q, want %q", concat, `{"id":"1"}`)
	}
	if concat != doneItem["arguments"] {
		t.Errorf("concat of deltas (%q) must equal done.arguments (%q)", concat, doneItem["arguments"])
	}
}

func extractSSEData(t *testing.T, body, eventName string) string {
	t.Helper()
	prefix := "event: " + eventName
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line == prefix && i+1 < len(lines) {
			dataLine := lines[i+1]
			if strings.HasPrefix(dataLine, "data: ") {
				return strings.TrimPrefix(dataLine, "data: ")
			}
		}
	}
	return ""
}

func extractAllSSEData(t *testing.T, body, eventName string) []string {
	t.Helper()
	prefix := "event: " + eventName
	lines := strings.Split(body, "\n")
	var out []string
	for i, line := range lines {
		if line == prefix && i+1 < len(lines) {
			dataLine := lines[i+1]
			if strings.HasPrefix(dataLine, "data: ") {
				out = append(out, strings.TrimPrefix(dataLine, "data: "))
			}
		}
	}
	return out
}

func TestResponsesHandlerStreamingMalformedChunk(t *testing.T) {
	streamBody := "data: not-valid-json\n\n" +
		"data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"model\":\"kimi-k2.6\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"}}]}\n\n" +
		"data: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(streamBody))
	}))
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","stream":true,"input":"hi"}`))
	rr := httptest.NewRecorder()
	defer func() {
		// swallow panic just in case (test ensures we don't panic)
		_ = recover()
	}()
	mux.ServeHTTP(rr, req)
	if !strings.Contains(rr.Body.String(), "ok") {
		t.Errorf("expected 'ok' in stream despite malformed chunk; body:\n%s", rr.Body.String())
	}
}

func TestResponsesHandlerCodexDesktopE2E(t *testing.T) {
	upstream := startFakeUpstream(t, `{
		"id":"chatcmpl-codex",
		"object":"chat.completion",
		"model":"kimi-k2.6",
		"choices":[{
			"index":0,
			"message":{"role":"assistant","content":"ok-codex"},
			"finish_reason":"stop"
		}],
		"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}
	}`)
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	body := `{
		"model":"kimi-k2.6",
		"input":"hi from codex desktop",
		"stream":false,
		"previous_response_id":null
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v", err)
	}
	if resp["object"] != "response" {
		t.Errorf("object = %v, want response", resp["object"])
	}
	if resp["model"] != "kimi-k2.6" {
		t.Errorf("model = %v, want kimi-k2.6", resp["model"])
	}
	output := responseOutput(t, resp)
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	content, ok := output[0]["content"].([]any)
	if !ok {
		t.Fatalf("content type = %T", output[0]["content"])
	}
	if len(content) == 0 {
		t.Fatalf("content empty")
	}
	part, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("content[0] type = %T", content[0])
	}
	if part["text"] != "ok-codex" {
		t.Errorf("text = %v, want 'ok-codex'", part["text"])
	}
}

func startFakeUpstream(t *testing.T, body string, hooks ...func(*http.Request)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range hooks {
			h(r)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
}

func responseOutput(t *testing.T, resp map[string]any) []map[string]any {
	t.Helper()
	switch arr := resp["output"].(type) {
	case []any:
		out := make([]map[string]any, len(arr))
		for i, item := range arr {
			m, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("output[%d] type = %T, want map", i, item)
			}
			out[i] = m
		}
		return out
	case []map[string]any:
		return arr
	default:
		t.Fatalf("output type = %T, want []any or []map[string]any", resp["output"])
		return nil
	}
}

func responseUsage(t *testing.T, resp map[string]any) map[string]any {
	t.Helper()
	u, ok := resp["usage"].(map[string]any)
	if !ok {
		t.Fatalf("usage type = %T, want map", resp["usage"])
	}
	return u
}

func intFromUsageValue(t *testing.T, v any) int {
	t.Helper()
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		t.Fatalf("usage value type = %T, want number", v)
		return 0
	}
}

func errorsAs(err error, target any) bool {
	return errors.As(err, target)
}

func TestWriteOpenAIErrorOmitsEmptyParamAndCode(t *testing.T) {
	rr := httptest.NewRecorder()
	WriteOpenAIError(rr, http.StatusBadRequest, "x", "invalid_request_error", "", "")
	var env OpenAIErrorEnvelope
	json.Unmarshal(rr.Body.Bytes(), &env)
	if env.Error.Param != "" {
		t.Errorf("Param = %q, want empty", env.Error.Param)
	}
	if env.Error.Code != "" {
		t.Errorf("Code = %q, want empty", env.Error.Code)
	}
}

func TestResponsesHandlerNewMuxRegistersRoute(t *testing.T) {
	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","input":"x"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code == http.StatusNotFound {
		t.Fatal("/v1/responses should be registered in NewMux")
	}
}

func TestBuildResponsesResponseObjectUsageZero(t *testing.T) {
	resp := BuildResponsesResponseObject("m", "x", nil, compat.TokenUsage{})
	usage := responseUsage(t, resp)
	if intFromUsageValue(t, usage["input_tokens"]) != 0 || intFromUsageValue(t, usage["output_tokens"]) != 0 || intFromUsageValue(t, usage["total_tokens"]) != 0 {
		t.Errorf("usage = %v, want all zeros when upstream usage is absent", usage)
	}
}

func TestResponsesToChatSafeAnthropicInputPreservesImageURL(t *testing.T) {
	rr := compat.ResponsesRequest{
		Model: "kimi-k2",
		Input: json.RawMessage(`[{"role":"user","content":[{"type":"input_text","text":"hi"},{"type":"input_image","image_url":"https://example.com/cat.png"}]}]`),
	}
	or := ResponsesToChatSafe(rr)
	if len(or.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(or.Messages))
	}
	parts, ok := or.Messages[0].Content.([]compat.OAIContentPart)
	if !ok {
		t.Fatalf("content = %T, want []OAIContentPart", or.Messages[0].Content)
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "https://example.com/cat.png" {
		t.Errorf("image part = %+v", parts[1])
	}
}

func TestConvertAnthropicToResponsesShape(t *testing.T) {
	parsed := AnthropicParsedResponse{
		Text: "Hello",
		Usage: compat.TokenUsage{
			InputTokens:  3,
			OutputTokens: 2,
			TotalTokens:  5,
			Present:      true,
		},
	}
	resp := ConvertAnthropicToResponses("claude-test", parsed)
	if resp["object"] != "response" {
		t.Errorf("object = %v, want response", resp["object"])
	}
	output := responseOutput(t, resp)
	if len(output) != 1 {
		t.Fatalf("output len = %d, want 1", len(output))
	}
	if output[0]["type"] != "message" {
		t.Errorf("output[0].type = %v, want message", output[0]["type"])
	}
	usage := responseUsage(t, resp)
	if intFromUsageValue(t, usage["input_tokens"]) != 3 {
		t.Errorf("usage.input_tokens = %v, want 3", usage["input_tokens"])
	}
}

func TestStreamChatCompletionAsResponsesEmptyBody(t *testing.T) {
	rr := httptest.NewRecorder()
	StreamChatCompletionAsResponses(rr, bytes.NewReader(nil), "m")
	body := rr.Body.String()
	if !strings.Contains(body, "data: [DONE]") {
		t.Errorf("expected [DONE] for empty body; got: %s", body)
	}
}

func TestResponseStreamToolCallAccumulatorDeltasAreIncremental(t *testing.T) {
	acc := newResponseStreamToolCallAccumulator()
	acc.Absorb(responseStreamToolCallDelta{Index: 0, ID: "call_1", Name: "lookup"})
	evs := acc.DrainEvents()
	if !eventHasOutputItemAdded(evs) {
		t.Errorf("first chunk with ID should emit output_item.added; got %v", evs)
	}
	got := decodeToolDeltas(t, evs)
	if len(got) != 0 {
		t.Errorf("first chunk (empty args) should not emit delta; got %v", got)
	}
	if !acc.HasEmittedAdded(0) {
		t.Errorf("first chunk with ID should mark the call as Added")
	}

	acc.Absorb(responseStreamToolCallDelta{Index: 0, Arguments: `{"id":"`})
	got = decodeToolDeltas(t, acc.DrainEvents())
	if len(got) != 1 || got[0] != `{"id":"` {
		t.Errorf("second chunk delta = %v, want [{\"id\":\"\"]", got)
	}

	acc.Absorb(responseStreamToolCallDelta{Index: 0, Arguments: `1"}`})
	got = decodeToolDeltas(t, acc.DrainEvents())
	if len(got) != 1 || got[0] != `1"}` {
		t.Errorf("third chunk delta = %v, want [1\"}]", got)
	}

	acc.Absorb(responseStreamToolCallDelta{Index: 0, Arguments: ""})
	got = decodeToolDeltas(t, acc.DrainEvents())
	if len(got) != 0 {
		t.Errorf("empty args delta should be a no-op; got %v", got)
	}

	done := decodeToolDeltas(t, acc.FlushDone())
	if len(done) != 1 || done[0] != `{"id":"1"}` {
		t.Errorf("done item arguments = %v, want {\"id\":\"1\"}", done)
	}
}

func deltasContainAdded(acc *responseStreamToolCallAccumulator) bool {
	return acc.HasEmittedAdded(0)
}

func eventHasOutputItemAdded(events []responseStreamEvent) bool {
	for _, ev := range events {
		if ev.Event == "response.output_item.added" {
			return true
		}
	}
	return false
}

// ResponseStreamAccumulatorSnapshot returns a snapshot of which events
// have been emitted so far, without consuming future drains. It is
// used by the test only to confirm the Added event was emitted on the
// first chunk.
func (a *responseStreamToolCallAccumulator) HasEmittedAdded(index int) bool {
	state, ok := a.calls[index]
	return ok && state.Added
}

func decodeToolDeltas(t *testing.T, events []responseStreamEvent) []string {
	t.Helper()
	var out []string
	for _, ev := range events {
		if ev.Event == "response.function_call_arguments.delta" {
			var parsed struct {
				Delta string `json:"delta"`
			}
			if err := json.Unmarshal([]byte(ev.Data), &parsed); err != nil {
				t.Fatalf("delta not JSON: %v", err)
			}
			out = append(out, parsed.Delta)
			continue
		}
		if ev.Event == "response.output_item.done" {
			var parsed struct {
				Arguments string `json:"arguments"`
			}
			if err := json.Unmarshal([]byte(ev.Data), &parsed); err != nil {
				t.Fatalf("done item not JSON: %v", err)
			}
			out = append(out, parsed.Arguments)
		}
	}
	return out
}

func TestResponsesHandlerJSONDecodeErrorShape(t *testing.T) {
	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`not even close`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v", err)
	}
	if !strings.Contains(env.Error.Message, "Invalid JSON") {
		t.Errorf("error message = %q, want 'Invalid JSON'", env.Error.Message)
	}
}

func TestResponsesHandlerUpstreamOpenAIMalformedBody(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a chat completion JSON"))
	}))
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("status = %d, want != 200 for malformed upstream; body=%s", rr.Code, rr.Body.String())
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v; raw=%s", err, rr.Body.String())
	}
	if env.Error.Type != "upstream_error" {
		t.Errorf("type = %q, want upstream_error", env.Error.Type)
	}
}

func TestResponsesHandlerUpstreamOpenAIErrorStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream boom","type":"upstream_error","code":"server_error"}}`))
	}))
	defer upstream.Close()
	old := OpenAIUpstreamURL
	OpenAIUpstreamURL = upstream.URL
	defer func() { OpenAIUpstreamURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"kimi-k2.6","input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rr.Code, rr.Body.String())
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v; raw=%s", err, rr.Body.String())
	}
	if env.Error.Message != "upstream boom" {
		t.Errorf("message = %q, want 'upstream boom'", env.Error.Message)
	}
	if env.Error.Type != "upstream_error" {
		t.Errorf("type = %q, want upstream_error", env.Error.Type)
	}
	if env.Error.Code != "server_error" {
		t.Errorf("code = %q, want server_error", env.Error.Code)
	}
}

func TestResponsesHandlerUpstreamAnthropicErrorStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"server_error","message":"anthropic boom"}}`))
	}))
	defer upstream.Close()
	old := AnthropicURL
	AnthropicURL = upstream.URL
	defer func() { AnthropicURL = old }()

	mux := NewMux(config.Config{Host: "127.0.0.1", Port: 0, APIKey: "test"})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"minimax-m3","input":"hi"}`))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rr.Code, rr.Body.String())
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body not JSON: %v; raw=%s", err, rr.Body.String())
	}
	if env.Error.Message != "anthropic boom" {
		t.Errorf("message = %q, want 'anthropic boom'", env.Error.Message)
	}
	if env.Error.Type != "server_error" {
		t.Errorf("type = %q, want server_error", env.Error.Type)
	}
}
