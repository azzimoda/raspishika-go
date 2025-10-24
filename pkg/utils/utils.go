package utils

import (
	"io"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

func Windows1251ToUTF8(s string) (string, error) {
	decoder := charmap.Windows1251.NewDecoder()

	reader := transform.NewReader(strings.NewReader(s), decoder)
	result, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}

	return string(result), nil
}

func EscapeMarkdown(s string) string {
	return strings.NewReplacer(
		"\\", "\\\\",
		"*", "\\*",
		"_", "\\_",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		".", "\\.",
		"-", "\\-",
	).Replace(s)
}
