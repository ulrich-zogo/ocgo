package tokens

import (
	"errors"
	"strings"
	"testing"
)

func TestEstimateTextTokensEmpty(t *testing.T) {
	if got := EstimateTextTokens(""); got != 0 {
		t.Errorf("EstimateTextTokens(\"\") = %d, want 0", got)
	}
	if got := EstimateTextTokens("   \t\n  "); got != 0 {
		t.Errorf("EstimateTextTokens(whitespace) = %d, want 0", got)
	}
}

func TestEstimateTextTokensShortIsAtLeastOne(t *testing.T) {
	if got := EstimateTextTokens("hello"); got < 1 {
		t.Errorf("EstimateTextTokens(\"hello\") = %d, want >= 1", got)
	}
	if got := EstimateTextTokens("a"); got != 1 {
		t.Errorf("EstimateTextTokens(\"a\") = %d, want 1", got)
	}
}

func TestEstimateTextTokensDeterministic(t *testing.T) {
	long := strings.Repeat("alpha bravo charlie delta echo foxtrot golf hotel india juliet ", 10)
	first := EstimateTextTokens(long)
	for i := 0; i < 5; i++ {
		again := EstimateTextTokens(long)
		if again != first {
			t.Fatalf("non-deterministic result: %d vs %d", first, again)
		}
	}
	if first < 50 {
		t.Errorf("long text estimate = %d, want >= 50", first)
	}
}

func TestEstimateTextTokensWordsFloor(t *testing.T) {
	// A long sequence of single-char "words" would otherwise be
	// under-counted by chars/4: 100 spaces + 100 chars => chars/4
	// would give ~50. We want at least 100 (the word count).
	text := strings.Repeat("a ", 100)
	got := EstimateTextTokens(text)
	if got < 100 {
		t.Errorf("EstimateTextTokens(spaced a*100) = %d, want >= 100 (word floor)", got)
	}
}

func TestEstimateTextTokensHandlesUTF8(t *testing.T) {
	// 16 runes (4 per token expected => 4). Trimming must not eat
	// the last space.
	french := "Bonjour à toi comment ça va très bien merci"
	got := EstimateTextTokens(french)
	if got < 6 {
		t.Errorf("EstimateTextTokens(french) = %d, want >= 6", got)
	}
}

func TestEstimateJSONTokensCountsNestedText(t *testing.T) {
	raw := []byte(`{"messages":[{"role":"user","content":"Bonjour le monde"}]}`)
	got := EstimateJSONTokens(raw)
	if got < 5 {
		t.Errorf("EstimateJSONTokens(nested) = %d, want >= 5", got)
	}
	// Includes the per-request overhead.
	if got < RequestOverheadTokens {
		t.Errorf("EstimateJSONTokens(nested) = %d, want >= RequestOverheadTokens(%d)", got, RequestOverheadTokens)
	}
}

func TestEstimateJSONTokensInvalidFallsBackToText(t *testing.T) {
	raw := []byte(`{not json`)
	got := EstimateJSONTokens(raw)
	if got < 1 {
		t.Errorf("EstimateJSONTokens(invalid) = %d, want >= 1 (text fallback)", got)
	}
}

func TestEstimateJSONTokensEmpty(t *testing.T) {
	if got := EstimateJSONTokens(nil); got != 0 {
		t.Errorf("EstimateJSONTokens(nil) = %d, want 0", got)
	}
	if got := EstimateJSONTokens([]byte("")); got != 0 {
		t.Errorf("EstimateJSONTokens(empty) = %d, want 0", got)
	}
}

func TestEstimateAnthropicCountTokensSystemMessagesTools(t *testing.T) {
	raw := []byte(`{
		"model":"minimax-m3",
		"system":"Tu es strict.",
		"messages":[
			{"role":"user","content":"Bonjour"},
			{"role":"assistant","content":[
				{"type":"text","text":"Bonjour, comment aider ?"}
			]},
			{"role":"user","content":[
				{"type":"tool_use","name":"lookup","input":{"id":42}}
			]},
			{"role":"assistant","content":[
				{"type":"tool_result","tool_use_id":"x1","content":"42"}
			]}
		],
		"tools":[
			{"name":"lookup","description":"Lookup data","input_schema":{"type":"object"}}
		]
	}`)

	est, err := EstimateAnthropicCountTokens(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 4 messages * MessageOverhead + system + 1 tool * ToolOverhead
	// + actual text. We just want a strictly positive number well
	// above the request overhead.
	wantMin := RequestOverheadTokens +
		4*MessageOverheadTokens +
		1*ToolOverheadTokens +
		EstimateTextTokens("Tu es strict.") +
		EstimateTextTokens("Bonjour") +
		EstimateTextTokens("Bonjour, comment aider ?")
	if est.InputTokens < wantMin {
		t.Errorf("anthropic estimate = %d, want >= %d", est.InputTokens, wantMin)
	}
}

func TestEstimateAnthropicCountTokensImageAddsOverhead(t *testing.T) {
	noImg, err := EstimateAnthropicCountTokens([]byte(`{
		"messages":[{"role":"user","content":"hello"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	withImg, err := EstimateAnthropicCountTokens([]byte(`{
		"messages":[{"role":"user","content":[
			{"type":"image","source":{"type":"base64","data":"..."}}
		]}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withImg.InputTokens-noImg.InputTokens < ImageOverheadTokens {
		t.Errorf("image overhead delta = %d, want >= %d",
			withImg.InputTokens-noImg.InputTokens, ImageOverheadTokens)
	}
}

func TestEstimateAnthropicCountTokensInvalidJSON(t *testing.T) {
	_, err := EstimateAnthropicCountTokens([]byte(`{not json`))
	if err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
	if !strings.HasPrefix(err.Error(), "invalid JSON:") {
		t.Errorf("error = %q, want invalid JSON prefix", err.Error())
	}
}

func TestEstimateAnthropicCountTokensEmpty(t *testing.T) {
	_, err := EstimateAnthropicCountTokens(nil)
	if err == nil {
		t.Fatalf("expected error for empty body")
	}
	if !errors.Is(err, ErrEmptyBody{}) {
		t.Errorf("error = %v, want ErrEmptyBody", err)
	}
}

func TestEstimateOpenAIChatTokensMessagesAndTools(t *testing.T) {
	raw := []byte(`{
		"model":"kimi-k2.6",
		"messages":[
			{"role":"system","content":"Tu es strict."},
			{"role":"user","content":"Bonjour"},
			{"role":"assistant","content":null,"tool_calls":[
				{"id":"call_1","type":"function","function":{
					"name":"lookup","arguments":"{\"id\":42}"
				}}
			]}
		],
		"tools":[
			{"type":"function","function":{
				"name":"lookup","description":"Lookup","parameters":{"type":"object"}
			}}
		]
	}`)

	est, err := EstimateOpenAIChatTokens(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.InputTokens < 1 {
		t.Errorf("estimate = %d, want >= 1", est.InputTokens)
	}
}

func TestEstimateOpenAIChatTokensImage(t *testing.T) {
	est, err := EstimateOpenAIChatTokens([]byte(`{
		"messages":[{"role":"user","content":[
			{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}
		]}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.InputTokens < ImageOverheadTokens {
		t.Errorf("image estimate = %d, want >= %d", est.InputTokens, ImageOverheadTokens)
	}
}

func TestEstimateOpenAIChatTokensInvalidJSON(t *testing.T) {
	if _, err := EstimateOpenAIChatTokens([]byte(`{`)); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestEstimateResponsesTokensInstructionsInputString(t *testing.T) {
	est, err := EstimateResponsesTokens([]byte(`{
		"model":"kimi-k2.6",
		"instructions":"Tu es strict.",
		"input":"Réponds uniquement OK."
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if est.InputTokens < 1 {
		t.Errorf("estimate = %d, want >= 1", est.InputTokens)
	}
}

func TestEstimateResponsesTokensInputArrayAndImage(t *testing.T) {
	est, err := EstimateResponsesTokens([]byte(`{
		"model":"kimi-k2.6",
		"instructions":"sys",
		"input":[
			{"role":"user","content":[
				{"type":"input_text","text":"Bonjour"},
				{"type":"input_image"}
			]},
			{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{\"a\":1}"},
			{"type":"function_call_output","call_id":"c1","output":"42"}
		],
		"tools":[
			{"type":"function","name":"lookup","description":"L","parameters":{"type":"object"}}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// We want a number that includes the image overhead, the
	// function_call + function_call_output text, and the tool
	// overhead.
	if est.InputTokens < ImageOverheadTokens+ToolOverheadTokens+RequestOverheadTokens {
		t.Errorf("estimate = %d, want >= %d",
			est.InputTokens,
			ImageOverheadTokens+ToolOverheadTokens+RequestOverheadTokens)
	}
}

func TestEstimateResponsesTokensFunctionCallArgumentsAreCounted(t *testing.T) {
	noCall, err := EstimateResponsesTokens([]byte(`{
		"input":[{"role":"user","content":"hi"}]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	withCall, err := EstimateResponsesTokens([]byte(`{
		"input":[
			{"role":"user","content":"hi"},
			{"type":"function_call","call_id":"c1","name":"lookup","arguments":"{\"a\":1,\"b\":2,\"c\":3}"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withCall.InputTokens <= noCall.InputTokens {
		t.Errorf("function_call did not increase estimate: %d vs %d",
			withCall.InputTokens, noCall.InputTokens)
	}
}

func TestEstimateResponsesTokensFunctionCallOutputIsCounted(t *testing.T) {
	noOut, err := EstimateResponsesTokens([]byte(`{
		"input":[
			{"role":"user","content":"hi"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	withOut, err := EstimateResponsesTokens([]byte(`{
		"input":[
			{"role":"user","content":"hi"},
			{"type":"function_call_output","call_id":"c1","output":"this is the tool result that should be counted"}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withOut.InputTokens <= noOut.InputTokens {
		t.Errorf("function_call_output did not increase estimate: %d vs %d",
			withOut.InputTokens, noOut.InputTokens)
	}
}

func TestEstimateResponsesTokensInvalidJSON(t *testing.T) {
	if _, err := EstimateResponsesTokens([]byte(`{`)); err == nil {
		t.Fatalf("expected error for invalid JSON")
	}
}

func TestEstimateResponsesTokensEmpty(t *testing.T) {
	_, err := EstimateResponsesTokens([]byte("   "))
	if err == nil {
		t.Fatalf("expected error for empty body")
	}
}

func TestEstimateDeterministicAcrossShapes(t *testing.T) {
	// Two equivalent Responses payloads in different key orders
	// should produce the same estimate.
	raw1 := []byte(`{"instructions":"hi","input":[{"role":"user","content":"hello"}]}`)
	raw2 := []byte(`{"input":[{"content":"hello","role":"user"}],"instructions":"hi"}`)
	e1, err := EstimateResponsesTokens(raw1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e2, err := EstimateResponsesTokens(raw2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e1.InputTokens != e2.InputTokens {
		t.Errorf("non-deterministic estimate: %d vs %d", e1.InputTokens, e2.InputTokens)
	}
}
