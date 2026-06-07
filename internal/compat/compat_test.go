package compat

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponsesInputToMessages(t *testing.T) {
	messages := ResponsesInputToMessages([]byte(`[{"type":"message","role":"developer","content":"rules"},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]`))
	if len(messages) != 2 {
		t.Fatalf("got %d messages", len(messages))
	}
	if messages[0].Role != "developer" || messages[0].Content != "rules" {
		t.Fatalf("bad developer conversion: %+v", messages[0])
	}
	if messages[1].Role != "user" || messages[1].Content != "hello" {
		t.Fatalf("bad user conversion: %+v", messages[1])
	}
}

func TestResponsesInputFunctionCallUsesCallID(t *testing.T) {
	messages := ResponsesInputToMessages([]byte(`[{"type":"tool_call","id":"call_123","function":{"name":"shell","arguments":"{\"cmd\":\"pwd\"}"}},{"type":"tool_result","call_id":"call_123","content":"hello from tool"}]`))
	if len(messages) != 2 {
		t.Fatalf("got %d messages", len(messages))
	}
	if len(messages[0].ToolCalls) != 1 || messages[0].ToolCalls[0].ID != "call_123" {
		t.Fatalf("tool call should have ID call_123: %+v", messages[0].ToolCalls)
	}
	if messages[1].ToolCallID != "call_123" {
		t.Fatalf("bad tool result ID: %+v", messages[1])
	}
}

func TestAnthropicToolUseHistoryIncludesFallbackReasoning(t *testing.T) {
	messages := ContentToOpenAI(AMessage{Role: "assistant", Content: []byte(`[{"type":"tool_use","id":"call_123","name":"Bash","input":{"command":"pwd"}}]`)})
	if len(messages) != 1 {
		t.Fatalf("got %d messages", len(messages))
	}
	if messages[0].Role != "assistant" || len(messages[0].ToolCalls) != 1 {
		t.Fatalf("bad tool call conversion: %+v", messages[0])
	}
}

func TestAnthropicToolResultPreservesFollowingUserText(t *testing.T) {
	messages := ContentToOpenAI(AMessage{Role: "user", Content: []byte(`[{"type":"tool_result","tool_use_id":"call_123","content":"09:33:16"},{"type":"text","text":"https://figma.example/design what's going on here?"}]`)})
	if len(messages) != 2 {
		t.Fatalf("got %d messages: %+v", len(messages), messages)
	}
	if messages[0].Role != "tool" || messages[0].ToolCallID != "call_123" || messages[0].Content != "09:33:16" {
		t.Fatalf("bad tool result conversion: %+v", messages[0])
	}
	contentStr, ok := messages[1].Content.(string)
	if !ok || !strings.Contains(contentStr, "figma.example") {
		t.Fatalf("following user text was not preserved: %+v", messages[1])
	}
}

func TestResponsesInputPreservesImages(t *testing.T) {
	messages := ResponsesInputToMessages([]byte(`[{"type":"message","role":"user","content":[{"type":"input_text","text":"describe this"},{"type":"input_image","image_url":"data:image/png;base64,abc","detail":"high"}]}]`))
	if len(messages) != 1 {
		t.Fatalf("got %d messages", len(messages))
	}
	parts, ok := messages[0].Content.([]OAIContentPart)
	if !ok {
		t.Fatalf("content should be multimodal parts: %+v", messages[0].Content)
	}
	if len(parts) != 2 || parts[0].Type != "text" || parts[0].Text != "describe this" {
		t.Fatalf("bad text part: %+v", parts)
	}
	if parts[1].Type != "image_url" || parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/png;base64,abc" || parts[1].ImageURL.Detail != "auto" {
		t.Fatalf("bad image part: %+v", parts[1])
	}
}

func TestAnthropicContentPreservesImages(t *testing.T) {
	messages := ContentToOpenAI(AMessage{Role: "user", Content: []byte(`[{"type":"text","text":"what is this?"},{"type":"image","source":{"type":"base64","media_type":"image/jpeg","data":"abc"}}]`)})
	if len(messages) != 1 {
		t.Fatalf("got %d messages", len(messages))
	}
	parts, ok := messages[0].Content.([]OAIContentPart)
	if !ok {
		t.Fatalf("content should be multimodal parts: %+v", messages[0].Content)
	}
	if len(parts) != 2 || parts[0].Text != "what is this?" {
		t.Fatalf("bad text part: %+v", parts)
	}
	if parts[1].ImageURL == nil || parts[1].ImageURL.URL != "data:image/jpeg;base64,abc" {
		t.Fatalf("bad image part: %+v", parts[1])
	}
}

func TestSanitizeOAIToolMessagesInsertsMissingBeforeNextMessage(t *testing.T) {
	in := []OAIMessage{
		{Role: "user", Content: "run pwd"},
		{Role: "assistant", ToolCalls: []OAIToolCall{{ID: "call_missing", Type: "function", Function: OAICallFunction{Name: "Bash"}}}},
		{Role: "assistant", Content: "done"},
	}
	out := SanitizeOAIToolMessages(in)
	if len(out) != len(in) {
		t.Fatalf("no reordering needed for mixed assistant messages, got %+v", out)
	}
}

func TestSanitizeOAIToolMessagesDropsLateToolMessage(t *testing.T) {
	in := []OAIMessage{
		{Role: "assistant", ToolCalls: []OAIToolCall{{ID: "call_1", Type: "function", Function: OAICallFunction{Name: "Bash"}}}},
		{Role: "assistant", Content: "done"},
		{Role: "tool", ToolCallID: "call_1", Content: "late result"},
	}
	out := SanitizeOAIToolMessages(in)
	if len(out) != len(in) {
		t.Fatalf("no reordering needed for interleaved tool message, got %+v", out)
	}
	if out[0].ToolCalls[0].ID != "call_1" {
		t.Fatalf("first assistant should have tool calls: %+v", out[0])
	}
	if out[2].Role != "tool" || out[2].ToolCallID != "call_1" {
		t.Fatalf("tool message should be preserved: %+v", out[2])
	}
}

func TestSanitizeOAIToolMessagesPreservesValidConsecutiveResults(t *testing.T) {
	in := []OAIMessage{
		{Role: "assistant", ToolCalls: []OAIToolCall{
			{ID: "call_a", Type: "function", Function: OAICallFunction{Name: "Bash"}},
			{ID: "call_b", Type: "function", Function: OAICallFunction{Name: "Read"}},
		}},
		{Role: "tool", ToolCallID: "call_b", Content: "b"},
		{Role: "tool", ToolCallID: "call_a", Content: "a"},
		{Role: "user", Content: "thanks"},
	}
	out := SanitizeOAIToolMessages(in)
	if len(out) != len(in) {
		t.Fatalf("valid sequence should remain same length, got %+v", out)
	}
	if out[1].Content != "a" || out[2].Content != "b" {
		t.Fatalf("tool results should be reordered to match call IDs: %+v", out)
	}
}

func TestSanitizeRawChatToolMessagesPreservesUnknownFields(t *testing.T) {
	body := []byte(`{"model":"deepseek-v4-pro","messages":[{"role":"assistant","content":"","name":"assistant-name","tool_calls":[{"id":"call_1","type":"function","function":{"name":"Bash","arguments":"{}"}}],"audio":{"id":"aud"}},{"role":"assistant","content":"done","refusal":"no"}]}`)
	out := SanitizeRawChatToolMessages(body)
	var req map[string]json.RawMessage
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatal(err)
	}
	var messages []map[string]json.RawMessage
	if err := json.Unmarshal(req["messages"], &messages); err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected unchanged messages, got %d: %s", len(messages), out)
	}
	if _, ok := messages[0]["name"]; !ok {
		t.Fatalf("unknown assistant field name was dropped: %s", messages[0])
	}
	if _, ok := messages[0]["audio"]; !ok {
		t.Fatalf("unknown assistant field audio was dropped: %s", messages[0])
	}
	if _, ok := messages[1]["refusal"]; !ok {
		t.Fatalf("unknown later assistant field refusal was dropped: %s", messages[1])
	}
}

func TestSanitizeRawChatToolMessagesDropsLateToolMessage(t *testing.T) {
	body := []byte(`{"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"Bash","arguments":"{}"}}]},{"role":"assistant","content":"done"},{"role":"tool","tool_call_id":"call_1","content":"late"}]}`)
	out := SanitizeRawChatToolMessages(body)
	var req map[string]json.RawMessage
	if err := json.Unmarshal(out, &req); err != nil {
		t.Fatal(err)
	}
	var roles []struct {
		Role       string `json:"role"`
		ToolCallID string `json:"tool_call_id"`
		Content    string `json:"content"`
	}
	if err := json.Unmarshal(req["messages"], &roles); err != nil {
		t.Fatal(err)
	}
	if len(roles) != 3 {
		t.Fatalf("expected unchanged message count, got %+v", roles)
	}
	if roles[0].Role != "assistant" {
		t.Fatalf("expected assistant at index 0, got %+v", roles[0])
	}
	if !strings.Contains(string(out), `"tool_calls"`) {
		t.Fatalf("assistant tool_calls array was lost: %s", out)
	}
}

func TestParseOpenAIStreamChunkReadsUsageOnly(t *testing.T) {
	chunk := ParseOpenAIStreamChunk([]byte(`{"choices":[],"usage":{"prompt_tokens":8,"completion_tokens":4,"total_tokens":12}}`))
	if !chunk.Usage.Present || chunk.Usage.InputTokens != 8 || chunk.Usage.OutputTokens != 4 || chunk.Usage.TotalTokens != 12 {
		t.Fatalf("bad stream usage: %+v", chunk.Usage)
	}
	if chunk.Content != "" || len(chunk.ToolCalls) != 0 {
		t.Fatalf("usage-only chunk should not include deltas: %+v", chunk)
	}
}

func TestNormalizeReasoningEffort(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{in: "minimal", want: "minimal"},
		{in: "0", want: "minimal"},
		{in: "low", want: "low"},
		{in: "1", want: "low"},
		{in: "medium", want: "medium"},
		{in: "2", want: "medium"},
		{in: "high", want: "high"},
		{in: "xhigh", want: "high"},
		{in: "max", want: "high"},
	} {
		if got := NormalizeReasoningEffort(tc.in); got != tc.want {
			t.Fatalf("NormalizeReasoningEffort(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestReasoningEffortExtraction(t *testing.T) {
	for _, tc := range []struct {
		raw  json.RawMessage
		want string
	}{
		{raw: []byte(`"low"`), want: "low"},
		{raw: []byte(`3`), want: "high"},
		{raw: []byte(`{"level":"medium"}`), want: "medium"},
		{raw: []byte(`{"type":"enabled"}`), want: "high"},
		{raw: []byte(`{"reasoning":{"depth":1}}`), want: "low"},
	} {
		if got := ReasoningEffortFromRaw(tc.raw); got != tc.want {
			t.Fatalf("ReasoningEffortFromRaw(%s) = %q, want %q", string(tc.raw), got, tc.want)
		}
	}
}
