package commands

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/shaunhoulihan/arazzo-go/internal"
)

func TestEffectiveLogLevel(t *testing.T) {
	t.Parallel()

	if got := effectiveLogLevel("info", false); got != slog.LevelInfo {
		t.Fatalf("default info: got %v", got)
	}
	if got := effectiveLogLevel("warn", false); got != slog.LevelWarn {
		t.Fatalf("warn: got %v", got)
	}
	if got := effectiveLogLevel("error", true); got != slog.LevelDebug {
		t.Fatalf("verbose overrides log-level: got %v", got)
	}
	if got := effectiveLogLevel("info", true); got != slog.LevelDebug {
		t.Fatalf("verbose: got %v", got)
	}
}

func TestVersionFlagPrintsBuildVersion(t *testing.T) {
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"--version"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetArgs(nil)
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "arazzo version " + internal.BuildVersion + "\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if rootCmd.Flags().ShorthandLookup("v") != nil {
		t.Fatal("root -v is reserved for execute-workflow verbose logs")
	}
}

func TestExecuteWorkflowVerboseFlag(t *testing.T) {
	t.Parallel()
	flag := executeCmd.Flags().Lookup("verbose")
	if flag == nil {
		t.Fatal("missing --verbose")
	}
	if flag.Shorthand != "v" {
		t.Fatalf("shorthand: got %q", flag.Shorthand)
	}
	if flag.DefValue != "false" {
		t.Fatalf("default: got %q", flag.DefValue)
	}
}
