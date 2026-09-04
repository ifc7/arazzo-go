// Package ui provides shared terminal styling aligned with the ifc-web brand.
package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

// Brand colors from ifc-web / docs/logo.svg.
const (
	hexPrimary      = "#6C63FF"
	hexPrimaryLight = "#5B52F4"
	hexSuccess      = "#00D27F"
	hexSuccessLight = "#0F766E"
	hexAccent       = "#C6F221"
	hexError        = "#EF4444"
	hexErrorLight   = "#DC2626"
	hexWarning      = "#F59E0B"
	hexWarningLight = "#D97706"
	hexMuted        = "#9CA3AF"
	hexMutedLight   = "#6B7280"
)

var (
	colorPrimary = lipgloss.AdaptiveColor{Light: hexPrimaryLight, Dark: hexPrimary}
	colorSuccess = lipgloss.AdaptiveColor{Light: hexSuccessLight, Dark: hexSuccess}
	colorAccent  = lipgloss.AdaptiveColor{Light: hexSuccessLight, Dark: hexAccent}
	colorError   = lipgloss.AdaptiveColor{Light: hexErrorLight, Dark: hexError}
	colorWarning = lipgloss.AdaptiveColor{Light: hexWarningLight, Dark: hexWarning}
	colorMuted   = lipgloss.AdaptiveColor{Light: hexMutedLight, Dark: hexMuted}
)

// Exported styles for reuse across commands.
var (
	Primary  = lipgloss.NewStyle().Foreground(colorPrimary)
	Success  = lipgloss.NewStyle().Foreground(colorSuccess)
	Accent   = lipgloss.NewStyle().Foreground(colorAccent)
	Error    = lipgloss.NewStyle().Foreground(colorError)
	Warning  = lipgloss.NewStyle().Foreground(colorWarning)
	Muted    = lipgloss.NewStyle().Foreground(colorMuted)
	Emphasis = lipgloss.NewStyle().Bold(true)
	Title    = lipgloss.NewStyle().Foreground(colorPrimary).Bold(true)
	Label    = lipgloss.NewStyle().Foreground(colorMuted)
)

// ColorEnabled reports whether ANSI styling should be applied.
func ColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("CLICOLOR_FORCE") == "1" {
		return true
	}
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

func render(style lipgloss.Style, s string) string {
	if !ColorEnabled() {
		return s
	}
	return style.Render(s)
}

// Apply renders s with style when color output is enabled.
func Apply(style lipgloss.Style, s string) string {
	return render(style, s)
}

// Wordmark returns a short branded mark for headers.
func Wordmark() string {
	return render(Title, "arazzo")
}

// ScreenTitle renders "arazzo · <title>" with a divider.
func ScreenTitle(title string) string {
	head := Wordmark() + render(Muted, "  ·  ") + render(Emphasis, title)
	return head + "\n" + Divider() + "\n"
}

// Divider is a faint horizontal rule.
func Divider() string {
	return render(Muted, "────────────────────────")
}

// Section renders a section heading.
func Section(text string) string {
	return render(Label, text)
}

// KeyHints renders muted secondary text (column headers, summaries, durations).
func KeyHints(text string) string {
	return render(Muted, text)
}

// Field renders a "Label: value" row.
func Field(label, value string) string {
	return render(Label, label+":") + " " + value
}
