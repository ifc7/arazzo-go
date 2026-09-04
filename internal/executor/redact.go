package executor

import (
	"encoding/json"
	"strings"
)

var secretKeyFragments = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"session",
	"totp",
	"otp",
	"credential",
	"authorization",
	"cookie",
}

func isSecretKey(key string) bool {
	k := strings.ToLower(key)
	for _, frag := range secretKeyFragments {
		if strings.Contains(k, frag) {
			return true
		}
	}
	return false
}

func redactValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if isSecretKey(k) {
				out[k] = "***"
				continue
			}
			out[k] = redactValue(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redactValue(val)
		}
		return out
	default:
		return v
	}
}

func redactJSON(v any) string {
	b, err := json.Marshal(redactValue(v))
	if err != nil {
		return "<unserializable>"
	}
	return string(b)
}
