package models

import (
	"context"
	"encoding/json"
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

	officialObject  = "model"
	officialOwnedBy = "opencode"
)

type OfficialModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type officialModelsResponse struct {
	Object string          `json:"object"`
	Data   []OfficialModel `json:"data"`
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
		"minimax-m3",
		"minimax-m2.7",
		"minimax-m2.5",
		"kimi-k2.6",
		"kimi-k2.5",
		"glm-5.1",
		"glm-5",
		"deepseek-v4-pro",
		"deepseek-v4-flash",
		"qwen3.7-max",
		"qwen3.7-plus",
		"qwen3.6-plus",
		"qwen3.5-plus",
		"mimo-v2-pro",
		"mimo-v2-omni",
		"mimo-v2.5-pro",
		"mimo-v2.5",
		"hy3-preview",
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
		return nil, fmt.Errorf("remote models API returned status %d", resp.StatusCode)
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

func fetchOfficialModels() ([]OfficialModel, error) {
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, officialModelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create official models request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("official models API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read official models response: %w", err)
	}

	return parseOfficialModelsBody(body)
}

func parseOfficialModelsBody(body []byte) ([]OfficialModel, error) {
	var apiResp officialModelsResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal official models: %w", err)
	}

	out := make([]OfficialModel, 0, len(apiResp.Data))
	seen := make(map[string]struct{}, len(apiResp.Data))
	for _, m := range apiResp.Data {
		id := NormalizeID(m.ID)
		if strings.TrimSpace(id) == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}

		obj := m.Object
		if obj == "" {
			obj = officialObject
		}
		ownedBy := m.OwnedBy
		if ownedBy == "" {
			ownedBy = officialOwnedBy
		}
		out = append(out, OfficialModel{
			ID:      id,
			Object:  obj,
			Created: m.Created,
			OwnedBy: ownedBy,
		})
	}
	return out, nil
}

func getRemoteModels() (map[string]remoteModelInfo, error) {
	return remoteModels.get()
}

func getOfficialModels() ([]OfficialModel, error) {
	return officialModels.get()
}

func NormalizeID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimPrefix(id, "opencode-go/")
	return id
}

func OfficialModels() ([]OfficialModel, error) {
	models, err := getOfficialModels()
	if err != nil {
		return nil, err
	}
	out := make([]OfficialModel, len(models))
	copy(out, models)
	return out, nil
}

func OfficialModelIDs() ([]string, error) {
	models, err := getOfficialModels()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(models))
	for i, m := range models {
		ids[i] = m.ID
	}
	return ids, nil
}

func KnownModels() []OfficialModel {
	if models, err := getOfficialModels(); err == nil && len(models) > 0 {
		out := make([]OfficialModel, len(models))
		copy(out, models)
		return out
	}

	if remote, err := getRemoteModels(); err == nil && len(remote) > 0 {
		ids := make([]string, 0, len(remote))
		for id := range remote {
			if strings.TrimSpace(id) != "" {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		out := make([]OfficialModel, len(ids))
		for i, id := range ids {
			out[i] = OfficialModel{
				ID:      id,
				Object:  officialObject,
				Created: 0,
				OwnedBy: officialOwnedBy,
			}
		}
		return out
	}

	out := make([]OfficialModel, len(fallbackModelIDs))
	for i, id := range fallbackModelIDs {
		out[i] = OfficialModel{
			ID:      id,
			Object:  officialObject,
			Created: 0,
			OwnedBy: officialOwnedBy,
		}
	}
	return out
}

func KnownIDs() []string {
	known := KnownModels()
	ids := make([]string, 0, len(known))
	for _, m := range known {
		ids = append(ids, m.ID)
	}
	return ids
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
	id := NormalizeID(model)

	meta := ModelMetadata{
		DisplayName:             id,
		Description:             "OpenCode Go model",
		InputModalities:         []string{"text"},
		CodexInputModalities:    []string{"text"},
		ContextWindow:           128000,
		MaxContextWindow:        128000,
		DefaultReasoningSummary: "none",
	}

	if remote, err := getRemoteModels(); err == nil {
		if info, ok := remote[id]; ok {
			if info.Name != "" {
				meta.DisplayName = info.Name
				meta.Description = info.Name + " via OpenCode Go"
			}
			if len(info.Modalities.Input) > 0 {
				meta.InputModalities = append([]string(nil), info.Modalities.Input...)
				meta.CodexInputModalities = CodexSupportedModalities(info.Modalities.Input)
			}
			if info.Limit.Context > 0 {
				meta.ContextWindow = info.Limit.Context
				meta.MaxContextWindow = info.Limit.Context
			}
		}
	}

	switch id {
	case "minimax-m3":
		if meta.DisplayName == id {
			meta.DisplayName = "MiniMax M3"
			meta.Description = "MiniMax M3 via OpenCode Go"
		}
		if meta.ContextWindow == 128000 {
			meta.ContextWindow = 512000
		}
		if meta.MaxContextWindow == 128000 {
			meta.MaxContextWindow = 512000
		}
		if len(meta.InputModalities) == 1 && meta.InputModalities[0] == "text" {
			meta.InputModalities = []string{"text", "image", "video"}
			meta.CodexInputModalities = []string{"text", "image"}
		}
		meta.UsesAnthropicEndpoint = true
		meta.ParallelToolCalls = true
		meta.SupportsImageOriginal = true
		meta.SupportsSearchTool = true
	case "minimax-m2.7":
		meta.UsesAnthropicEndpoint = true
		meta.ParallelToolCalls = true
		meta.SupportsImageOriginal = true
		meta.SupportsSearchTool = true
	case "minimax-m2.5":
		meta.UsesAnthropicEndpoint = true
		meta.ParallelToolCalls = true
		meta.SupportsImageOriginal = true
		meta.SupportsSearchTool = true
	case "qwen3.7-max":
		meta.UsesAnthropicEndpoint = true
		meta.ParallelToolCalls = true
		meta.SupportsImageOriginal = true
		meta.SupportsSearchTool = true
	case "qwen3.7-plus":
		meta.ParallelToolCalls = true
	case "hy3-preview":
		meta.ParallelToolCalls = true
	case "kimi-k2.6", "kimi-k2.5", "mimo-v2-omni":
		if len(meta.InputModalities) == 1 && meta.InputModalities[0] == "text" {
			meta.InputModalities = []string{"text", "image"}
			meta.CodexInputModalities = []string{"text", "image"}
		}
		meta.ParallelToolCalls = true
		meta.SupportsImageOriginal = true
		meta.SupportsSearchTool = true
	}

	return meta
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
