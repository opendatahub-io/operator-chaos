package cli

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// ANSI color codes for verdict coloring.
const (
	ansiReset  = "\033[0m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiRed    = "\033[31m"
	ansiGray   = "\033[90m"
)

// isTTY reports whether stdout is connected to a terminal.
func isTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// paddedColorVerdict returns the verdict string padded to the given width
// and wrapped in ANSI color codes when stdout is a TTY. The padding is
// applied to the plain text before wrapping so that ANSI escape bytes do
// not throw off column alignment.
func paddedColorVerdict(verdict string, width int) string {
	padded := fmt.Sprintf("%-*s", width, verdict)
	if !isTTY() {
		return padded
	}
	switch verdict {
	case "Resilient":
		return fmt.Sprintf("%s%s%s", ansiGreen, padded, ansiReset)
	case "Degraded":
		return fmt.Sprintf("%s%s%s", ansiYellow, padded, ansiReset)
	case "Failed":
		return fmt.Sprintf("%s%s%s", ansiRed, padded, ansiReset)
	case "Inconclusive":
		return fmt.Sprintf("%s%s%s", ansiGray, padded, ansiReset)
	default:
		return padded
	}
}

// colorVerdict returns the verdict string wrapped in ANSI color codes
// when stdout is a TTY. Otherwise, it returns the verdict unchanged.
// Green=Resilient, Yellow=Degraded, Red=Failed, Gray=Inconclusive.
func colorVerdict(verdict string) string {
	if !isTTY() {
		return verdict
	}
	switch verdict {
	case "Resilient":
		return fmt.Sprintf("%s%s%s", ansiGreen, verdict, ansiReset)
	case "Degraded":
		return fmt.Sprintf("%s%s%s", ansiYellow, verdict, ansiReset)
	case "Failed":
		return fmt.Sprintf("%s%s%s", ansiRed, verdict, ansiReset)
	case "Inconclusive":
		return fmt.Sprintf("%s%s%s", ansiGray, verdict, ansiReset)
	default:
		return verdict
	}
}
