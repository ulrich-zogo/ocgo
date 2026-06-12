package support

import (
	"testing"
)

func TestRedactJSONAPIKey(t *testing.T) {
	in := []byte(`{"api_key":"SUPER_SECRET_TEST_KEY","host":"127.0.0.1","port":3456}`)
	out := RedactJSONBytes(in)
	if contains(string(out), "SUPER_SECRET_TEST_KEY") {
		t.Fatal("secret key leaked in JSON redaction")
	}
	if !contains(string(out), "[REDACTED]") {
		t.Fatal("expected [REDACTED] in output")
	}
}

func TestRedactJSONNestedToken(t *testing.T) {
	in := []byte(`{"nested":{"token":"my-secret-token"},"api_key":"another-secret"}`)
	out := RedactJSONBytes(in)
	if contains(string(out), "my-secret-token") {
		t.Fatal("nested token leaked")
	}
	if contains(string(out), "another-secret") {
		t.Fatal("api_key leaked")
	}
}

func TestRedactJSONAuthorization(t *testing.T) {
	in := []byte(`{"Authorization":"Bearer sk-my-test-key-12345"}`)
	out := RedactJSONBytes(in)
	if contains(string(out), "sk-my-test-key-12345") {
		t.Fatal("bearer token leaked")
	}
	if contains(string(out), "[REDACTED]") {
		return
	}
	if contains(string(out), "redacted") {
		return
	}
	t.Fatal("expected redacted value in output")
}

func TestRedactJSONAccessToken(t *testing.T) {
	in := []byte(`{"access_token":"ghp_abc123def456","x-api-key":"xap-key-789"}`)
	out := RedactJSONBytes(in)
	if contains(string(out), "ghp_abc123def456") {
		t.Fatal("access_token leaked")
	}
	if contains(string(out), "xap-key-789") {
		t.Fatal("x-api-key leaked")
	}
}

func TestRedactJSONInvalid(t *testing.T) {
	in := []byte(`not json { api_key: "leaked" }`)
	out := RedactJSONBytes(in)
	if contains(string(out), "leaked") {
		t.Fatal("value leaked through invalid JSON redaction")
	}
}

func TestRedactTOMLAPIKey(t *testing.T) {
	in := `api_key = "sk-my-test-key"`
	out := RedactText(in)
	if contains(out, "sk-my-test-key") {
		t.Fatal("TOML api_key leaked")
	}
}

func TestRedactTextBearerToken(t *testing.T) {
	in := `Authorization: Bearer sk-my-bearer-token`
	out := RedactText(in)
	if contains(out, "sk-my-bearer-token") {
		t.Fatal("bearer token leaked in text")
	}
}

func TestRedactTextOpenAIToken(t *testing.T) {
	in := `sk-my-openai-key-abcdef`
	out := RedactText(in)
	if !contains(out, "[REDACTED]") {
		t.Fatal("expected OpenAI key to be redacted")
	}
}

func TestRedactTextEnvVarStyle(t *testing.T) {
	in := `ANTHROPIC_AUTH_TOKEN=sk-ant-my-token`
	out := RedactText(in)
	if contains(out, "sk-ant-my-token") {
		t.Fatal("env var style token leaked")
	}
}

func TestRedactTextOPENCODE(t *testing.T) {
	in := `OPENCODE_GO_API_KEY=sk-oc-my-key`
	out := RedactText(in)
	if contains(out, "sk-oc-my-key") {
		t.Fatal("opencode key leaked")
	}
}

func TestRedactTextOPENAI(t *testing.T) {
	in := `OPENAI_API_KEY=sk-openai-key`
	out := RedactText(in)
	if contains(out, "sk-openai-key") {
		t.Fatal("openai key leaked")
	}
}

func TestRedactTextSecretPassword(t *testing.T) {
	in := `password = "my-password"`
	out := RedactText(in)
	if contains(out, "my-password") {
		t.Fatal("password leaked")
	}
}

func TestRedactTextRefreshToken(t *testing.T) {
	in := `refresh_token = "r-abc-123-def"`
	out := RedactText(in)
	if contains(out, "r-abc-123-def") {
		t.Fatal("refresh_token leaked")
	}
}

func TestRedactTextSecretInLogLine(t *testing.T) {
	in := `2026-06-12 INFO request authorized: Bearer sk-my-key for user test`
	out := RedactText(in)
	if contains(out, "sk-my-key") {
		t.Fatal("bearer token leaked in log line")
	}
	if !contains(out, "[REDACTED]") {
		t.Fatal("expected [REDACTED] in redacted log line")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
