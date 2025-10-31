package utils

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const GroupRegexp = `^(\p{Cyrillic}{3,5})[- ]*(\d{2})[- ]*\(?(9|11)\)?[- ]*(\d)$`

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

// ValidateGroupNameFormat determines whether the given string can be formatted into a valid group name,
// and if it can, returns valid group name, else returns an error.
// It uses regexp provided by the constant GroupRegexp.
//
// Important: the function doesn't validate case, i.e. if string "иСпТ-22-(9)-2" is given, the result is the same.
func ValidateGroupNameFormat(s string) (string, error) {
	r := regexp.MustCompile(GroupRegexp)
	if !r.MatchString(s) {
		return s, fmt.Errorf("group name '%s' doesn't match the pattern", s)
	}
	subs := r.FindStringSubmatch(s)
	return fmt.Sprintf("%s-%s-(%s)-%s", subs[1], subs[2], subs[3], subs[4]), nil
}

func DerefOrTypeDefault[T any](s *T) T {
	var val T
	if s == nil {
		return val
	} else {
		return *s
	}
}
