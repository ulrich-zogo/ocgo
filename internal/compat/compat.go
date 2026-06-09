package compat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

type AnthropicRequest struct {
	Model           string          `json:"model"`
	MaxTokens       int             `json:"max_tokens"`
	System          json.RawMessage `json:"system,omitempty"`
	Messages        []AMessage      `json:"messages"`
	Stream          bool            `json:"stream,omitempty"`
	Tools           []ATool         `json:"tools,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	Thinking        json.RawMessage `json:"thinking,omitempty"`
	Reasoning       json.RawMessage `json:"reasoning,omitempty"`
	ReasoningEffort json.RawMessage `json:"reasoning_effort,omitempty"`
	Effort          json.RawMessage `json:"effort,omitempty"`
	Level           json.RawMessage `json:"level,omitempty"`
	Depth           json.RawMessage `json:"depth,omitempty"`
	OutputConfig    json.RawMessage `json:"output_config,omitempty"`
}

type AMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type ATool struct {
	Type           string          `json:"type,omitempty"`
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	InputSchema    json.RawMessage `json:"input_schema,omitempty"`
	MaxUses        int             `json:"max_uses,omitempty"`
	AllowedDomains []string        `json:"allowed_domains,omitempty"`
	BlockedDomains []string        `json:"blocked_domains,omitempty"`
	UserLocation   json.RawMessage `json:"user_location,omitempty"`
}

type OAIRequest struct {
	Model           string            `json:"model"`
	Messages        []OAIMessage      `json:"messages"`
	Stream          bool              `json:"stream,omitempty"`
	StreamOptions   *OAIStreamOptions `json:"stream_options,omitempty"`
	MaxTokens       int               `json:"max_tokens,omitempty"`
	Temperature     *float64          `json:"temperature,omitempty"`
	TopP            *float64          `json:"top_p,omitempty"`
	Tools           []OAITool         `json:"tools,omitempty"`
	ReasoningEffort string            `json:"reasoning_effort,omitempty"`
	AnthropicTools  []ATool           `json:"-"`
}

type OAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type ResponsesRequest struct {
	Model           string          `json:"model"`
	Input           json.RawMessage `json:"input"`
	Instructions    string          `json:"instructions,omitempty"`
	Stream          bool            `json:"stream,omitempty"`
	MaxTokens       int             `json:"max_output_tokens,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	TopP            *float64        `json:"top_p,omitempty"`
	Tools           []ResponseTool  `json:"tools,omitempty"`
	Thinking        json.RawMessage `json:"thinking,omitempty"`
	Reasoning       json.RawMessage `json:"reasoning,omitempty"`
	ReasoningEffort json.RawMessage `json:"reasoning_effort,omitempty"`
	Effort          json.RawMessage `json:"effort,omitempty"`
	Level           json.RawMessage `json:"level,omitempty"`
	Depth           json.RawMessage `json:"depth,omitempty"`
	OutputConfig    json.RawMessage `json:"output_config,omitempty"`
}

type ResponseTool struct {
	Type              string          `json:"type"`
	Name              string          `json:"name,omitempty"`
	Description       string          `json:"description,omitempty"`
	Parameters        json.RawMessage `json:"parameters,omitempty"`
	SearchContextSize string          `json:"search_context_size,omitempty"`
	UserLocation      json.RawMessage `json:"user_location,omitempty"`
}

type OAIMessage struct {
	Role             string        `json:"role"`
	Content          any           `json:"content,omitempty"`
	ToolCalls        []OAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
}

type OAIContentPart struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *OAIImageURL `json:"image_url,omitempty"`
}

type OAIImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type OAITool struct {
	Type     string      `json:"type"`
	Function OAIFunction `json:"function"`
}

type OAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type OAIToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function OAICallFunction `json:"function"`
}

type OAICallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type TokenUsage struct {
	InputTokens       int
	OutputTokens      int
	TotalTokens       int
	CachedInputTokens int
	Present           bool
}

type StreamedResponseToolCall struct {
	OutputIndex int
	Call        OAIToolCall
}

type OpenAIStreamToolCall struct {
	Index     int    `json:"index"`
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type OpenAIStreamChunk struct {
	Content          string
	ReasoningContent string
	ToolCalls        []OpenAIStreamToolCall
	Usage            TokenUsage
}

type RawChatMessageInfo struct {
	Role        string
	ToolCallID  string
	ToolCallIDs []string
}

var ReasoningContentCache = struct {
	sync.Mutex
	ByCallID map[string]string
}{ByCallID: map[string]string{}}

const MaxAnthropicToolResultContentChars = 120000
const UnavailableToolResultContent = "Tool result unavailable."

func MarshalJSON(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func SystemText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return BlockText(raw)
}

func BlockText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, block := range blocks {
		if t, ok := block["type"].(string); ok && t == "text" {
			if text, ok := block["text"].(string); ok {
				sb.WriteString(text)
			}
		}
	}
	return sb.String()
}

func OpenAIContentText(content any) string {
	if content == nil {
		return ""
	}
	switch v := content.(type) {
	case string:
		return v
	case []OAIContentPart:
		var sb strings.Builder
		for _, part := range v {
			if part.Type == "text" {
				sb.WriteString(part.Text)
			}
		}
		return sb.String()
	case []any:
		var sb strings.Builder
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				if t, ok := m["type"].(string); ok && t == "text" {
					if text, ok := m["text"].(string); ok {
						sb.WriteString(text)
					}
				}
			}
		}
		return sb.String()
	}
	return ""
}

func OpenAIContentValue(text string, parts []OAIContentPart, hasImage bool) any {
	if hasImage {
		if len(parts) == 0 {
			return []OAIContentPart{{Type: "text", Text: text}}
		}
		if text != "" && (len(parts) == 0 || parts[0].Type != "text" || parts[0].Text != text) {
			result := make([]OAIContentPart, 0, len(parts)+1)
			result = append(result, OAIContentPart{Type: "text", Text: text})
			result = append(result, parts...)
			return result
		}
		return parts
	}
	if len(parts) == 1 && parts[0].Type == "text" {
		return parts[0].Text
	}
	if len(parts) > 1 {
		return parts
	}
	return text
}

func ContentHasImage(content any) bool {
	if content == nil {
		return false
	}
	switch v := content.(type) {
	case string:
		return false
	case []OAIContentPart:
		for _, p := range v {
			if p.Type == "image_url" || p.Type == "input_image" {
				return true
			}
		}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				t, _ := m["type"].(string)
				if t == "image_url" || t == "input_image" {
					return true
				}
			}
		}
	}
	return false
}

func RequestHasImages(or OAIRequest) bool {
	for _, msg := range or.Messages {
		if ContentHasImage(msg.Content) {
			return true
		}
	}
	return false
}

func ValidateImageSupport(or OAIRequest, supportsImages func(string) bool) error {
	if !RequestHasImages(or) {
		return nil
	}
	if supportsImages != nil && !supportsImages(or.Model) {
		return fmt.Errorf("model %s does not support images", or.Model)
	}
	return nil
}

func AnthropicImageURL(b map[string]json.RawMessage) *OAIImageURL {
	src, ok := b["source"]
	if !ok {
		return nil
	}
	var source struct {
		Type      string `json:"type"`
		MediaType string `json:"media_type"`
		Data      string `json:"data"`
		URL       string `json:"url"`
	}
	if err := json.Unmarshal(src, &source); err != nil {
		return nil
	}
	switch source.Type {
	case "base64":
		return &OAIImageURL{
			URL:    "data:" + source.MediaType + ";base64," + source.Data,
			Detail: "auto",
		}
	case "url":
		return &OAIImageURL{
			URL:    source.URL,
			Detail: "auto",
		}
	}
	return nil
}

func ResponsesImageURL(p map[string]json.RawMessage) *OAIImageURL {
	raw, ok := p["image_url"]
	if !ok {
		return nil
	}
	var urlStr string
	if json.Unmarshal(raw, &urlStr) == nil {
		return &OAIImageURL{URL: urlStr, Detail: "auto"}
	}
	var obj struct {
		URL    string `json:"url"`
		Detail string `json:"detail"`
	}
	if json.Unmarshal(raw, &obj) == nil {
		return &OAIImageURL{URL: obj.URL, Detail: obj.Detail}
	}
	return nil
}

func ResponsesContent(raw json.RawMessage) any {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return string(raw)
	}
	var parts []OAIContentPart
	for _, item := range items {
		typ, _ := item["type"].(string)
		switch typ {
		case "input_text":
			text, _ := item["text"].(string)
			parts = append(parts, OAIContentPart{Type: "text", Text: text})
		case "input_image":
			if imgStr, ok := item["image_url"].(string); ok {
				parts = append(parts, OAIContentPart{Type: "image_url", ImageURL: &OAIImageURL{URL: imgStr, Detail: "auto"}})
			} else if imgMap, ok := item["image_url"].(map[string]any); ok {
				url, _ := imgMap["url"].(string)
				detail, _ := imgMap["detail"].(string)
				parts = append(parts, OAIContentPart{Type: "image_url", ImageURL: &OAIImageURL{URL: url, Detail: detail}})
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 && parts[0].Type == "text" {
		return parts[0].Text
	}
	return parts
}

func ResponsesContentText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return ""
	}
	var sb strings.Builder
	for _, item := range items {
		typ, _ := item["type"].(string)
		if typ == "input_text" {
			text, _ := item["text"].(string)
			sb.WriteString(text)
		}
	}
	return sb.String()
}

func ContentToOpenAI(m AMessage) []OAIMessage {
	raw := m.Content
	if len(raw) == 0 {
		return []OAIMessage{{Role: m.Role, Content: ""}}
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return []OAIMessage{{Role: m.Role, Content: text}}
	}
	var blocks []map[string]any
	if err := json.Unmarshal(raw, &blocks); err != nil {
		return []OAIMessage{{Role: m.Role, Content: string(raw)}}
	}
	var contentStr string
	var contentParts []OAIContentPart
	var toolCalls []OAIToolCall
	var result []OAIMessage
	hasImage := false

	flushAssistant := func() {
		if contentStr == "" && len(contentParts) == 0 && len(toolCalls) == 0 {
			return
		}
		var c any
		if hasImage {
			if contentStr != "" {
				parts := make([]OAIContentPart, 0, 1+len(contentParts))
				parts = append(parts, OAIContentPart{Type: "text", Text: contentStr})
				parts = append(parts, contentParts...)
				c = parts
			} else {
				c = contentParts
			}
		} else if len(contentParts) > 0 {
			c = contentParts
		} else {
			c = contentStr
		}
		result = append(result, OAIMessage{
			Role:      m.Role,
			Content:   c,
			ToolCalls: toolCalls,
		})
		contentStr = ""
		contentParts = nil
		toolCalls = nil
		hasImage = false
	}

	for _, block := range blocks {
		typ, _ := block["type"].(string)
		switch typ {
		case "text":
			t, _ := block["text"].(string)
			if hasImage || len(contentParts) > 0 {
				contentParts = append(contentParts, OAIContentPart{Type: "text", Text: t})
			} else {
				contentStr += t
			}
		case "image":
			img := AnthropicImageURLFromBlock(block)
			if img != nil {
				hasImage = true
				contentParts = append(contentParts, OAIContentPart{Type: "image_url", ImageURL: img})
			}
		case "tool_use":
			id, _ := block["id"].(string)
			name, _ := block["name"].(string)
			var args string
			if input, ok := block["input"]; ok {
				a, err := json.Marshal(input)
				if err == nil {
					args = string(a)
				}
			}
			toolCalls = append(toolCalls, OAIToolCall{
				ID:   id,
				Type: "function",
				Function: OAICallFunction{
					Name:      name,
					Arguments: args,
				},
			})
		case "tool_result":
			flushAssistant()
			toolUseID, _ := block["tool_use_id"].(string)
			tc := extractToolResultContentAny(block["content"])
			if tc == nil {
				tc = ""
			}
			result = append(result, OAIMessage{
				Role:       "tool",
				ToolCallID: toolUseID,
				Content:    tc,
			})
		}
	}
	flushAssistant()
	return result
}

func AnthropicImageURLFromBlock(block map[string]any) *OAIImageURL {
	src, ok := block["source"].(map[string]any)
	if !ok {
		return nil
	}
	st, _ := src["type"].(string)
	switch st {
	case "base64":
		mt, _ := src["media_type"].(string)
		data, _ := src["data"].(string)
		return &OAIImageURL{URL: "data:" + mt + ";base64," + data, Detail: "auto"}
	case "url":
		u, _ := src["url"].(string)
		return &OAIImageURL{URL: u, Detail: "auto"}
	}
	return nil
}

func extractToolResultContentAny(v any) any {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []any:
		var sb strings.Builder
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				if t, _ := m["type"].(string); t == "text" {
					if text, ok := m["text"].(string); ok {
						sb.WriteString(text)
					}
				}
			}
		}
		return sb.String()
	case map[string]any:
		if text, ok := val["text"].(string); ok {
			return text
		}
		b, _ := json.Marshal(val)
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

func OpenAIContentToAnthropic(content any) json.RawMessage {
	blocks := AnthropicBlocksFromOpenAIContent(content)
	if len(blocks) == 0 {
		return nil
	}
	if len(blocks) == 1 {
		if t, ok := blocks[0]["type"].(string); ok && t == "text" {
			if text, ok := blocks[0]["text"].(string); ok {
				return MarshalJSON(text)
			}
		}
	}
	return MarshalJSON(blocks)
}

func AnthropicBlocksFromOpenAIContent(content any) []map[string]any {
	var blocks []map[string]any
	switch v := content.(type) {
	case string:
		if v != "" {
			blocks = append(blocks, map[string]any{"type": "text", "text": v})
		}
	case []OAIContentPart:
		for _, part := range v {
			blocks = AppendAnthropicPart(blocks, part.Type, part.Text, part.ImageURL)
		}
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				t, _ := m["type"].(string)
				switch t {
				case "text":
					text, _ := m["text"].(string)
					blocks = append(blocks, map[string]any{"type": "text", "text": text})
				case "image_url":
					img := ImageURLFromAny(m, m["image_url"])
					if img != nil {
						blocks = append(blocks, map[string]any{
							"type":   "image",
							"source": AnthropicImageSource(img.URL),
						})
					}
				}
			}
		}
	}
	return blocks
}

func AppendAnthropicPart(out []map[string]any, typ, text string, image *OAIImageURL) []map[string]any {
	switch typ {
	case "text":
		out = append(out, map[string]any{"type": "text", "text": text})
	case "image_url", "input_image":
		if image != nil {
			out = append(out, map[string]any{
				"type":   "image",
				"source": AnthropicImageSource(image.URL),
			})
		}
	}
	return out
}

func ImageURLFromAny(imageValue, urlValue any) *OAIImageURL {
	switch v := urlValue.(type) {
	case string:
		return &OAIImageURL{URL: v, Detail: "auto"}
	case map[string]any:
		url, _ := v["url"].(string)
		detail, _ := v["detail"].(string)
		if detail == "" {
			detail = "auto"
		}
		return &OAIImageURL{URL: url, Detail: detail}
	}
	if m, ok := imageValue.(map[string]any); ok {
		url, _ := m["url"].(string)
		detail, _ := m["detail"].(string)
		if detail == "" {
			detail = "auto"
		}
		return &OAIImageURL{URL: url, Detail: detail}
	}
	return nil
}

func AnthropicImageSource(url string) map[string]any {
	if strings.HasPrefix(url, "data:") {
		rest := url[5:]
		commaIdx := strings.Index(rest, ",")
		if commaIdx < 0 {
			return map[string]any{"type": "url", "url": url}
		}
		mediaInfo := rest[:commaIdx]
		data := rest[commaIdx+1:]
		var mediaType string
		if semiIdx := strings.Index(mediaInfo, ";"); semiIdx >= 0 {
			mediaType = mediaInfo[:semiIdx]
		} else {
			mediaType = mediaInfo
		}
		return map[string]any{
			"type":       "base64",
			"media_type": mediaType,
			"data":       data,
		}
	}
	return map[string]any{
		"type": "url",
		"url":  url,
	}
}

func ToolParametersOrDefault(raw json.RawMessage) json.RawMessage {
	if len(raw) > 0 {
		return raw
	}
	return json.RawMessage("{}")
}

func ResponseBuiltinToolToAnthropic(t ResponseTool) (ATool, bool) {
	switch t.Type {
	case "web_search":
		at := ATool{
			Type: "web_search_preview",
			Name: "web_search",
		}
		if t.SearchContextSize != "" {
			at.Description = t.SearchContextSize
		}
		if t.UserLocation != nil {
			at.UserLocation = t.UserLocation
		}
		return at, true
	case "web_extractor":
		return ATool{
			Type: "web_extractor",
			Name: "web_extractor",
		}, true
	case "code_interpreter":
		return ATool{
			Type: "code_interpreter",
			Name: "code_interpreter",
		}, true
	case "file_search":
		return ATool{
			Type: "file_search",
			Name: "file_search",
		}, true
	case "computer_use":
		return ATool{
			Type: "computer_use",
			Name: "computer_use",
		}, true
	case "text_editor":
		return ATool{
			Type: "text_editor",
			Name: "text_editor",
		}, true
	default:
		return ATool{}, false
	}
}

func AppendUniqueAnthropicTool(tools []ATool, tool ATool) []ATool {
	for _, t := range tools {
		if t.Name == tool.Name {
			return tools
		}
	}
	return append(tools, tool)
}

func ToolCallIDOrder(calls []OAIToolCall) []string {
	ids := make([]string, len(calls))
	for i, call := range calls {
		ids[i] = call.ID
	}
	return ids
}

func ContainsString(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

func SanitizeOAIToolMessages(messages []OAIMessage) []OAIMessage {
	result := make([]OAIMessage, 0, len(messages))
	i := 0
	for i < len(messages) {
		msg := messages[i]
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			ids := ToolCallIDOrder(msg.ToolCalls)
			result = append(result, msg)
			i++
			toolMsgs := make([]*OAIMessage, len(ids))
			for i < len(messages) && messages[i].Role == "tool" {
				matched := false
				for j, id := range ids {
					if messages[i].ToolCallID == id {
						toolMsgs[j] = &messages[i]
						matched = true
						break
					}
				}
				if !matched {
					result = append(result, messages[i])
				}
				i++
			}
			for _, tm := range toolMsgs {
				if tm != nil {
					result = append(result, *tm)
				}
			}
		} else {
			result = append(result, msg)
			i++
		}
	}
	return result
}

func SanitizeRawChatToolMessages(body []byte) []byte {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return body
	}
	msgsRaw, ok := raw["messages"]
	if !ok {
		return body
	}
	var messages []json.RawMessage
	if err := json.Unmarshal(msgsRaw, &messages); err != nil {
		return body
	}
	sanitized, changed := SanitizeRawChatMessages(messages)
	if !changed {
		return body
	}
	sanitizedRaw, err := json.Marshal(sanitized)
	if err != nil {
		return body
	}
	raw["messages"] = sanitizedRaw
	result, err := json.Marshal(raw)
	if err != nil {
		return body
	}
	return result
}

func SanitizeRawChatMessages(messages []json.RawMessage) ([]json.RawMessage, bool) {
	result := make([]json.RawMessage, len(messages))
	copy(result, messages)
	changed := false
	i := 0
	for i < len(result) {
		info := ParseRawChatMessage(result[i])
		if info.Role == "assistant" && len(info.ToolCallIDs) > 0 {
			expectedIDs := info.ToolCallIDs
			start := i + 1
			end := start
			for end < len(result) {
				tInfo := ParseRawChatMessage(result[end])
				if tInfo.Role != "tool" {
					break
				}
				end++
			}
			count := end - start
			if count > 0 {
				toolMsgs := make([]json.RawMessage, count)
				toolIDs := make([]string, count)
				for k := 0; k < count; k++ {
					toolMsgs[k] = result[start+k]
					tInfo := ParseRawChatMessage(toolMsgs[k])
					toolIDs[k] = tInfo.ToolCallID
				}
				reordered := make([]json.RawMessage, count)
				used := make([]bool, count)
				nextIdx := 0
				for _, id := range expectedIDs {
					for l := 0; l < count; l++ {
						if !used[l] && toolIDs[l] == id {
							reordered[nextIdx] = toolMsgs[l]
							used[l] = true
							nextIdx++
							break
						}
					}
				}
				for l := 0; l < count; l++ {
					if !used[l] {
						reordered[nextIdx] = toolMsgs[l]
						nextIdx++
					}
				}
				for k := 0; k < count; k++ {
					if string(reordered[k]) != string(result[start+k]) {
						changed = true
					}
					result[start+k] = reordered[k]
				}
			}
			i = end
		} else {
			i++
		}
	}
	return result, changed
}

func ParseRawChatMessage(raw json.RawMessage) RawChatMessageInfo {
	var info RawChatMessageInfo
	var msg struct {
		Role       string `json:"role"`
		ToolCallID string `json:"tool_call_id"`
		ToolCalls  []struct {
			ID string `json:"id"`
		} `json:"tool_calls"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return info
	}
	info.Role = msg.Role
	info.ToolCallID = msg.ToolCallID
	for _, tc := range msg.ToolCalls {
		info.ToolCallIDs = append(info.ToolCallIDs, tc.ID)
	}
	return info
}

func RawToolPlaceholderMessage(callID string) json.RawMessage {
	data, _ := json.Marshal(map[string]string{
		"role":         "tool",
		"tool_call_id": callID,
		"content":      UnavailableToolResultContent,
	})
	return data
}

func CachedReasoningContent(calls []OAIToolCall) string {
	ReasoningContentCache.Lock()
	defer ReasoningContentCache.Unlock()
	var sb strings.Builder
	for _, call := range calls {
		if content, ok := ReasoningContentCache.ByCallID[call.ID]; ok {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(content)
		}
	}
	return sb.String()
}

func CacheReasoningContent(calls []OAIToolCall, reasoning string) {
	ReasoningContentCache.Lock()
	defer ReasoningContentCache.Unlock()
	for _, call := range calls {
		ReasoningContentCache.ByCallID[call.ID] = reasoning
	}
}

func AssistantToolCallsMessage(calls []OAIToolCall) OAIMessage {
	return OAIMessage{
		Role:      "assistant",
		ToolCalls: calls,
	}
}

func NormalizeReasoningEffort(effort string) string {
	switch strings.ToLower(effort) {
	case "0", "minimal", "min", "none", "off", "disabled", "false":
		return "minimal"
	case "1", "low", "light":
		return "low"
	case "2", "medium", "med", "normal", "default":
		return "medium"
	case "3", "4", "high", "xhigh", "max", "maximum", "deep", "true", "enabled":
		return "high"
	default:
		if effort == "" {
			return ""
		}
		return strings.ToLower(effort)
	}
}

func ReasoningEffortFromRaw(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var s string
	if json.Unmarshal(raw, &s) == nil {
		return NormalizeReasoningEffort(s)
	}

	var n float64
	if json.Unmarshal(raw, &n) == nil {
		return NormalizeReasoningEffort(FormatReasoningNumber(n))
	}

	var obj map[string]any
	if json.Unmarshal(raw, &obj) != nil {
		return ""
	}

	for _, key := range []string{"reasoning_effort", "effort", "level", "depth"} {
		if v, ok := obj[key]; ok {
			if r := ReasoningEffortFromAny(v); r != "" {
				return r
			}
		}
	}

	if t, ok := obj["type"].(string); ok {
		switch strings.ToLower(t) {
		case "enabled":
			return "high"
		case "disabled":
			return "minimal"
		}
	}

	for _, nest := range []string{"reasoning", "thinking", "output_config"} {
		if v, ok := obj[nest]; ok {
			if r := ReasoningEffortFromAny(v); r != "" {
				return r
			}
		}
	}

	return ""
}

func ReasoningEffortFromAny(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return NormalizeReasoningEffort(s)
	}
	if n, ok := v.(float64); ok {
		return NormalizeReasoningEffort(FormatReasoningNumber(n))
	}
	if raw, ok := v.(json.RawMessage); ok {
		return ReasoningEffortFromRaw(raw)
	}
	if obj, ok := v.(map[string]any); ok {
		for _, key := range []string{"reasoning_effort", "effort", "level", "depth"} {
			if val, ok := obj[key]; ok {
				if r := ReasoningEffortFromAny(val); r != "" {
					return r
				}
			}
		}
		if t, ok := obj["type"].(string); ok {
			switch strings.ToLower(t) {
			case "enabled":
				return "high"
			case "disabled":
				return "minimal"
			}
		}
		for _, nest := range []string{"reasoning", "thinking", "output_config"} {
			if val, ok := obj[nest]; ok {
				if r := ReasoningEffortFromAny(val); r != "" {
					return r
				}
			}
		}
	}
	return ""
}

func FormatReasoningNumber(n float64) string {
	if n == float64(int64(n)) {
		return fmt.Sprintf("%.0f", n)
	}
	return fmt.Sprintf("%g", n)
}

func UsageFromJSON(raw json.RawMessage) TokenUsage {
	if len(raw) == 0 {
		return TokenUsage{}
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return TokenUsage{}
	}
	return UsageFromFields(fields)
}

func UsageFromAnyMap(v any) TokenUsage {
	if m, ok := v.(map[string]any); ok {
		return UsageFromFields(m)
	}
	return TokenUsage{}
}

func MergeUsage(a, b TokenUsage) TokenUsage {
	if !a.Present {
		return b
	}
	if !b.Present {
		return a
	}
	return TokenUsage{
		InputTokens:       a.InputTokens + b.InputTokens,
		OutputTokens:      a.OutputTokens + b.OutputTokens,
		TotalTokens:       a.TotalTokens + b.TotalTokens,
		CachedInputTokens: a.CachedInputTokens + b.CachedInputTokens,
		Present:           true,
	}
}

func UsageFromFields(fields map[string]any) TokenUsage {
	u := TokenUsage{}
	u.InputTokens = IntField(fields, "input_tokens")
	if u.InputTokens == 0 {
		u.InputTokens = IntField(fields, "prompt_tokens")
	}
	u.OutputTokens = IntField(fields, "output_tokens")
	if u.OutputTokens == 0 {
		u.OutputTokens = IntField(fields, "completion_tokens")
	}
	u.TotalTokens = IntField(fields, "total_tokens")
	u.CachedInputTokens = CachedTokens(fields)
	u.Present = u.InputTokens > 0 || u.OutputTokens > 0 || u.TotalTokens > 0
	return u
}

func IntField(fields map[string]any, name string) int {
	if v, ok := fields[name]; ok {
		return IntFromAny(v)
	}
	return 0
}

func IntFromAny(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i)
		}
	}
	return 0
}

func CachedTokens(fields map[string]any) int {
	if v := IntField(fields, "cache_creation_input_tokens"); v > 0 {
		return v
	}
	if v := IntField(fields, "cache_read_input_tokens"); v > 0 {
		return v
	}
	if v := IntField(fields, "cached_tokens"); v > 0 {
		return v
	}
	return 0
}

func AnthropicUsage(u TokenUsage) map[string]int {
	m := map[string]int{
		"input_tokens":  u.InputTokens,
		"output_tokens": u.OutputTokens,
	}
	if u.CachedInputTokens > 0 {
		m["cache_read_input_tokens"] = u.CachedInputTokens
	}
	return m
}

func AnthropicDeltaUsage(u TokenUsage) map[string]int {
	return map[string]int{
		"output_tokens": u.OutputTokens,
	}
}

func ResponsesUsage(u TokenUsage) map[string]any {
	return map[string]any{
		"input_tokens":  u.InputTokens,
		"output_tokens": u.OutputTokens,
		"total_tokens":  u.TotalTokens,
	}
}

func OpenAIUsage(u TokenUsage) map[string]any {
	m := map[string]any{
		"prompt_tokens":     u.InputTokens,
		"completion_tokens": u.OutputTokens,
		"total_tokens":      u.TotalTokens,
	}
	if u.CachedInputTokens > 0 {
		m["prompt_tokens_details"] = map[string]any{
			"cached_tokens": u.CachedInputTokens,
		}
	}
	return m
}

func ResponsesInputToMessages(raw json.RawMessage) []OAIMessage {
	if len(raw) == 0 {
		return nil
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return []OAIMessage{{Role: "user", Content: s}}
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	var messages []OAIMessage
	var pendingToolCalls []OAIToolCall
	var pendingContent []OAIContentPart
	pendingHasImage := false

	flushPending := func() {
		if len(pendingToolCalls) > 0 {
			var c any
			if pendingHasImage {
				c = pendingContent
			} else if len(pendingContent) == 1 && pendingContent[0].Type == "text" {
				c = pendingContent[0].Text
			} else if len(pendingContent) > 0 {
				c = pendingContent
			}
			messages = append(messages, OAIMessage{
				Role:      "assistant",
				Content:   c,
				ToolCalls: pendingToolCalls,
			})
		} else if pendingHasImage {
			messages = append(messages, OAIMessage{
				Role:    "user",
				Content: pendingContent,
			})
		} else if len(pendingContent) > 0 {
			var text string
			for _, p := range pendingContent {
				text += p.Text
			}
			messages = append(messages, OAIMessage{
				Role:    "user",
				Content: text,
			})
		}
		pendingToolCalls = nil
		pendingContent = nil
		pendingHasImage = false
	}

	for _, item := range items {
		typ, _ := item["type"].(string)
		switch typ {
		case "message":
			flushPending()
			role, _ := item["role"].(string)
			contentRaw, _ := json.Marshal(item["content"])
			content := ResponsesContent(contentRaw)
			messages = append(messages, OAIMessage{Role: role, Content: content})
		case "tool_call":
			id, _ := item["id"].(string)
			fn, _ := item["function"].(map[string]any)
			name, _ := fn["name"].(string)
			args, _ := fn["arguments"].(string)
			pendingToolCalls = append(pendingToolCalls, OAIToolCall{
				ID:   id,
				Type: "function",
				Function: OAICallFunction{
					Name:      name,
					Arguments: args,
				},
			})
		case "function_call":
			flushPending()
			callID, _ := item["call_id"].(string)
			if callID == "" {
				callID, _ = item["id"].(string)
			}
			name, _ := item["name"].(string)
			args, _ := item["arguments"].(string)
			messages = append(messages, OAIMessage{
				Role: "assistant",
				ToolCalls: []OAIToolCall{{
					ID:   callID,
					Type: "function",
					Function: OAICallFunction{
						Name:      name,
						Arguments: args,
					},
				}},
			})
		case "function_call_output":
			flushPending()
			callID, _ := item["call_id"].(string)
			output := item["output"]
			var content any
			if s, ok := output.(string); ok {
				content = s
			} else {
				content = output
			}
			messages = append(messages, OAIMessage{
				Role:       "tool",
				ToolCallID: callID,
				Content:    content,
			})
		case "tool_result":
			flushPending()
			callID, _ := item["call_id"].(string)
			content := extractToolResultContentAny(item["content"])
			messages = append(messages, OAIMessage{
				Role:       "tool",
				ToolCallID: callID,
				Content:    content,
			})
		case "input_text":
			text, _ := item["text"].(string)
			if text != "" {
				pendingContent = append(pendingContent, OAIContentPart{Type: "text", Text: text})
			}
		case "input_image":
			pendingHasImage = true
			var imgURL string
			if u, ok := item["image_url"].(string); ok {
				imgURL = u
			} else if m, ok := item["image_url"].(map[string]any); ok {
				imgURL, _ = m["url"].(string)
			}
			if imgURL != "" {
				pendingContent = append(pendingContent, OAIContentPart{
					Type:     "image_url",
					ImageURL: &OAIImageURL{URL: imgURL, Detail: "auto"},
				})
			}
		default:
			if role, ok := item["role"].(string); ok {
				flushPending()
				contentRaw, _ := json.Marshal(item["content"])
				content := ResponsesContent(contentRaw)
				messages = append(messages, OAIMessage{Role: role, Content: content})
			}
		}
	}
	flushPending()
	return messages
}

func StreamUsageOptions(streaming bool) *OAIStreamOptions {
	if streaming {
		return &OAIStreamOptions{IncludeUsage: true}
	}
	return nil
}

func ParseOpenAIStreamChunk(data []byte) OpenAIStreamChunk {
	chunk := bytes.TrimPrefix(data, []byte("data: "))
	if len(chunk) == 0 || string(chunk) == "[DONE]" {
		return OpenAIStreamChunk{}
	}
	var raw struct {
		Choices []struct {
			Delta struct {
				Content          string                   `json:"content"`
				ReasoningContent string                   `json:"reasoning_content"`
				ToolCalls        []OpenAIStreamToolCall   `json:"tool_calls"`
			} `json:"delta"`
			Index int `json:"index"`
		} `json:"choices"`
		Usage json.RawMessage `json:"usage"`
	}
	if err := json.Unmarshal(chunk, &raw); err != nil {
		return OpenAIStreamChunk{}
	}
	result := OpenAIStreamChunk{}
	if len(raw.Choices) > 0 {
		result.Content = raw.Choices[0].Delta.Content
		result.ReasoningContent = raw.Choices[0].Delta.ReasoningContent
		if raw.Choices[0].Delta.ToolCalls != nil {
			result.ToolCalls = raw.Choices[0].Delta.ToolCalls
		}
	}
	if raw.Usage != nil {
		result.Usage = UsageFromJSON(raw.Usage)
	}
	return result
}

func OpenAITextDelta(data []byte) string {
	return ParseOpenAIStreamChunk(data).Content
}

func NormalizeSystem(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		wrapped, _ := json.Marshal([]map[string]any{
			{"type": "text", "text": s},
		})
		return wrapped
	}
	return raw
}

func NormalizeContent(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		wrapped, _ := json.Marshal([]map[string]any{
			{"type": "text", "text": s},
		})
		return wrapped
	}
	return raw
}

func CopyAnthropicToolResultContent(dst map[string]any, src map[string]json.RawMessage) {
	if content, ok := src["content"]; ok {
		var v any
		if err := json.Unmarshal(content, &v); err == nil {
			dst["content"] = TruncateToolResultContent(v)
		} else {
			dst["content"] = string(content)
		}
	}
}

func TruncateToolResultContent(v any) any {
	remaining := MaxAnthropicToolResultContentChars
	return TruncateToolResultContentValue(v, &remaining)
}

func TruncateToolResultContentValue(v any, remaining *int) any {
	if remaining == nil || *remaining <= 0 {
		return v
	}
	switch val := v.(type) {
	case string:
		return TruncateStringToBudget(val, remaining)
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, vv := range val {
			result[k] = TruncateToolResultContentValue(vv, remaining)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, vv := range val {
			result[i] = TruncateToolResultContentValue(vv, remaining)
		}
		return result
	default:
		return v
	}
}

func TruncateStringToBudget(s string, remaining *int) string {
	if remaining == nil || *remaining <= 0 {
		return ""
	}
	if len(s) <= *remaining {
		*remaining -= len(s)
		return s
	}
	result := s[:*remaining]
	*remaining = 0
	return result
}

func CopyRawJSONField(dst map[string]any, src map[string]json.RawMessage, key string) {
	if val, ok := src[key]; ok {
		var v any
		if err := json.Unmarshal(val, &v); err == nil {
			dst[key] = v
		} else {
			dst[key] = string(val)
		}
	}
}

func RawJSONAny(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	return v, true
}

func StripCacheControl(v any) any {
	switch val := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, vv := range val {
			if k == "cache_control" {
				continue
			}
			result[k] = StripCacheControl(vv)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, vv := range val {
			result[i] = StripCacheControl(vv)
		}
		return result
	default:
		return v
	}
}
