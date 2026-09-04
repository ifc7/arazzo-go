package ui

import (
	"strings"
	"testing"
)

func TestScreenTitle(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := ScreenTitle("Workflows")
	if !strings.HasPrefix(got, "arazzo") {
		t.Fatalf("expected wordmark, got %q", got)
	}
	if !strings.Contains(got, "Workflows") || !strings.Contains(got, "─") {
		t.Fatalf("expected title and divider, got %q", got)
	}
}

func TestField(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if got := Field("status", "success"); got != "status: success" {
		t.Fatalf("got %q", got)
	}
}
