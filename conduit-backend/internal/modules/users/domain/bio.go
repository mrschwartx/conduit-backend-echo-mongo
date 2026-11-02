package domain

import (
	sharedomain "conduit/backend/internal/shared/domain"
)

const bioMaxLength = 300

type Bio struct {
	value string
}

func NewBio(raw sharedomain.Raw) (*Bio, error) {
	if raw.Value() == nil {
		return nil, nil
	}
	text, ok := raw.Text()
	if !ok {
		return nil, sharedomain.NewFieldError("bio", sharedomain.Invalid)
	}
	value := sharedomain.TrimText(text)
	if value == "" {
		return nil, nil
	}
	if sharedomain.CountCharacters(value) > bioMaxLength {
		return nil, sharedomain.NewFieldError("bio", sharedomain.TooLong(bioMaxLength))
	}
	return &Bio{value: value}, nil
}

func RestoreBio(value *string) *Bio {
	if value == nil {
		return nil
	}
	return &Bio{value: *value}
}

func (b *Bio) Value() *string {
	if b == nil {
		return nil
	}
	value := b.value
	return &value
}
