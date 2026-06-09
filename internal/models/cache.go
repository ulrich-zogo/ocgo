package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ocgo/internal/config"
)

const (
	catalogCacheVersion = 1
	sourceOfficial      = "official"
	sourceCache         = "cache"
	sourceRemote        = "remote"
	sourceFallback      = "fallback"
)

type CatalogCache struct {
	Version   int             `json:"version"`
	Source    string          `json:"source"`
	FetchedAt time.Time       `json:"fetched_at"`
	Models    []OfficialModel `json:"models"`
}

var CatalogCacheFile = config.ModelCatalogCacheFile

func ReadCatalogCache(path string) ([]OfficialModel, error) {
	if path == "" {
		return nil, fmt.Errorf("empty cache path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cache: %w", err)
	}
	var cache CatalogCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("parse cache: %w", err)
	}
	if cache.Version != catalogCacheVersion {
		return nil, fmt.Errorf("cache version %d != %d", cache.Version, catalogCacheVersion)
	}
	return normalizeOfficialModels(cache.Models), nil
}

func WriteCatalogCache(path string, models []OfficialModel, source string, fetchedAt time.Time) error {
	if path == "" {
		return fmt.Errorf("empty cache path")
	}
	if models == nil {
		models = []OfficialModel{}
	}
	cache := CatalogCache{
		Version:   catalogCacheVersion,
		Source:    source,
		FetchedAt: fetchedAt,
		Models:    append([]OfficialModel(nil), models...),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("write cache: %w", err)
	}
	return nil
}

func normalizeOfficialModels(in []OfficialModel) []OfficialModel {
	out := make([]OfficialModel, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, m := range in {
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
	return out
}


