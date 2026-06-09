package proxy

import (
	"encoding/json"
	"net/http"
	"strings"
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

func WriteUpstreamOpenAIError(w http.ResponseWriter, status int, body []byte) {
	env := OpenAIErrorEnvelope{
		Error: OpenAIError{
			Message: strings.TrimSpace(string(body)),
			Type:    "upstream_error",
			Code:    "upstream_failure",
		},
	}
	if msg, typ, param, code, ok := extractUpstreamOpenAIError(body); ok {
		env.Error.Message = msg
		if typ != "" {
			env.Error.Type = typ
		}
		env.Error.Param = param
		env.Error.Code = code
	}
	w.Header().Set("Content-Type", "application/json")
	if status == 0 {
		status = http.StatusBadGateway
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}

func extractUpstreamOpenAIError(body []byte) (message, typ, param, code string, ok bool) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", "", "", "", false
	}
	if !strings.HasPrefix(trimmed, "{") {
		return "", "", "", "", false
	}
	var env OpenAIErrorEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return "", "", "", "", false
	}
	if env.Error.Message == "" && env.Error.Type == "" {
		return "", "", "", "", false
	}
	return env.Error.Message, env.Error.Type, env.Error.Param, env.Error.Code, true
}
