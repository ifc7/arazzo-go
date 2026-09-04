package executor

import (
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	t.Parallel()
	got := redactJSON(map[string]any{
		"email":    "user@example.com",
		"password": "super-secret",
		"nested": map[string]any{
			"session": "tok",
		},
	})
	if strings.Contains(got, "super-secret") || strings.Contains(got, `"session":"tok"`) {
		t.Fatalf("secrets leaked: %s", got)
	}
	if !strings.Contains(got, "user@example.com") {
		t.Fatalf("email should remain visible: %s", got)
	}
}
