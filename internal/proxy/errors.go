package proxy

import (
	"encoding/json"
	"net/http"
)

type OpenAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param,omitempty"`
	Code    string `json:"code,omitempty"`
}

type OpenAIErrorEnvelope struct {
	Error OpenAIError `json:"error"`
}

func WriteOpenAIError(w http.ResponseWriter, status int, message, typ, param, code string) {
	if typ == "" {
		typ = "invalid_request_error"
	}
	body := OpenAIErrorEnvelope{
		Error: OpenAIError{
			Message: message,
			Type:    typ,
			Param:   param,
			Code:    code,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
