package domain

import (
	"fmt"
	"slices"
	"strings"

	sharedomain "conduit/backend/internal/shared/domain"
)

const (
	MaxTags      = 10
	MaxTagLength = 30
)

type TagList struct {
	values []string
}

func NewTagList(raw sharedomain.Raw) (TagList, error) {
	if !raw.IsSet() || raw.Value() == nil {
		return TagList{values: []string{}}, nil
	}
	entries, ok := raw.Value().([]any)
	if !ok {
		return TagList{}, sharedomain.NewFieldError("tagList", sharedomain.Invalid)
	}
	for _, entry := range entries {
		if _, problem := parseTag(entry); problem != "" {
			return TagList{}, sharedomain.NewFieldError("tagList", problem)
		}
	}
	tags := make([]string, 0, len(entries))
	for _, entry := range entries {
		tag, _ := parseTag(entry)
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	if len(tags) > MaxTags {
		return TagList{}, sharedomain.NewFieldError("tagList", fmt.Sprintf("is too long (maximum is %d tags)", MaxTags))
	}
	return TagList{values: tags}, nil
}

func parseTag(entry any) (string, string) {
	text, ok := entry.(string)
	if !ok {
		return "", sharedomain.Invalid
	}
	tag := sharedomain.TrimText(text)
	if tag == "" {
		return "", "can't contain blank tags"
	}
	if strings.ContainsFunc(tag, func(character rune) bool { return character == ',' || sharedomain.IsSpace(character) }) {
		return "", "cannot contain whitespace or commas inside a tag"
	}
	if sharedomain.CountCharacters(tag) > MaxTagLength {
		return "", fmt.Sprintf("cannot contain a tag longer than %d characters", MaxTagLength)
	}
	return tag, ""
}

func RestoreTagList(values []string) TagList {
	if values == nil {
		values = []string{}
	}
	return TagList{values: slices.Clone(values)}
}

func (t TagList) Values() []string {
	return slices.Clone(t.values)
}

func (t TagList) Equals(other TagList) bool {
	return slices.Equal(t.values, other.values)
}
