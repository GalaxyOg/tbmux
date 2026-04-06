package cliutil

import (
	"fmt"
	"os"
)

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiBold   = "\033[1m"
)

func ColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	st, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}

func Green(s string) string {
	return colorize(ansiGreen, s)
}

func Red(s string) string {
	return colorize(ansiRed, s)
}

func Yellow(s string) string {
	return colorize(ansiYellow, s)
}

func Blue(s string) string {
	return colorize(ansiBlue, s)
}

func Bold(s string) string {
	return colorize(ansiBold, s)
}

func StatusBool(v bool) string {
	if v {
		return Green("YES")
	}
	return Red("NO")
}

func StatusWord(word string) string {
	switch word {
	case "ok":
		return Green("OK")
	case "warn":
		return Yellow("WARN")
	case "error":
		return Red("ERROR")
	default:
		return word
	}
}

func colorize(code, s string) string {
	if !ColorEnabled() {
		return s
	}
	return fmt.Sprintf("%s%s%s", code, s, ansiReset)
}
