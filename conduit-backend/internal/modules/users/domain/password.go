package domain

import (
	"fmt"

	sharedomain "conduit/backend/internal/shared/domain"
)

const (
	passwordMinLength = 8
	passwordMaxLength = 72
	passwordMaxBytes  = 72
)

type PlainPassword struct {
	value string
}

func NewPlainPassword(raw sharedomain.Raw) (PlainPassword, error) {
	text, ok := raw.Text()
	if !ok || text == "" {
		return PlainPassword{}, sharedomain.NewFieldError("password", sharedomain.Blank)
	}
	length := sharedomain.CountCharacters(text)
	if length < passwordMinLength {
		return PlainPassword{}, sharedomain.NewFieldError("password", sharedomain.TooShort(passwordMinLength))
	}
	if length > passwordMaxLength {
		return PlainPassword{}, sharedomain.NewFieldError("password", sharedomain.TooLong(passwordMaxLength))
	}
	if len(text) > passwordMaxBytes {
		return PlainPassword{}, sharedomain.NewFieldError("password", fmt.Sprintf("is too long (maximum is %d bytes)", passwordMaxBytes))
	}
	return PlainPassword{value: text}, nil
}

func (p PlainPassword) Value() string {
	return p.value
}
