package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	DefaultLimit   = 20
	MaxLimit       = 50
	maxSafeInteger = 9007199254740991
)

var integerPattern = regexp.MustCompile(`^\d+$`)

type Pagination struct {
	Offset int
	Limit  int
}

func parseInteger(values []string, minimum, fallback int) (int, string) {
	if len(values) == 0 {
		return fallback, ""
	}
	if len(values) > 1 {
		return 0, "must be an integer"
	}
	text := strings.TrimSpace(values[0])
	if !integerPattern.MatchString(text) {
		return 0, "must be an integer"
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil || parsed > maxSafeInteger || int(parsed) < minimum {
		return 0, fmt.Sprintf("must be at least %d", minimum)
	}
	return int(parsed), ""
}

func ParsePagination(offset, limit []string) (Pagination, error) {
	parsedOffset, offsetProblem := parseInteger(offset, 0, 0)
	parsedLimit, limitProblem := parseInteger(limit, 1, DefaultLimit)
	problems := map[string][]string{}
	if offsetProblem != "" {
		problems["offset"] = []string{offsetProblem}
	}
	if limitProblem != "" {
		problems["limit"] = []string{limitProblem}
	}
	if len(problems) > 0 {
		return Pagination{}, NewValidationError(problems)
	}
	return Pagination{Offset: parsedOffset, Limit: min(parsedLimit, MaxLimit)}, nil
}
