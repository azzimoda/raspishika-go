package utils

import (
	"io"
	"strings"

	"github.com/schollz/closestmatch"
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

// MatchStrings returns the closest matches to the target string in the given list of strings.
func MatchStrings(strs []string, target string, n int) []string {
	for _, s := range strs {
		if strings.EqualFold(s, target) {
			return []string{s}
		}
	}
	return closestmatch.New(strs, []int{2, 3, 4}).ClosestN(target, n)
}
