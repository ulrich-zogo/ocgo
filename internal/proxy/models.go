package proxy

import (
	"encoding/json"
	"fmt"
	"net/http"

	"ocgo/internal/models"
)

const (
	modelsListObject    = "list"
	modelsEntryObject   = "model"
	modelsEntryOwnedBy  = "opencode"
)

type modelsListResponse struct {
	Object string                 `json:"object"`
	Data   []models.OfficialModel `json:"data"`
}

func ProxyModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	known := models.KnownModels()

	data := make([]models.OfficialModel, 0, len(known))
	for _, m := range known {
		entry := m
		if entry.Object == "" {
			entry.Object = modelsEntryObject
		}
		if entry.OwnedBy == "" {
			entry.OwnedBy = modelsEntryOwnedBy
		}
		data = append(data, entry)
	}

	resp := modelsListResponse{
		Object: modelsListObject,
		Data:   data,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("encode models response: %v", err), http.StatusInternalServerError)
		return
	}
}
