package proxy

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"ocgo/internal/compat"
	"ocgo/internal/config"
	"ocgo/internal/mapping"
	"ocgo/internal/models"
)

type ResponsesValidationError struct {
	Status  int
	Message string
	Param   string
	Code    string
}

// OpenAIUpstreamURL allows tests to redirect Chat Completions forwarding.
// Production callers should leave this at the empty string (the package
// falls back to config.OpenAIURL at request time).
var OpenAIUpstreamURL = ""

func openAIUpstreamURL() string {
	if OpenAIUpstreamURL != "" {
		return OpenAIUpstreamURL
	}
	return config.OpenAIURL
}

type responsesStreamChunk struct {
	Content string
	ToolCalls []struct {
		ID        string
		Name      string
		Arguments string
	}
	Usage compat.TokenUsage
}

func parseStreamChunkForResponses(data []byte) responsesStreamChunk {
	chunk := compat.ParseOpenAIStreamChunk(data)
	out := responsesStreamChunk{
		Content: chunk.Content,
		Usage:   chunk.Usage,
	}
	for _, tc := range chunk.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, struct {
			ID        string
			Name      string
			Arguments string
		}{
			ID:        tc.ID,
			Name:      extractStreamToolName(data, tc.Index),
			Arguments: extractStreamToolArguments(data, tc.Index, tc.Arguments),
		})
	}
	return out
}

func extractStreamToolName(data []byte, index int) string {
	var raw struct {
		Choices []struct {
			Delta struct {
				ToolCalls []struct {
					Index    int    `json:"index"`
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(stripDataPrefix(data), &raw); err != nil {
		return ""
	}
	for _, c := range raw.Choices {
		for _, tc := range c.Delta.ToolCalls {
			if tc.Index == index {
				return tc.Function.Name
			}
		}
	}
	return ""
}

func extractStreamToolArguments(data []byte, index int, fallback string) string {
	var raw struct {
		Choices []struct {
			Delta struct {
				ToolCalls []struct {
					Index    int    `json:"index"`
					Function struct {
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(stripDataPrefix(data), &raw); err != nil {
		return fallback
	}
	for _, c := range raw.Choices {
		for _, tc := range c.Delta.ToolCalls {
			if tc.Index == index {
				if tc.Function.Arguments != "" {
					return tc.Function.Arguments
				}
				return fallback
			}
		}
	}
	return fallback
}

func stripDataPrefix(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte("data: "))
}

func (e *ResponsesValidationError) Error() string { return e.Message }

func ValidateResponsesRequest(rr compat.ResponsesRequest) error {
	if strings.TrimSpace(rr.Model) == "" {
		return &ResponsesValidationError{
			Status:  http.StatusBadRequest,
			Message: "Invalid request: model is required",
			Param:   "model",
			Code:    "invalid_request",
		}
	}
	hasInstructions := strings.TrimSpace(rr.Instructions) != ""
	hasInput := len(rr.Input) > 0 && !isEmptyResponsesInput(rr.Input)
	if !hasInstructions && !hasInput {
		return &ResponsesValidationError{
			Status:  http.StatusBadRequest,
			Message: "Invalid request: either 'input' or 'instructions' must be provided",
			Param:   "input",
			Code:    "invalid_request",
		}
	}
	if rr.MaxTokens < 0 {
		return &ResponsesValidationError{
			Status:  http.StatusBadRequest,
			Message: "Invalid request: max_output_tokens must be positive",
			Param:   "max_output_tokens",
			Code:    "invalid_request",
		}
	}
	return nil
}

func isEmptyResponsesInput(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s) == ""
	}
	var arr []json.RawMessage
	if json.Unmarshal(raw, &arr) == nil {
		return len(arr) == 0
	}
	var items []map[string]any
	if json.Unmarshal(raw, &items) == nil {
		return len(items) == 0
	}
	return false
}

func DecodeResponsesRequest(body io.Reader) (compat.ResponsesRequest, error) {
	var rr compat.ResponsesRequest
	dec := json.NewDecoder(body)
	dec.UseNumber()
	if err := dec.Decode(&rr); err != nil {
		return rr, err
	}
	return rr, nil
}

func ResponsesToChatSafe(rr compat.ResponsesRequest) compat.OAIRequest {
	or := compat.ResponsesInputToMessages(rr.Input)
	if rr.Instructions != "" {
		inserted := make([]compat.OAIMessage, 0, len(or)+1)
		inserted = append(inserted, compat.OAIMessage{Role: "system", Content: rr.Instructions})
		inserted = append(inserted, or...)
		or = inserted
	}
	tools := ConvertResponsesTools(rr.Tools)
	out := compat.OAIRequest{
		Model:       rr.Model,
		Messages:    or,
		Stream:      rr.Stream,
		MaxTokens:   rr.MaxTokens,
		Temperature: rr.Temperature,
		TopP:        rr.TopP,
		Tools:       tools,
	}
	if rr.Thinking != nil {
		out.ReasoningEffort = "medium"
	}
	for _, raw := range []json.RawMessage{rr.Reasoning, rr.ReasoningEffort, rr.Effort, rr.Level, rr.Depth, rr.OutputConfig} {
		if effort := compat.ReasoningEffortFromRaw(raw); effort != "" && out.ReasoningEffort == "" {
			out.ReasoningEffort = effort
		}
	}
	out.Model = mapping.ResolveToolModel("codex", rr.Model)
	return out
}

func ConvertResponsesTools(tools []compat.ResponseTool) []compat.OAITool {
	var out []compat.OAITool
	for _, t := range tools {
		switch t.Type {
		case "function", "custom":
			out = append(out, compat.OAITool{
				Type: "function",
				Function: compat.OAIFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  compat.ToolParametersOrDefault(t.Parameters),
				},
			})
		}
	}
	return out
}

func BuildResponsesResponseObject(model string, text string, toolCalls []compat.OAIToolCall, usage compat.TokenUsage) map[string]any {
	now := time.Now().UTC()
	id := "resp_" + fmt.Sprintf("%d", now.UnixNano())
	createdAt := now.Unix()
	output := []map[string]any{}
	if text != "" {
		output = append(output, map[string]any{
			"type":   "message",
			"id":     "msg_" + fmt.Sprintf("%d", now.UnixNano()),
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": text, "annotations": []any{}},
			},
		})
	}
	for _, tc := range toolCalls {
		var args json.RawMessage
		if tc.Function.Arguments != "" {
			args = json.RawMessage(tc.Function.Arguments)
		} else {
			args = json.RawMessage("{}")
		}
		output = append(output, map[string]any{
			"type":      "function_call",
			"id":        tc.ID,
			"call_id":   tc.ID,
			"name":      tc.Function.Name,
			"arguments": string(args),
			"status":    "completed",
		})
	}
	resp := map[string]any{
		"id":         id,
		"object":     "response",
		"created_at": createdAt,
		"status":     "completed",
		"model":      model,
		"output":     output,
		"usage":      compat.ResponsesUsage(usage),
	}
	return resp
}

func WriteResponsesResponseObject(w http.ResponseWriter, model string, text string, toolCalls []compat.OAIToolCall, usage compat.TokenUsage) {
	resp := BuildResponsesResponseObject(model, text, toolCalls, usage)
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(resp)
	w.Write(b)
}

func WriteResponsesError(w http.ResponseWriter, err error) {
	var ve *ResponsesValidationError
	if errors.As(err, &ve) {
		WriteOpenAIError(w, ve.Status, ve.Message, "invalid_request_error", ve.Param, ve.Code)
		return
	}
	WriteOpenAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "", "invalid_request")
}

func StreamChatCompletionAsResponses(w http.ResponseWriter, body io.Reader, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	createdEvent := map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id":     "resp_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			"object": "response",
			"status": "in_progress",
			"model":  model,
		},
	}
	if b, err := json.Marshal(createdEvent); err == nil {
		fmt.Fprintf(w, "event: response.created\ndata: %s\n\n", b)
		flusher.Flush()
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var mu sync.Mutex
	var lastUsage compat.TokenUsage

	flushDone := func() {
		mu.Lock()
		defer mu.Unlock()
		completed := map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     "resp",
				"object": "response",
				"status": "completed",
				"model":  model,
				"usage":  compat.ResponsesUsage(lastUsage),
			},
		}
		if b, err := json.Marshal(completed); err == nil {
			fmt.Fprintf(w, "event: response.completed\ndata: %s\n\n", b)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		chunk := parseStreamChunkForResponses([]byte(data))
		if chunk.Content != "" {
			delta := map[string]any{
				"type":  "response.output_text.delta",
				"delta": chunk.Content,
			}
			mu.Lock()
			if b, err := json.Marshal(delta); err == nil {
				fmt.Fprintf(w, "event: response.output_text.delta\ndata: %s\n\n", b)
				flusher.Flush()
			}
			mu.Unlock()
		}
		if len(chunk.ToolCalls) > 0 {
			for _, tc := range chunk.ToolCalls {
				args := tc.Arguments
				itemAdded := map[string]any{
					"type":     "response.output_item.added",
					"item": map[string]any{
						"type":    "function_call",
						"id":      tc.ID,
						"call_id": tc.ID,
						"name":    tc.Name,
					},
				}
				mu.Lock()
				if b, err := json.Marshal(itemAdded); err == nil {
					fmt.Fprintf(w, "event: response.output_item.added\ndata: %s\n\n", b)
				}
				if args != "" {
					argDelta := map[string]any{
						"type": "response.function_call_arguments.delta",
						"item_id": tc.ID,
						"delta":   args,
					}
					if b, err := json.Marshal(argDelta); err == nil {
						fmt.Fprintf(w, "event: response.function_call_arguments.delta\ndata: %s\n\n", b)
					}
				}
				itemDone := map[string]any{
					"type": "response.output_item.done",
					"item": map[string]any{
						"type":      "function_call",
						"id":        tc.ID,
						"call_id":   tc.ID,
						"name":      tc.Name,
						"arguments": args,
					},
				}
				if b, err := json.Marshal(itemDone); err == nil {
					fmt.Fprintf(w, "event: response.output_item.done\ndata: %s\n\n", b)
				}
				flusher.Flush()
				mu.Unlock()
			}
		}
		if chunk.Usage.Present {
			lastUsage = chunk.Usage
		}
	}
	flushDone()
	if err := scanner.Err(); err != nil {
		mu.Lock()
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"error\":{\"message\":%q}}\n\n", err.Error())
		flusher.Flush()
		mu.Unlock()
	}
}

func StreamAnthropicAsResponses(w http.ResponseWriter, body io.Reader, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	responseID := "resp_" + fmt.Sprintf("%d", time.Now().UnixNano())
	createdEvent := map[string]any{
		"type": "response.created",
		"response": map[string]any{
			"id":     responseID,
			"object": "response",
			"status": "in_progress",
			"model":  model,
		},
	}
	if b, err := json.Marshal(createdEvent); err == nil {
		fmt.Fprintf(w, "event: response.created\ndata: %s\n\n", b)
		flusher.Flush()
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var mu sync.Mutex
	var usage compat.TokenUsage

	flushDone := func() {
		mu.Lock()
		defer mu.Unlock()
		completed := map[string]any{
			"type": "response.completed",
			"response": map[string]any{
				"id":     responseID,
				"object": "response",
				"status": "completed",
				"model":  model,
				"usage":  compat.ResponsesUsage(usage),
			},
		}
		if b, err := json.Marshal(completed); err == nil {
			fmt.Fprintf(w, "event: response.completed\ndata: %s\n\n", b)
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"delta"`
			Usage      compat.TokenUsage `json:"usage"`
			ContentBlk map[string]any    `json:"content_block"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				delta := map[string]any{
					"type":  "response.output_text.delta",
					"delta": event.Delta.Text,
				}
				mu.Lock()
				if b, err := json.Marshal(delta); err == nil {
					fmt.Fprintf(w, "event: response.output_text.delta\ndata: %s\n\n", b)
					flusher.Flush()
				}
				mu.Unlock()
			}
		case "message_delta":
			if event.Usage.OutputTokens > 0 {
				usage.OutputTokens = event.Usage.OutputTokens
			}
		case "message_start":
			var ms struct {
				Message struct {
					Usage compat.TokenUsage `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal([]byte(data), &ms); err == nil {
				usage.InputTokens = ms.Message.Usage.InputTokens
			}
		}
	}
	flushDone()
	if err := scanner.Err(); err != nil {
		mu.Lock()
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"error\":{\"message\":%q}}\n\n", err.Error())
		flusher.Flush()
		mu.Unlock()
	}
}

func ConvertAnthropicToResponses(model string, parsed AnthropicParsedResponse) map[string]any {
	now := time.Now().UTC()
	respID := "resp_" + fmt.Sprintf("%d", now.UnixNano())
	output := []map[string]any{}
	if parsed.Text != "" {
		output = append(output, map[string]any{
			"type":   "message",
			"id":     "msg_" + fmt.Sprintf("%d", now.UnixNano()),
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": parsed.Text, "annotations": []any{}},
			},
		})
	}
	for _, tc := range parsed.ToolCalls {
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		output = append(output, map[string]any{
			"type":      "function_call",
			"id":        tc.ID,
			"call_id":   tc.ID,
			"name":      tc.Function.Name,
			"arguments": args,
			"status":    "completed",
		})
	}
	return map[string]any{
		"id":         respID,
		"object":     "response",
		"created_at": now.Unix(),
		"status":     "completed",
		"model":      model,
		"output":     output,
		"usage":      compat.ResponsesUsage(parsed.Usage),
	}
}

func ConvertChatCompletionToResponses(model string, body []byte) (map[string]any, error) {
	var raw struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Choices []struct {
			Message struct {
				Role      string             `json:"role"`
				Content   string             `json:"content"`
				ToolCalls []compat.OAIToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.Model == "" {
		raw.Model = model
	}
	usage := compat.UsageFromJSON(raw.Usage)
	now := time.Now().UTC()
	text := ""
	var toolCalls []compat.OAIToolCall
	if len(raw.Choices) > 0 {
		text = raw.Choices[0].Message.Content
		toolCalls = raw.Choices[0].Message.ToolCalls
	}
	output := []map[string]any{}
	if text != "" {
		output = append(output, map[string]any{
			"type":   "message",
			"id":     "msg_" + fmt.Sprintf("%d", now.UnixNano()),
			"status": "completed",
			"role":   "assistant",
			"content": []map[string]any{
				{"type": "output_text", "text": text, "annotations": []any{}},
			},
		})
	}
	for _, tc := range toolCalls {
		args := tc.Function.Arguments
		if args == "" {
			args = "{}"
		}
		output = append(output, map[string]any{
			"type":      "function_call",
			"id":        tc.ID,
			"call_id":   tc.ID,
			"name":      tc.Function.Name,
			"arguments": args,
			"status":    "completed",
		})
	}
	return map[string]any{
		"id":         "resp_" + raw.ID,
		"object":     "response",
		"created_at": now.Unix(),
		"status":     "completed",
		"model":      raw.Model,
		"output":     output,
		"usage":      compat.ResponsesUsage(usage),
	}, nil
}

func WriteResponsesFromChatCompletion(w http.ResponseWriter, model string, body []byte) {
	resp, err := ConvertChatCompletionToResponses(model, body)
	if err != nil {
		WriteResponsesError(w, fmt.Errorf("convert upstream response: %w", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(resp)
	w.Write(b)
}

func ResponsesHandler(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	if r.Method != http.MethodPost {
		WriteOpenAIError(w, http.StatusMethodNotAllowed, "Method not allowed: only POST is supported on /v1/responses", "invalid_request_error", "", "method_not_allowed")
		return
	}
	rr, err := DecodeResponsesRequest(r.Body)
	if err != nil {
		WriteOpenAIError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err), "invalid_request_error", "", "invalid_json")
		return
	}
	if err := ValidateResponsesRequest(rr); err != nil {
		WriteResponsesError(w, err)
		return
	}
	or := ResponsesToChatSafe(rr)
	if err := ValidateImageSupport(or); err != nil {
		WriteOpenAIError(w, http.StatusBadRequest, err.Error(), "invalid_request_error", "", "model_not_supported")
		return
	}
	runResponsesHandler(w, r, cfg, rr, or)
}

func runResponsesHandler(w http.ResponseWriter, r *http.Request, cfg config.Config, rr compat.ResponsesRequest, or compat.OAIRequest) {
	model := rr.Model
	if models.UsesAnthropicEndpoint(model) {
		ar := ChatToAnthropic(or)
		ar.Model = mapping.ResolveToolModel("claude", model)
		resp, err := ForwardAnthropic(r.Context(), cfg, ar)
		if err != nil {
			WriteOpenAIError(w, http.StatusBadGateway, fmt.Sprintf("Upstream error: %v", err), "upstream_error", "", "upstream_failure")
			return
		}
		defer resp.Body.Close()
		CopyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		if rr.Stream {
			StreamAnthropicAsResponses(w, resp.Body, model)
		} else {
			parsed := ParseAnthropicResponse(resp.Body)
			respObj := ConvertAnthropicToResponses(model, parsed)
			b, _ := json.Marshal(respObj)
			w.Write(b)
		}
		return
	}

	body, err := json.Marshal(or)
	if err != nil {
		WriteOpenAIError(w, http.StatusInternalServerError, fmt.Sprintf("marshal: %v", err), "server_error", "", "internal")
		return
	}
	openURL := openAIUpstreamURL()
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, openURL, bytes.NewReader(body))
	if err != nil {
		WriteOpenAIError(w, http.StatusInternalServerError, fmt.Sprintf("create request: %v", err), "server_error", "", "internal")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		WriteOpenAIError(w, http.StatusBadGateway, fmt.Sprintf("Upstream error: %v", err), "upstream_error", "", "upstream_failure")
		return
	}
	defer resp.Body.Close()

	CopyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if or.Stream {
		StreamChatCompletionAsResponses(w, resp.Body, model)
	} else {
		upstreamBody, _ := io.ReadAll(resp.Body)
		WriteResponsesFromChatCompletion(w, model, upstreamBody)
	}
}
