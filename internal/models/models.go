package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	remoteModelsURL   = "https://models.dev/api.json"
	officialModelsURL = "https://opencode.ai/zen/go/v1/models"
)

type officialModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

type remoteModelInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Modalities struct {
		Input  []string `json:"input"`
		Output []string `json:"output"`
	} `json:"modalities"`
	Limit struct {
		Context int `json:"context"`
		Output  int `json:"output"`
	} `json:"limit"`
}

type remoteAPIResponse struct {
	OpenCodeGo struct {
		Models map[string]remoteModelInfo `json:"models"`
	} `json:"opencode-go"`
}

var httpClient = &http.Client{Timeout: 8 * time.Second}

type lazyFetcher[T any] struct {
	mu      sync.RWMutex
	data    T
	err     error
	fetched bool
	fetch   func() (T, error)
}

func newLazyFetcher[T any](fetch func() (T, error)) *lazyFetcher[T] {
	return &lazyFetcher[T]{fetch: fetch}
}

func (f *lazyFetcher[T]) get() (T, error) {
	f.mu.RLock()
	if f.fetched {
		f.mu.RUnlock()
		return f.data, f.err
	}
	f.mu.RUnlock()

	f.mu.Lock()
	if f.fetched {
		f.mu.Unlock()
		return f.data, f.err
	}
	f.data, f.err = f.fetch()
	f.fetched = true
	f.mu.Unlock()
	return f.data, f.err
}

func (f *lazyFetcher[T]) refresh() {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, err := f.fetch()
	f.data = data
	f.err = err
	f.fetched = true
}

var (
	remoteModels   = newLazyFetcher(fetchRemoteModels)
	officialModels = newLazyFetcher(fetchOfficialModels)

	fallbackModelIDs = []string{
		"glm-5.1", "glm-5",
		"kimi-k2.6", "kimi-k2.5",
		"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-pro", "mimo-v2-omni",
		"minimax-m3", "minimax-m2.7", "minimax-m2.5",
		"deepseek-v4-pro", "deepseek-v4-flash",
		"qwen3.7-max", "qwen3.6-plus", "qwen3.5-plus",
	}
)

func fetchRemoteModels() (map[string]remoteModelInfo, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, remoteModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create remote models request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("remote models API returned non-200 status")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read remote models response: %w", err)
	}

	var apiResp remoteAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal remote models: %w", err)
	}

	return apiResp.OpenCodeGo.Models, nil
}

func fetchOfficialModels() ([]string, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, officialModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create official models request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read official models response: %w", err)
	}

	var apiResp officialModelsResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal official models: %w", err)
	}

	ids := make([]string, len(apiResp.Data))
	for i, d := range apiResp.Data {
		ids[i] = d.ID
	}
	return ids, nil
}

func getRemoteModels() (map[string]remoteModelInfo, error) {
	return remoteModels.get()
}

func getOfficialModels() ([]string, error) {
	return officialModels.get()
}

func NormalizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "opencode-go/")
	return id
}

func KnownIDs() []string {
	seen := make(map[string]struct{})
	var result []string

	official, err := getOfficialModels()
	if err == nil {
		for _, id := range official {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
	}

	remote, err := getRemoteModels()
	if err == nil {
		for id := range remote {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				result = append(result, id)
			}
		}
	}

	for _, id := range fallbackModelIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			result = append(result, id)
		}
	}

	sort.Strings(result)
	return result
}

func IsKnown(id string) bool {
	id = NormalizeID(id)
	for _, known := range KnownIDs() {
		if known == id {
			return true
		}
	}
	return false
}

func RefreshAll() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		remoteModels.refresh()
	}()
	go func() {
		defer wg.Done()
		officialModels.refresh()
	}()
	wg.Wait()
}

type ModelMetadata struct {
	DisplayName             string
	Description             string
	InputModalities         []string
	CodexInputModalities    []string
	ContextWindow           int
	MaxContextWindow        int
	UsesAnthropicEndpoint   bool
	ParallelToolCalls       bool
	SupportsImageOriginal   bool
	SupportsSearchTool      bool
	SupportedReasoning      []any
	DefaultReasoningLevel   any
	ReasoningSummaries      bool
	DefaultReasoningSummary string
}

func Metadata(model string) ModelMetadata {
	switch NormalizeID(model) {
	case "minimax-m3":
		return ModelMetadata{
			DisplayName:           "MiniMax M3",
			Description:           "MiniMax M3 model",
			InputModalities:       []string{"text", "image"},
			CodexInputModalities:  []string{"text", "image"},
			ContextWindow:         1048576,
			MaxContextWindow:      1048576,
			UsesAnthropicEndpoint: false,
			ParallelToolCalls:     true,
			SupportsImageOriginal: true,
			SupportsSearchTool:    true,
			SupportedReasoning:    []any{},
			DefaultReasoningLevel: nil,
			ReasoningSummaries:    false,
			DefaultReasoningSummary: "",
		}
	case "minimax-m2.7":
		return ModelMetadata{
			DisplayName:           "MiniMax M2.7",
			Description:           "MiniMax M2.7 model",
			InputModalities:       []string{"text", "image"},
			CodexInputModalities:  []string{"text", "image"},
			ContextWindow:         262144,
			MaxContextWindow:      262144,
			UsesAnthropicEndpoint: false,
			ParallelToolCalls:     true,
			SupportsImageOriginal: true,
			SupportsSearchTool:    true,
			SupportedReasoning:    []any{},
			DefaultReasoningLevel: nil,
			ReasoningSummaries:    false,
			DefaultReasoningSummary: "",
		}
	case "minimax-m2.5":
		return ModelMetadata{
			DisplayName:           "MiniMax M2.5",
			Description:           "MiniMax M2.5 model",
			InputModalities:       []string{"text", "image"},
			CodexInputModalities:  []string{"text", "image"},
			ContextWindow:         262144,
			MaxContextWindow:      262144,
			UsesAnthropicEndpoint: false,
			ParallelToolCalls:     true,
			SupportsImageOriginal: true,
			SupportsSearchTool:    true,
			SupportedReasoning:    []any{},
			DefaultReasoningLevel: nil,
			ReasoningSummaries:    false,
			DefaultReasoningSummary: "",
		}
	case "qwen3.7-max":
		return ModelMetadata{
			DisplayName:           "Qwen 3.7 Max",
			Description:           "Qwen 3.7 Max model",
			InputModalities:       []string{"text", "image"},
			CodexInputModalities:  []string{"text", "image"},
			ContextWindow:         131072,
			MaxContextWindow:      131072,
			UsesAnthropicEndpoint: false,
			ParallelToolCalls:     true,
			SupportsImageOriginal: true,
			SupportsSearchTool:    true,
			SupportedReasoning:    []any{},
			DefaultReasoningLevel: nil,
			ReasoningSummaries:    false,
			DefaultReasoningSummary: "",
		}
	case "kimi-k2.6":
		return ModelMetadata{
			DisplayName:           "Kimi K2.6",
			Description:           "Kimi K2.6 model",
			InputModalities:       []string{"text", "image"},
			CodexInputModalities:  []string{"text", "image"},
			ContextWindow:         131072,
			MaxContextWindow:      131072,
			UsesAnthropicEndpoint: false,
			ParallelToolCalls:     true,
			SupportsImageOriginal: true,
			SupportsSearchTool:    true,
			SupportedReasoning:    []any{},
			DefaultReasoningLevel: nil,
			ReasoningSummaries:    false,
			DefaultReasoningSummary: "",
		}
	case "kimi-k2.5":
		return ModelMetadata{
			DisplayName:           "Kimi K2.5",
			Description:           "Kimi K2.5 model",
			InputModalities:       []string{"text", "image"},
			CodexInputModalities:  []string{"text", "image"},
			ContextWindow:         131072,
			MaxContextWindow:      131072,
			UsesAnthropicEndpoint: false,
			ParallelToolCalls:     true,
			SupportsImageOriginal: true,
			SupportsSearchTool:    true,
			SupportedReasoning:    []any{},
			DefaultReasoningLevel: nil,
			ReasoningSummaries:    false,
			DefaultReasoningSummary: "",
		}
	case "mimo-v2-omni":
		return ModelMetadata{
			DisplayName:           "MIMO V2 Omni",
			Description:           "MIMO V2 Omni model",
			InputModalities:       []string{"text", "image"},
			CodexInputModalities:  []string{"text", "image"},
			ContextWindow:         131072,
			MaxContextWindow:      131072,
			UsesAnthropicEndpoint: false,
			ParallelToolCalls:     true,
			SupportsImageOriginal: true,
			SupportsSearchTool:    true,
			SupportedReasoning:    []any{},
			DefaultReasoningLevel: nil,
			ReasoningSummaries:    false,
			DefaultReasoningSummary: "",
		}
	}
	return ModelMetadata{}
}

func UsesAnthropicEndpoint(id string) bool {
	return Metadata(id).UsesAnthropicEndpoint
}

func SupportsImages(id string) bool {
	return Metadata(id).SupportsImageOriginal
}

func InputModalities(id string) []string {
	return Metadata(id).InputModalities
}

func CodexSupportedModalities(modalities []string) []string {
	result := make([]string, 0, len(modalities))
	for _, m := range modalities {
		m = strings.TrimSpace(strings.ToLower(m))
		if m == "text" || m == "image" {
			result = append(result, m)
		}
	}
	if len(result) == 0 {
		return []string{"text"}
	}
	return result
}
