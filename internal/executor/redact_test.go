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

func TestTruncateForLog(t *testing.T) {
	t.Parallel()
	if got := truncateForLog("short", 10); got != "short" {
		t.Fatalf("got %q", got)
	}
	if got := truncateForLog("abcdefghij", 4); got != "abcd…" {
		t.Fatalf("got %q", got)
	}
}

func TestRedactURLHidesAuthorizationQuery(t *testing.T) {
	t.Parallel()
	raw := "https://example.com/api/v1/asset-models?Authorization=Bearer+secret-token&limit=10"
	got := redactURL(raw)
	if strings.Contains(got, "secret-token") {
		t.Fatalf("token leaked: %s", got)
	}
	if !strings.Contains(got, "limit=10") {
		t.Fatalf("non-secret query should remain: %s", got)
	}
}
