package ui

import "fmt"

// Successln prints a success message with a green checkmark.
func Successln(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(render(Success, "✓") + " " + msg)
}

// Errorln prints an error message with a red cross (to stdout).
func Errorln(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(render(Error, "✗") + " " + msg)
}

// Warnln prints a warning message.
func Warnln(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(render(Warning, "!") + " " + msg)
}

// Infoln prints a muted informational message.
func Infoln(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(render(Muted, "·") + " " + msg)
}
