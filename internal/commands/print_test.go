package commands

import (
	"errors"
	"strings"
	"testing"
	"time"

	libarazzo "github.com/pb33f/libopenapi/arazzo"

	"github.com/shaunhoulihan/arazzo-go/internal/runner"
)

func TestFormatWorkflowList(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := formatWorkflowList([]runner.WorkflowInfo{
		{WorkflowID: "login-with-password", Summary: "Log in with email and password"},
		{WorkflowID: "other"},
	})
	if !strings.Contains(got, "arazzo") || !strings.Contains(got, "Workflows") {
		t.Fatalf("missing title: %q", got)
	}
	if !strings.Contains(got, "login-with-password") || !strings.Contains(got, "Log in with email and password") {
		t.Fatalf("missing row: %q", got)
	}
	if !strings.Contains(got, "2 workflows") {
		t.Fatalf("missing summary: %q", got)
	}
}

func TestFormatWorkflowInfo(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := formatWorkflowInfo(&runner.WorkflowInfo{
		WorkflowID:  "login-with-password",
		Summary:     "Log in",
		Description: "Drives POST /auth/login.",
		OutputNames: []string{"session"},
		Steps: []runner.StepInfo{
			{StepID: "submit-credentials", OperationID: "$sourceDescriptions.userSessionApi.passwordLogin"},
		},
	})
	if !strings.Contains(got, "login-with-password") {
		t.Fatalf("missing workflow id: %q", got)
	}
	if !strings.Contains(got, "summary:") || !strings.Contains(got, "Log in") {
		t.Fatalf("missing summary: %q", got)
	}
	if !strings.Contains(got, "submit-credentials") {
		t.Fatalf("missing step: %q", got)
	}
	if !strings.Contains(got, "outputs: session") {
		t.Fatalf("missing outputs: %q", got)
	}
}

func TestFormatResult(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := formatResult(&libarazzo.WorkflowResult{
		WorkflowId: "login-with-password",
		Success:    false,
		Duration:   12 * time.Millisecond,
		Error:      errors.New("unauthorized"),
		Steps: []*libarazzo.StepResult{
			{
				StepId:   "submit-credentials",
				Success:  false,
				Duration: 5 * time.Millisecond,
				Error:    errors.New("401"),
			},
		},
	})
	if !strings.Contains(got, "status: failed") {
		t.Fatalf("missing status: %q", got)
	}
	if !strings.Contains(got, "submit-credentials") || !strings.Contains(got, "401") {
		t.Fatalf("missing step: %q", got)
	}
	if !strings.Contains(got, "unauthorized") {
		t.Fatalf("missing workflow error: %q", got)
	}
}
