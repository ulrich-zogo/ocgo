package support

import (
	"encoding/json"
	"regexp"
	"strings"
)

var redactFields = map[string]bool{
	"api_key":               true,
	"apikey":                true,
	"access_token":          true,
	"refresh_token":         true,
	"token":                 true,
	"secret":                true,
	"password":              true,
	"authorization":         true,
	"bearer":                true,
	"x-api-key":             true,
	"x_api_key":             true,
	"anthropic_auth_token":  true,
	"opencode_go_api_key":   true,
	"openai_api_key":        true,
	"ANTHROPIC_AUTH_TOKEN":  true,
	"OPENCODE_GO_API_KEY":   true,
	"OPENAI_API_KEY":        true,
}

var redactPatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?i)(api_key\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(token\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(password\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(secret\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(refresh_token\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(access_token\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(x-api-key\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(x_api_key\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(anthropic_auth_token\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(opencode_go_api_key\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`(?i)(openai_api_key\s*=\s*)"[^"]*"`), `$1"[REDACTED]"`},
	{regexp.MustCompile(`Bearer\s+sk-[^"'` + "`" + `\s,;:)]+`), "Bearer [REDACTED]"},
	{regexp.MustCompile(`\b(ANTHROPIC_AUTH_TOKEN|OPENCODE_GO_API_KEY|OPENAI_API_KEY)=` + "`?[^`\"'\\s,;:)]+`?"), `$1=[REDACTED]`},
	{regexp.MustCompile(`\bsk-[a-zA-Z0-9_-]{8,}\b`), "[REDACTED]"},
}

func RedactJSONBytes(input []byte) []byte {
	var raw any
	if err := json.Unmarshal(input, &raw); err != nil {
		return []byte(redactText(string(input)))
	}
	redactValue(raw)
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return []byte(redactText(string(input)))
	}
	return b
}

func redactValue(v any) {
	switch m := v.(type) {
	case map[string]any:
		for k, val := range m {
			if redactFields[strings.ToLower(k)] {
				m[k] = "[REDACTED]"
			} else {
				redactValue(val)
			}
		}
	case []any:
		for _, item := range m {
			redactValue(item)
		}
	}
}

func RedactText(input string) string {
	return redactText(input)
}

func redactText(input string) string {
	result := input
	for _, p := range redactPatterns {
		result = p.re.ReplaceAllString(result, p.repl)
	}
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		lines[i] = redactLineByFields(line)
	}
	return strings.Join(lines, "\n")
}

func redactLineByFields(line string) string {
	lower := strings.ToLower(line)
	for field := range redactFields {
		if strings.Contains(lower, field) {
			line = regexp.MustCompile(`(?i)(`+regexp.QuoteMeta(field)+`\s*[:=]\s*)(\S+)`).ReplaceAllString(line, `$1[REDACTED]`)
		}
	}
	return line
}
