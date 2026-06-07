package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"ocgo/internal/compat"
	"ocgo/internal/config"
	"ocgo/internal/mapping"
	"ocgo/internal/models"
)

var AnthropicURL = "https://opencode.ai/zen/go/v1/messages"

type AnthropicParsedResponse struct {
	Text      string
	ToolCalls []compat.OAIToolCall
	Usage     compat.TokenUsage
}

func RunServer(cfg config.Config) error {
	if err := os.MkdirAll(config.ConfigDir(), 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	pidPath := config.PIDFile()
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("write pid file: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		ProxyMessages(w, r, cfg)
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		ProxyChatCompletions(w, r, cfg)
	})
	mux.HandleFunc("/v1/responses", func(w http.ResponseWriter, r *http.Request) {
		ProxyResponses(w, r, cfg)
	})
	mux.HandleFunc("/v1/count_tokens", CountTokens)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	fmt.Printf("ocgo proxy listening on http://%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func CountTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"input_tokens":0}`))
}

func ProxyMessages(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var ar compat.AnthropicRequest
	if err := json.NewDecoder(r.Body).Decode(&ar); err != nil {
		http.Error(w, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}

	model := ar.Model

	if strings.HasPrefix(model, "claude") || strings.HasPrefix(model, "anthropic") {
		ar.Model = mapping.ResolveToolModel("claude", model)
		resp, err := ForwardAnthropic(r.Context(), cfg, ar)
		if err != nil {
			http.Error(w, fmt.Sprintf("forward: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		CopyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)

		if ar.Stream {
			StreamAnthropic(w, resp.Body, model)
		} else {
			WriteAnthropicResponse(w, resp.Body, model)
		}
		return
	}

	or := ConvertRequest(ar)
	or.Model = mapping.ResolveToolModel("codex", model)

	if err := ValidateImageSupport(or); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	body, err := json.Marshal(or)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal request: %v", err), http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, config.OpenAIURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("forward: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	CopyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if or.Stream {
		StreamChatCompletionsFromAnthropic(w, resp.Body, model)
	} else {
		WriteChatCompletionsResponseFromAnthropic(w, resp.Body, model)
	}
}

func ProxyChatCompletions(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusInternalServerError)
		return
	}

	body, err := PrepareChatBody(rawBody)
	if err != nil {
		http.Error(w, fmt.Sprintf("prepare body: %v", err), http.StatusBadRequest)
		return
	}

	var reqMap map[string]any
	if err := json.Unmarshal(body, &reqMap); err != nil {
		http.Error(w, fmt.Sprintf("parse request: %v", err), http.StatusBadRequest)
		return
	}

	model, _ := reqMap["model"].(string)

	if strings.HasPrefix(model, "claude") || strings.HasPrefix(model, "anthropic") {
		var or compat.OAIRequest
		if err := json.Unmarshal(body, &or); err != nil {
			http.Error(w, fmt.Sprintf("parse OAI request: %v", err), http.StatusBadRequest)
			return
		}

		ar := ChatToAnthropic(or)
		ar.Model = mapping.ResolveToolModel("claude", model)

		resp, err := ForwardAnthropic(r.Context(), cfg, ar)
		if err != nil {
			http.Error(w, fmt.Sprintf("forward: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		CopyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)

		if or.Stream {
			StreamChatCompletionsFromAnthropic(w, resp.Body, model)
		} else {
			WriteChatCompletionsResponseFromAnthropic(w, resp.Body, model)
		}
		return
	}

	model = mapping.ResolveToolModel("codex", model)
	body = bytes.Replace(body, []byte(`"`+reqMap["model"].(string)+`"`), []byte(`"`+model+`"`), 1)

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, config.OpenAIURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("forward: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	CopyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	io.Copy(w, resp.Body)
}

func ProxyResponses(w http.ResponseWriter, r *http.Request, cfg config.Config) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rr compat.ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
		http.Error(w, fmt.Sprintf("decode request: %v", err), http.StatusBadRequest)
		return
	}

	model := rr.Model
	or := ResponsesToChat(rr)

	if err := ValidateImageSupport(or); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if strings.HasPrefix(model, "claude") || strings.HasPrefix(model, "anthropic") {
		ar := ChatToAnthropic(or)
		ar.Model = mapping.ResolveToolModel("claude", model)

		resp, err := ForwardAnthropic(r.Context(), cfg, ar)
		if err != nil {
			http.Error(w, fmt.Sprintf("forward: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		CopyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)

		if rr.Stream {
			StreamResponsesFromAnthropic(w, resp.Body, model)
		} else {
			WriteResponsesResponseFromAnthropic(w, resp.Body, model)
		}
		return
	}

	or.Model = mapping.ResolveToolModel("codex", model)

	body, err := json.Marshal(or)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal request: %v", err), http.StatusInternalServerError)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, config.OpenAIURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("create request: %v", err), http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("forward: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	CopyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	if or.Stream {
		StreamResponses(w, resp.Body, model)
	} else {
		WriteResponsesResponse(w, resp.Body, model)
	}
}

func CopyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func ForwardAnthropic(ctx context.Context, cfg config.Config, ar compat.AnthropicRequest) (*http.Response, error) {
	NormalizeAnthropicRequestForUpstream(&ar)

	body, err := json.Marshal(ar)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, AnthropicURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", cfg.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	return http.DefaultClient.Do(req)
}

func NormalizeAnthropicRequestForUpstream(ar *compat.AnthropicRequest) {
	EnsureAnthropicRequestDefaults(ar)

	ar.Thinking = nil
	ar.Reasoning = nil
	ar.ReasoningEffort = nil
	ar.Effort = nil
	ar.Level = nil
	ar.Depth = nil
	ar.OutputConfig = nil

	if len(ar.System) > 0 {
		ar.System = compat.NormalizeSystem(ar.System)
	}

	for i, msg := range ar.Messages {
		if len(msg.Content) > 0 {
			var contentBlocks []json.RawMessage
			if err := json.Unmarshal(msg.Content, &contentBlocks); err != nil {
				continue
			}
			normalized := make([]json.RawMessage, 0, len(contentBlocks))
			for _, c := range contentBlocks {
				nc := compat.NormalizeContent(c)
				if nc != nil {
					normalized = append(normalized, nc)
				}
			}
			normalizedData, _ := json.Marshal(normalized)
			ar.Messages[i].Content = normalizedData
		}
	}
}

func EnsureAnthropicRequestDefaults(ar *compat.AnthropicRequest) {
	ar.Model = mapping.ResolveToolModel("claude", ar.Model)
	if ar.MaxTokens == 0 {
		ar.MaxTokens = 4096
	}
}

func PrepareChatBody(body []byte) ([]byte, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse body: %w", err)
	}

	var opts *compat.OAIStreamOptions
	if raw, ok := req["stream_options"]; ok {
		b, _ := json.Marshal(raw)
		json.Unmarshal(b, &opts)
	}
	if opts != nil && opts.IncludeUsage {
		if req["stream"] == true {
			req["stream_options"] = map[string]any{"include_usage": true}
		}
	}

	if ApplyRawChatReasoningEffort(req) {
		body, _ = json.Marshal(req)
	}

	effort := RawChatReasoningEffort(req)
	if effort != "" {
		req["reasoning_effort"] = effort
		delete(req, "reasoning_level")
		delete(req, "reasoning_depth")
		body, _ = json.Marshal(req)
	}

	if model, ok := req["model"].(string); ok {
		req["model"] = mapping.ResolveToolModel("codex", model)
		body, _ = json.Marshal(req)
	}

	if RawChatBodyHasImages(req) {
		StripRawChatImageDetails(req)
		body, _ = json.Marshal(req)
	}

	sanitized := sanitizeToolMessages(req)
	if sanitized {
		body, _ = json.Marshal(req)
	}

	return body, nil
}

func sanitizeToolMessages(req map[string]any) bool {
	messages, ok := req["messages"].([]any)
	if !ok {
		return false
	}
	changed := false
	for i, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		role, _ := m["role"].(string)
		if role != "tool" {
			continue
		}
		content, ok := m["content"]
		if !ok {
			continue
		}
		switch c := content.(type) {
		case string:
			if len(c) > 200000 {
				m["content"] = c[:200000]
				changed = true
			}
		case map[string]any:
			m["content"] = compat.TruncateToolResultContent(c)
			changed = true
		case []any:
			m["content"] = compat.TruncateToolResultContent(c)
			changed = true
		}
		messages[i] = m
	}
	if changed {
		req["messages"] = messages
	}
	return changed
}

func ApplyRawChatReasoningEffort(req map[string]any) bool {
	effort := RawChatReasoningEffort(req)
	if effort == "" {
		return false
	}
	req["reasoning_effort"] = effort
	delete(req, "reasoning_level")
	delete(req, "reasoning_depth")
	delete(req, "thinking")
	delete(req, "thinking_budget")
	return true
}

func RawChatReasoningEffort(req map[string]any) string {
	if e, ok := req["reasoning_effort"].(string); ok && e != "" {
		return e
	}
	if e, ok := req["reasoning_level"].(string); ok && e != "" {
		return e
	}
	if e, ok := req["reasoning_depth"].(string); ok && e != "" {
		return e
	}
	if e, ok := req["effort"].(string); ok && e != "" {
		return e
	}
	if _, ok := req["thinking"]; ok {
		return "medium"
	}

	messages, ok := req["messages"].([]any)
	if !ok {
		return ""
	}
	for _, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		content, ok := m["content"]
		if !ok {
			continue
		}
		switch c := content.(type) {
		case []any:
			for _, block := range c {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				typ, _ := b["type"].(string)
				if typ == "thinking" {
					return "medium"
				}
			}
		}
	}
	return ""
}

func DownstreamReasoningEffort(values ...json.RawMessage) string {
	for _, v := range values {
		if len(v) == 0 {
			continue
		}
		var blocks []map[string]any
		if err := json.Unmarshal(v, &blocks); err != nil {
			continue
		}
		for _, block := range blocks {
			typ, _ := block["type"].(string)
			if typ == "thinking" {
				return "medium"
			}
		}
	}
	return ""
}

func RawChatBodyHasImages(req map[string]any) bool {
	messages, ok := req["messages"].([]any)
	if !ok {
		return false
	}
	for _, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		content, ok := m["content"]
		if !ok {
			continue
		}
		switch c := content.(type) {
		case []any:
			for _, block := range c {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				typ, _ := b["type"].(string)
				if typ == "image_url" || typ == "image" || typ == "input_image" {
					return true
				}
				if _, ok := b["image_url"]; ok {
					return true
				}
				if _, ok := b["input_image"]; ok {
					return true
				}
			}
		case map[string]any:
			if _, ok := c["image_url"]; ok {
				return true
			}
			if _, ok := c["input_image"]; ok {
				return true
			}
		}
	}
	return false
}

func StripRawChatImageDetails(req map[string]any) bool {
	messages, ok := req["messages"].([]any)
	if !ok {
		return false
	}
	changed := false
	for _, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		content, ok := m["content"]
		if !ok {
			continue
		}
		switch c := content.(type) {
		case []any:
			for i, block := range c {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				if iu, ok := b["image_url"]; ok {
					if iuMap, ok := iu.(map[string]any); ok {
						if _, hasDetail := iuMap["detail"]; hasDetail {
							delete(iuMap, "detail")
							b["image_url"] = iuMap
							c[i] = b
							changed = true
						}
					}
				}
			}
			m["content"] = c
		case map[string]any:
			if iu, ok := c["image_url"]; ok {
				if iuMap, ok := iu.(map[string]any); ok {
					if _, hasDetail := iuMap["detail"]; hasDetail {
						delete(iuMap, "detail")
						c["image_url"] = iuMap
						m["content"] = c
						changed = true
					}
				}
			}
		}
	}
	return changed
}

func convertAnthropicToOAI(ar compat.AnthropicRequest) compat.OAIRequest {
	var messages []compat.OAIMessage
	for _, msg := range ar.Messages {
		messages = append(messages, compat.ContentToOpenAI(msg)...)
	}

	var tools []compat.OAITool
	for _, t := range ar.Tools {
		tools = append(tools, compat.OAITool{
			Type: "function",
			Function: compat.OAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  compat.ToolParametersOrDefault(t.InputSchema),
			},
		})
	}

	or := compat.OAIRequest{
		Model:          ar.Model,
		Messages:       messages,
		Stream:         ar.Stream,
		MaxTokens:      ar.MaxTokens,
		Temperature:    ar.Temperature,
		TopP:           ar.TopP,
		Tools:          tools,
		AnthropicTools: ar.Tools,
	}

	if ar.Thinking != nil {
		or.ReasoningEffort = "medium"
	}
	if effort := compat.ReasoningEffortFromRaw(ar.ReasoningEffort); effort != "" {
		or.ReasoningEffort = effort
	}
	if effort := compat.ReasoningEffortFromRaw(ar.Effort); effort != "" && or.ReasoningEffort == "" {
		or.ReasoningEffort = effort
	}

	return or
}

func convertResponsesToChat(rr compat.ResponsesRequest) compat.OAIRequest {
	messages := compat.ResponsesInputToMessages(rr.Input)

	if rr.Instructions != "" {
		insert := make([]compat.OAIMessage, 0, len(messages)+1)
		insert = append(insert, compat.OAIMessage{Role: "system", Content: rr.Instructions})
		messages = append(insert, messages...)
	}

	var tools []compat.OAITool
	for _, t := range rr.Tools {
		if t.Type == "function" || t.Type == "custom" {
			tools = append(tools, compat.OAITool{
				Type: "function",
				Function: compat.OAIFunction{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Parameters,
				},
			})
		}
	}

	or := compat.OAIRequest{
		Model:        rr.Model,
		Messages:     messages,
		Stream:       rr.Stream,
		MaxTokens:    rr.MaxTokens,
		Temperature:  rr.Temperature,
		TopP:         rr.TopP,
		Tools:        tools,
	}

	if rr.Thinking != nil {
		or.ReasoningEffort = "medium"
	}
	if effort := compat.ReasoningEffortFromRaw(rr.ReasoningEffort); effort != "" {
		or.ReasoningEffort = effort
	}

	return or
}

func convertOAIToAnthropic(or compat.OAIRequest) compat.AnthropicRequest {
	var messages []compat.AMessage
	var systemRaw json.RawMessage

	msgs := or.Messages

	if len(msgs) > 0 && msgs[0].Role == "system" {
		switch c := msgs[0].Content.(type) {
		case string:
			systemRaw = compat.MarshalJSON(c)
		default:
			b, _ := json.Marshal(c)
			var text string
			if json.Unmarshal(b, &text) == nil {
				systemRaw = compat.MarshalJSON(text)
			} else {
				systemRaw = b
			}
		}
		msgs = msgs[1:]
	}

	for _, msg := range msgs {
		switch msg.Role {
		case "tool":
			content := map[string]any{
				"type":        "tool_result",
				"tool_use_id": msg.ToolCallID,
			}
			compat.CopyAnthropicToolResultContent(content, map[string]json.RawMessage{
				"content": compat.MarshalJSON(msg.Content),
			})
			data, _ := json.Marshal(content)
			messages = append(messages, compat.AMessage{
				Role:    "user",
				Content: data,
			})

		case "assistant":
			blocks := compat.AnthropicBlocksFromOpenAIContent(msg.Content)

			if msg.ReasoningContent != "" && len(blocks) > 0 {
				if t, ok := blocks[0]["type"].(string); ok && t == "text" {
					blocks[0]["text"] = msg.ReasoningContent + "\n" + compat.OpenAIContentText(msg.Content)
				}
			}

			for _, tc := range msg.ToolCalls {
				var args json.RawMessage
				if tc.Function.Arguments != "" {
					args = json.RawMessage(tc.Function.Arguments)
				}
				blocks = append(blocks, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": args,
				})
			}

			data, _ := json.Marshal(blocks)
			messages = append(messages, compat.AMessage{
				Role:    "assistant",
				Content: data,
			})

		default:
			content := compat.OpenAIContentToAnthropic(msg.Content)
			messages = append(messages, compat.AMessage{
				Role:    msg.Role,
				Content: content,
			})
		}
	}

	tools := or.AnthropicTools
	if len(tools) == 0 && len(or.Tools) > 0 {
		for _, t := range or.Tools {
			if t.Type == "function" {
				tools = append(tools, compat.ATool{
					Type:        "custom",
					Name:        t.Function.Name,
					Description: t.Function.Description,
					InputSchema: t.Function.Parameters,
				})
			}
		}
	}

	ar := compat.AnthropicRequest{
		Model:       or.Model,
		Messages:    messages,
		Stream:      or.Stream,
		MaxTokens:   or.MaxTokens,
		Temperature: or.Temperature,
		TopP:        or.TopP,
		Tools:       tools,
		System:      systemRaw,
	}

	if or.ReasoningEffort != "" {
		ar.ReasoningEffort = compat.MarshalJSON(or.ReasoningEffort)
	}

	return ar
}

func ConvertRequest(ar compat.AnthropicRequest) compat.OAIRequest {
	or := convertAnthropicToOAI(ar)
	or.Model = mapping.ResolveToolModel("codex", ar.Model)
	return or
}

func ResponsesToChat(rr compat.ResponsesRequest) compat.OAIRequest {
	or := convertResponsesToChat(rr)
	or.Model = mapping.ResolveToolModel("codex", rr.Model)
	return or
}

func ChatToAnthropic(or compat.OAIRequest) compat.AnthropicRequest {
	ar := convertOAIToAnthropic(or)
	ar.Model = mapping.ResolveToolModel("claude", or.Model)
	return ar
}

func ValidateImageSupport(or compat.OAIRequest) error {
	if !compat.RequestHasImages(or) {
		return nil
	}
	if !models.SupportsImages(or.Model) {
		return errors.New("model " + or.Model + " does not support images")
	}
	return nil
}

func StreamAnthropic(w http.ResponseWriter, body io.Reader, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var mu sync.Mutex
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

		mu.Lock()
		w.Write([]byte(line + "\n\n"))
		flusher.Flush()
		mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		mu.Lock()
		w.Write([]byte("data: {\"type\":\"error\",\"error\":{\"message\":\"" + err.Error() + "\"}}\n\n"))
		flusher.Flush()
		mu.Unlock()
	}
}

func StreamChatCompletionsFromAnthropic(w http.ResponseWriter, body io.Reader, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var mu sync.Mutex
	var messageStart struct {
		Message struct {
			Usage struct {
				InputTokens int `json:"input_tokens"`
			} `json:"usage"`
		} `json:"message"`
	}
	var usage compat.TokenUsage

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
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			json.Unmarshal([]byte(data), &messageStart)
			if messageStart.Message.Usage.InputTokens > 0 {
				usage.InputTokens = messageStart.Message.Usage.InputTokens
			}

		case "content_block_start":
			var cbs struct {
				Index   int            `json:"index"`
				Content map[string]any `json:"content_block"`
			}
			json.Unmarshal([]byte(data), &cbs)

			mu.Lock()
			switch cbs.Content["type"].(string) {
			case "text":
				text, _ := cbs.Content["text"].(string)
				WriteChatCompletionChunk(w, model, map[string]any{
					"role":    "assistant",
					"content": text,
				}, nil)
			case "tool_use":
				id, _ := cbs.Content["id"].(string)
				name, _ := cbs.Content["name"].(string)
				tc := map[string]any{
					"id":   id,
					"type": "function",
					"function": map[string]any{
						"name":      name,
						"arguments": "",
					},
				}
				WriteChatCompletionChunk(w, model, map[string]any{
					"role":       "assistant",
					"content":    nil,
					"tool_calls": []any{tc},
				}, nil)
			}
			flusher.Flush()
			mu.Unlock()

		case "content_block_delta":
			var cbd struct {
				Index int `json:"index"`
				Delta struct {
					Type        string `json:"type"`
					Text        string `json:"text"`
					PartialJSON string `json:"partial_json"`
				} `json:"delta"`
			}
			json.Unmarshal([]byte(data), &cbd)

			mu.Lock()
			switch cbd.Delta.Type {
			case "text_delta":
				WriteChatCompletionChunk(w, model, map[string]any{
					"content": cbd.Delta.Text,
				}, nil)
			case "input_json_delta":
				WriteChatCompletionChunk(w, model, map[string]any{
					"tool_calls": []any{map[string]any{
						"index":    cbd.Index,
						"function": map[string]any{"arguments": cbd.Delta.PartialJSON},
					}},
				}, nil)
			}
			flusher.Flush()
			mu.Unlock()

		case "content_block_stop":
			mu.Lock()
			flusher.Flush()
			mu.Unlock()

		case "message_delta":
			var md struct {
				Delta struct {
					StopReason   string `json:"stop_reason"`
					StopSequence string `json:"stop_sequence"`
				} `json:"delta"`
				Usage compat.TokenUsage `json:"usage"`
			}
			json.Unmarshal([]byte(data), &md)
			if md.Usage.OutputTokens > 0 {
				usage.OutputTokens = md.Usage.OutputTokens
			}

			mu.Lock()
			var finishReason *string
			if md.Delta.StopReason != "" {
				sr := mapAnthropicStopReason(md.Delta.StopReason)
				finishReason = &sr
			}
			WriteChatCompletionChunk(w, model, map[string]any{}, finishReason)
			flusher.Flush()
			mu.Unlock()

		case "message_stop":
			mu.Lock()
			if usage.InputTokens > 0 || usage.OutputTokens > 0 {
				usageChunk, _ := json.Marshal(map[string]any{
					"choices": []any{},
					"usage":   usage,
				})
				w.Write([]byte("data: " + string(usageChunk) + "\n\n"))
			}
			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			mu.Unlock()
		}
	}

	if err := scanner.Err(); err != nil {
		mu.Lock()
		w.Write([]byte("data: {\"error\":\"" + err.Error() + "\"}\n\n"))
		flusher.Flush()
		mu.Unlock()
	}
}

func StreamResponses(w http.ResponseWriter, body io.Reader, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var mu sync.Mutex
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

		mu.Lock()
		w.Write([]byte(line + "\n\n"))
		flusher.Flush()
		mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		mu.Lock()
		w.Write([]byte("data: {\"type\":\"error\",\"error\":{\"message\":\"" + err.Error() + "\"}}\n\n"))
		flusher.Flush()
		mu.Unlock()
	}
}

func StreamResponsesFromAnthropic(w http.ResponseWriter, body io.Reader, model string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		io.Copy(w, body)
		return
	}

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var mu sync.Mutex
	var responseID string
	var responseModel string
	var usage compat.TokenUsage

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
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "message_start":
			var ms struct {
				Message struct {
					ID    string           `json:"id"`
					Model string           `json:"model"`
					Usage compat.TokenUsage `json:"usage"`
				} `json:"message"`
			}
			json.Unmarshal([]byte(data), &ms)
			responseID = ms.Message.ID
			responseModel = ms.Message.Model
			usage = ms.Message.Usage

		case "content_block_start":
			var cbs struct {
				Index   int            `json:"index"`
				Content map[string]any `json:"content_block"`
			}
			json.Unmarshal([]byte(data), &cbs)

			mu.Lock()
			if t, _ := cbs.Content["type"].(string); t == "text" {
				text, _ := cbs.Content["text"].(string)
				resp := map[string]any{
					"type":    "response.output_text.delta",
					"delta":   text,
					"item_id": responseID,
				}
				b, _ := json.Marshal(resp)
				w.Write([]byte("data: " + string(b) + "\n\n"))
				flusher.Flush()
			}
			mu.Unlock()

		case "content_block_delta":
			var cbd struct {
				Index int `json:"index"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			json.Unmarshal([]byte(data), &cbd)

			mu.Lock()
			if cbd.Delta.Type == "text_delta" {
				resp := map[string]any{
					"type":    "response.output_text.delta",
					"delta":   cbd.Delta.Text,
					"item_id": responseID,
				}
				b, _ := json.Marshal(resp)
				w.Write([]byte("data: " + string(b) + "\n\n"))
				flusher.Flush()
			}
			mu.Unlock()

		case "content_block_stop":
			mu.Lock()
			flusher.Flush()
			mu.Unlock()

		case "message_delta":
			var md struct {
				Delta struct {
					StopReason string `json:"stop_reason"`
				} `json:"delta"`
				Usage compat.TokenUsage `json:"usage"`
			}
			json.Unmarshal([]byte(data), &md)
			if md.Usage.OutputTokens > 0 {
				usage.OutputTokens = md.Usage.OutputTokens
			}

		case "message_stop":
			mu.Lock()
			resp := map[string]any{
				"type": "response.complete",
				"response": map[string]any{
					"id":    responseID,
					"model": responseModel,
					"usage": usage,
				},
			}
			b, _ := json.Marshal(resp)
			w.Write([]byte("data: " + string(b) + "\n\n"))
			w.Write([]byte("data: [DONE]\n\n"))
			flusher.Flush()
			mu.Unlock()
		}
	}

	if err := scanner.Err(); err != nil {
		mu.Lock()
		w.Write([]byte("data: {\"type\":\"error\",\"error\":{\"message\":\"" + err.Error() + "\"}}\n\n"))
		flusher.Flush()
		mu.Unlock()
	}
}

func WriteAnthropicResponse(w http.ResponseWriter, body io.Reader, model string) {
	parsed := ParseAnthropicResponse(body)
	if parsed.Text == "" && len(parsed.ToolCalls) == 0 {
		w.Write([]byte("{}"))
		return
	}

	content := make([]map[string]any, 0)
	if parsed.Text != "" {
		content = append(content, map[string]any{
			"type": "text",
			"text": parsed.Text,
		})
	}
	for _, tc := range parsed.ToolCalls {
		args := json.RawMessage(tc.Function.Arguments)
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": args,
		})
	}

	resp := map[string]any{
		"content": content,
		"role":    "assistant",
		"model":   model,
		"usage":   parsed.Usage,
	}
	if len(parsed.ToolCalls) > 0 {
		resp["stop_reason"] = "tool_use"
	} else {
		resp["stop_reason"] = "end_turn"
	}

	b, _ := json.Marshal(resp)
	w.Write(b)
}

func WriteChatCompletionsResponseFromAnthropic(w http.ResponseWriter, body io.Reader, model string) {
	parsed := ParseAnthropicResponse(body)

	resp := map[string]any{
		"id":     "chatcmpl-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"object": "chat.completion",
		"model":  model,
		"choices": []map[string]any{
			{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": parsed.Text,
				},
				"finish_reason": "stop",
			},
		},
		"usage": parsed.Usage,
	}

	if len(parsed.ToolCalls) > 0 {
		msg := resp["choices"].([]map[string]any)[0]["message"].(map[string]any)
		msg["tool_calls"] = parsed.ToolCalls
		resp["choices"].([]map[string]any)[0]["message"] = msg
		if parsed.Text == "" {
			resp["choices"].([]map[string]any)[0]["finish_reason"] = "tool_calls"
		}
	}

	b, _ := json.Marshal(resp)
	w.Write(b)
}

func WriteResponsesResponse(w http.ResponseWriter, body io.Reader, model string) {
	respBody, err := io.ReadAll(body)
	if err != nil {
		http.Error(w.(http.ResponseWriter), fmt.Sprintf("read response: %v", err), http.StatusInternalServerError)
		return
	}
	w.Write(respBody)
}

func WriteResponsesResponseFromAnthropic(w http.ResponseWriter, body io.Reader, model string) {
	parsed := ParseAnthropicResponse(body)

	output := make([]map[string]any, 0)
	if parsed.Text != "" {
		output = append(output, map[string]any{
			"type": "message",
			"role": "assistant",
			"content": []map[string]any{
				{
					"type": "output_text",
					"text": parsed.Text,
				},
			},
		})
	}
	for _, tc := range parsed.ToolCalls {
		args := json.RawMessage(tc.Function.Arguments)
		output = append(output, map[string]any{
			"type":      "function_call",
			"id":        tc.ID,
			"name":      tc.Function.Name,
			"arguments": args,
		})
	}

	resp := map[string]any{
		"id":     "resp-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"model":  model,
		"object": "response",
		"output": output,
		"usage":  parsed.Usage,
	}

	b, _ := json.Marshal(resp)
	w.Write(b)
}

func WriteChatCompletionChunk(w io.Writer, model string, delta map[string]any, finishReason *string) {
	chunk := map[string]any{
		"id":     "chatcmpl-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		"object": "chat.completion.chunk",
		"model":  model,
		"choices": []map[string]any{
			{
				"index": 0,
				"delta": delta,
			},
		},
	}

	if finishReason != nil {
		chunk["choices"].([]map[string]any)[0]["finish_reason"] = *finishReason
	}

	b, _ := json.Marshal(chunk)
	w.Write([]byte("data: " + string(b) + "\n\n"))
}

func mapAnthropicStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "stop_sequence":
		return "stop"
	default:
		return reason
	}
}

func ParseAnthropicResponse(body io.Reader) AnthropicParsedResponse {
	var result AnthropicParsedResponse

	var fullBody []byte
	var err error
	if buf, ok := body.(*bytes.Buffer); ok {
		fullBody = buf.Bytes()
	} else if rb, ok := body.(io.ReadCloser); ok {
		fullBody, err = io.ReadAll(rb)
	} else {
		fullBody, err = io.ReadAll(body)
	}
	if err != nil {
		return result
	}

	var msg struct {
		Content    []json.RawMessage `json:"content"`
		Model      string            `json:"model"`
		Usage      compat.TokenUsage `json:"usage"`
		Role       string            `json:"role"`
		StopReason string            `json:"stop_reason"`
	}

	if err := json.Unmarshal(fullBody, &msg); err != nil {
		return result
	}

	result.Usage = msg.Usage

	for _, raw := range msg.Content {
		var block struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			ID    string          `json:"id"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal(raw, &block); err != nil {
			continue
		}

		switch block.Type {
		case "text":
			result.Text += block.Text
		case "tool_use":
			args := string(block.Input)
			if args == "" {
				args = "{}"
			}
			result.ToolCalls = append(result.ToolCalls, compat.OAIToolCall{
				ID:   block.ID,
				Type: "function",
				Function: compat.OAICallFunction{
					Name:      block.Name,
					Arguments: args,
				},
			})
		}
	}

	return result
}
