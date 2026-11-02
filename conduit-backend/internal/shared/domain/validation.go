package domain

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	Blank   = "can't be blank"
	Invalid = "is invalid"
)

func TooShort(min int) string {
	return fmt.Sprintf("is too short (minimum is %d characters)", min)
}

func TooLong(max int) string {
	return fmt.Sprintf("is too long (maximum is %d characters)", max)
}

type TextRule struct {
	Min     int
	Max     int
	Pattern *regexp.Regexp
}

func ValidateText(field string, raw Raw, rule TextRule) (string, error) {
	text, ok := raw.Text()
	if !ok {
		return "", NewFieldError(field, Blank)
	}
	value := TrimText(text)
	length := CountCharacters(value)
	minimum := rule.Min
	if length == 0 && minimum > 0 {
		return "", NewFieldError(field, Blank)
	}
	if length < minimum {
		return "", NewFieldError(field, TooShort(minimum))
	}
	if length > rule.Max {
		return "", NewFieldError(field, TooLong(rule.Max))
	}
	if rule.Pattern != nil && !rule.Pattern.MatchString(value) {
		return "", NewFieldError(field, Invalid)
	}
	return value, nil
}

func CountCharacters(value string) int {
	return utf8.RuneCountInString(value)
}

func IsSpace(character rune) bool {
	if character == 0xFEFF {
		return true
	}
	return character != 0x85 && unicode.IsSpace(character)
}

func TrimText(value string) string {
	return strings.TrimFunc(value, IsSpace)
}
