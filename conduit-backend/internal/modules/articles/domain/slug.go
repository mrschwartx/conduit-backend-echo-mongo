package domain

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const (
	MaxSlugLength = 80
	fallbackSlug  = "article"
)

var (
	ErrSlugTaken  = errors.New("slug is already taken")
	reservedSlugs = []string{"feed"}
	nonAlnum      = regexp.MustCompile(`[^a-z0-9]+`)
)

func Slugify(title string) string {
	decomposed := norm.NFKD.String(title)
	stripped := strings.Map(func(character rune) rune {
		if unicode.Is(unicode.M, character) {
			return -1
		}
		return character
	}, decomposed)
	base := trimDashes(nonAlnum.ReplaceAllString(strings.ToLower(stripped), "-"))
	if len(base) > MaxSlugLength {
		base = base[:MaxSlugLength]
	}
	base = trimDashes(base)
	if base == "" {
		return fallbackSlug
	}
	return base
}

func SlugCandidate(base string, attempt int) string {
	if attempt == 1 {
		return base
	}
	suffix := "-" + strconv.Itoa(attempt)
	room := MaxSlugLength - len(suffix)
	if len(base) > room {
		base = base[:room]
	}
	return trimDashes(base) + suffix
}

func IsReservedSlug(slug string) bool {
	return slices.Contains(reservedSlugs, slug)
}

func trimDashes(value string) string {
	return strings.Trim(value, "-")
}
